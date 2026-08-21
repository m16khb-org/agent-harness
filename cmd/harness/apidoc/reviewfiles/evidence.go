package reviewfiles

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Evidence extracts a bounded, machine-generated summary of the business-logic
// error contracts reachable from the changed API files: service methods the
// changed controllers call, the exceptions those methods throw, microservice
// hops from ClientProxy.send/emit to @MessagePattern handlers, and the exception
// filters that map those errors to HTTP statuses. It is review input, not a
// verdict: the host agent cross-checks the documented responses against it.
func Evidence(repo string, files []string) string {
	extractor := newEvidenceExtractor(repo)
	for _, file := range files {
		if !isControllerLike(file) || !strings.HasSuffix(file, ".ts") {
			continue
		}
		text, ok := extractor.readRepoFile(file)
		if !ok {
			continue
		}
		extractor.extractController(file, text)
	}
	return extractor.render()
}

const (
	evidenceMaxFiles   = 4000
	evidenceMaxFileLen = 512 * 1024
	evidenceMaxBytes   = 12 * 1024
)

type evidenceEntry struct {
	lines []string
}

type evidenceExtractor struct {
	repo      string
	classText map[string]string // class name -> file content
	classPath map[string]string // class name -> repo-relative path
	fileText  map[string]string // repo-relative path -> file content
	entries   map[string]*evidenceEntry
	collected int
}

func newEvidenceExtractor(repo string) *evidenceExtractor {
	return &evidenceExtractor{
		repo:      repo,
		classText: map[string]string{},
		classPath: map[string]string{},
		fileText:  map[string]string{},
		entries:   map[string]*evidenceEntry{},
	}
}

func (e *evidenceExtractor) entry(key string) *evidenceEntry {
	entry, ok := e.entries[key]
	if !ok {
		entry = &evidenceEntry{}
		e.entries[key] = entry
	}
	return entry
}

var (
	injectionRe   = regexp.MustCompile(`(?:private|public|protected)\s+(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`)
	methodNameRe2 = regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|async\s+|static\s+)*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	serviceCallRe = regexp.MustCompile(`this\.([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	throwRe       = regexp.MustCompile(`throw\s+new\s+([A-Za-z_][A-Za-z0-9_]*)\s*(\([^;]*)?`)
	patternArgRe  = regexp.MustCompile(`\.(?:send|emit)\s*\(\s*(\{[^}]*\}|['"\x60][^'"\x60]+['"\x60])`)
	classDeclRe   = regexp.MustCompile(`\b(?:export\s+)?(?:abstract\s+)?class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	catchRe       = regexp.MustCompile(`@Catch\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)`)
	httpStatusRe  = regexp.MustCompile(`HttpStatus\.([A-Z_]+)|\.status\s*\(\s*(\d{3})`)
)

func isControllerLike(file string) bool {
	lower := strings.ToLower(file)
	return strings.Contains(lower, "controller") || strings.Contains(lower, "handler") || strings.Contains(lower, "route") || strings.Contains(lower, "router")
}

func (e *evidenceExtractor) readRepoFile(rel string) (string, bool) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(e.repo, clean))
	if err != nil || int64(len(b)) > evidenceMaxFileLen {
		return "", false
	}
	return string(b), true
}

// extractController parses one changed controller file: constructor injections,
// then every member method's body for service calls and microservice sends.
func (e *evidenceExtractor) extractController(file, text string) {
	injections := parseInjections(text)
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		m := methodNameRe2.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		bodyStart, body, ok := extractMethodBody(lines, i)
		if !ok {
			continue
		}
		method := m[1]
		if method == "constructor" {
			continue
		}
		key := file + "#" + method
		for _, call := range serviceCallRe.FindAllStringSubmatch(body, -1) {
			className, known := injections[call[1]]
			if !known {
				continue
			}
			if isClientProxyType(className) {
				continue
			}
			serviceText, servicePath, ok := e.resolveClass(className)
			if !ok {
				continue
			}
			e.addThrows(key, servicePath, serviceText, call[2], 0)
		}
		for _, send := range patternArgRe.FindAllStringSubmatch(body, -1) {
			e.addMicroserviceHop(key, send[1])
		}
		_ = bodyStart
	}
}

// parseInjections maps constructor property names to injected class names.
func parseInjections(text string) map[string]string {
	injections := map[string]string{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "constructor(") {
			continue
		}
		depth := 0
		for j := i; j < len(lines); j++ {
			for _, ch := range lines[j] {
				switch ch {
				case '(':
					depth++
				case ')':
					depth--
				}
			}
			for _, m := range injectionRe.FindAllStringSubmatch(lines[j], -1) {
				injections[m[1]] = m[2]
			}
			if j > i && depth <= 0 {
				return injections
			}
			if j == i && depth <= 0 {
				return injections
			}
		}
		return injections
	}
	return injections
}

func isClientProxyType(className string) bool {
	return strings.Contains(className, "ClientProxy") || strings.Contains(className, "Client")
}

// extractMethodBody returns the balanced body of the method whose signature
// starts at lines[signature] and the 0-based line where the body opens.
func extractMethodBody(lines []string, signature int) (int, string, bool) {
	i := signature
	depth := 0
	for i < len(lines) && !strings.Contains(lines[i], "{") {
		if strings.Contains(lines[i], ";") {
			return 0, "", false
		}
		i++
	}
	if i >= len(lines) {
		return 0, "", false
	}
	bodyStart := i
	var body []string
	for ; i < len(lines); i++ {
		body = append(body, lines[i])
		depth += evidenceBraceDepthDelta(lines[i])
		if depth <= 0 && i >= bodyStart {
			return bodyStart, strings.Join(body, "\n"), true
		}
	}
	return bodyStart, strings.Join(body, "\n"), true
}

// resolveClass locates a class declaration in the repo, building the class
// index lazily on first use.
func (e *evidenceExtractor) resolveClass(className string) (text, path string, ok bool) {
	if cached, found := e.classText[className]; found {
		if cached == "" {
			return "", "", false
		}
		return cached, e.classPath[className], true
	}
	e.buildClassIndex()
	if text, ok := e.classText[className]; ok && text != "" {
		return text, e.classPath[className], true
	}
	return "", "", false
}

func (e *evidenceExtractor) buildClassIndex() {
	filepath.WalkDir(e.repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "dist" || name == ".git" || name == "coverage" || name == "build" || name == ".next" || strings.HasPrefix(name, ".") {
				if path != e.repo {
					return fs.SkipDir
				}
			}
			return nil
		}
		if len(e.classPath) >= evidenceMaxFiles {
			return fs.SkipAll
		}
		if !strings.HasSuffix(path, ".ts") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > evidenceMaxFileLen {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(e.repo, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		text := string(b)
		e.fileText[rel] = text
		for _, m := range classDeclRe.FindAllStringSubmatch(text, -1) {
			if _, exists := e.classPath[m[1]]; !exists {
				e.classPath[m[1]] = rel
				e.classText[m[1]] = text
			}
		}
		return nil
	})
}

// addThrows appends the exceptions thrown by serviceClass#method (plus one
// same-class hop) to the evidence entry.
func (e *evidenceExtractor) addThrows(key, servicePath, serviceText, method string, depth int) {
	if depth > 1 || e.collected > 200 {
		return
	}
	lines := strings.Split(serviceText, "\n")
	for i := 0; i < len(lines); i++ {
		m := methodNameRe2.FindStringSubmatch(lines[i])
		if m == nil || m[1] != method {
			continue
		}
		_, body, ok := extractMethodBody(lines, i)
		if !ok {
			return
		}
		for j, line := range strings.Split(body, "\n") {
			if t := throwRe.FindStringSubmatch(line); t != nil {
				e.entry(key).lines = append(e.entry(key).lines, fmt.Sprintf("%s: throw new %s%s (%s:%d)", method, t[1], throwDetail(t[2]), filepath.Base(servicePath), i+j+1))
				e.collected++
			}
		}
		if depth == 0 {
			for _, call := range thisMethodCallRe.FindAllStringSubmatch(body, -1) {
				e.addThrows(key, servicePath, serviceText, call[1], depth+1)
			}
		}
		return
	}
}

var thisMethodCallRe = regexp.MustCompile(`this\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

func throwDetail(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if len(arg) > 72 {
		arg = arg[:72]
		if cut := strings.LastIndexAny(arg, " ,'\t"); cut > 24 {
			arg = arg[:cut]
		}
		arg = strings.TrimSpace(arg) + "..."
		opened := strings.Count(arg, "(") - strings.Count(arg, ")")
		if opened > 0 {
			arg += strings.Repeat(")", opened)
		} else if strings.Count(arg, "'")%2 == 1 {
			arg += "'"
		} else if strings.Count(arg, "\"")%2 == 1 {
			arg += "\""
		}
		return " " + arg
	}
	arg = strings.TrimSuffix(strings.TrimSpace(arg), ",")
	return " " + strings.TrimSpace(arg)
}

// patternWindow returns the decorator line plus its continuation lines so a
// multi-line @MessagePattern({ cmd: ... }) still shares literals with the hop.
func patternWindow(lines []string, from int) string {
	var b strings.Builder
	depth := 0
	for i := from; i < len(lines) && i < from+4; i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
		depth += evidenceBraceDepthDelta(lines[i]) + parenDepthDeltaOf(lines[i])
		if i > from && depth <= 0 {
			break
		}
	}
	return b.String()
}

func parenDepthDeltaOf(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '(':
			delta++
		case ')':
			delta--
		}
	}
	return delta
}

// addMicroserviceHop links a ClientProxy.send/emit pattern to @MessagePattern
// handlers elsewhere in the repo and their service throws.
func (e *evidenceExtractor) addMicroserviceHop(key, patternArg string) {
	literals := stringLiterals(patternArg)
	if len(literals) == 0 {
		return
	}
	e.buildClassIndex()
	matched := 0
	for _, path := range sortedPaths(e.fileText) {
		text := e.fileText[path]
		if !strings.Contains(text, "@MessagePattern") && !strings.Contains(text, "@EventPattern") {
			continue
		}
		if !fileContainsAny(text, literals) {
			continue
		}
		matched++
		e.entry(key).lines = append(e.entry(key).lines, fmt.Sprintf("microservice hop %s matched @MessagePattern/@EventPattern in %s", patternArg, path))
		injections := parseInjections(text)
		lines := strings.Split(text, "\n")
		for i := 0; i < len(lines); i++ {
			if !strings.Contains(lines[i], "@MessagePattern") && !strings.Contains(lines[i], "@EventPattern") {
				continue
			}
			if !fileContainsAny(patternWindow(lines, i), literals) {
				continue
			}
			handler := nextMethodName(lines, i)
			if handler == "" {
				continue
			}
			_, body, ok := handlerBody(lines, i, handler)
			if !ok {
				continue
			}
			for _, call := range serviceCallRe.FindAllStringSubmatch(body, -1) {
				className, known := injections[call[1]]
				if !known || isClientProxyType(className) {
					continue
				}
				serviceText, servicePath, ok := e.resolveClass(className)
				if !ok {
					continue
				}
				e.entry(key).lines = append(e.entry(key).lines, fmt.Sprintf("  handler %s -> %s.%s (%s)", handler, className, call[2], servicePath))
				e.addThrows(key, servicePath, serviceText, call[2], 0)
			}
		}
		if matched >= 4 {
			break
		}
	}
	if matched == 0 {
		e.entry(key).lines = append(e.entry(key).lines, fmt.Sprintf("microservice hop %s: no @MessagePattern/@EventPattern handler found in repo (remote service)", patternArg))
	}
	e.addExceptionFilters(key)
}

func (e *evidenceExtractor) addExceptionFilters(key string) {
	e.buildClassIndex()
	for _, path := range sortedPaths(e.fileText) {
		text := e.fileText[path]
		catch := catchRe.FindStringSubmatch(text)
		if catch == nil {
			continue
		}
		var mappings []string
		for _, m := range httpStatusRe.FindAllStringSubmatch(text, -1) {
			if m[1] != "" {
				mappings = append(mappings, "HttpStatus."+m[1])
			} else if m[2] != "" {
				mappings = append(mappings, m[2])
			}
		}
		if len(mappings) == 0 {
			continue
		}
		e.entry(key).lines = append(e.entry(key).lines, fmt.Sprintf("exception filter @Catch(%s) in %s maps to %s", catch[1], path, strings.Join(mappings, ", ")))
	}
}

func nextMethodName(lines []string, from int) string {
	for i := from; i < len(lines) && i < from+12; i++ {
		if m := methodNameRe2.FindStringSubmatch(lines[i]); m != nil && m[1] != "constructor" {
			return m[1]
		}
	}
	return ""
}

func handlerBody(lines []string, from int, handler string) (int, string, bool) {
	for i := from; i < len(lines) && i < from+12; i++ {
		m := methodNameRe2.FindStringSubmatch(lines[i])
		if m == nil || m[1] != handler {
			continue
		}
		return extractMethodBody(lines, i)
	}
	return 0, "", false
}

func stringLiterals(text string) []string {
	var literals []string
	re := regexp.MustCompile(`['"\x60]([^'"\x60]+)['"\x60]`)
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		if len(m[1]) >= 2 {
			literals = append(literals, m[1])
		}
	}
	return literals
}

func fileContainsAny(text string, literals []string) bool {
	for _, literal := range literals {
		if strings.Contains(text, literal) {
			return true
		}
	}
	return false
}

func sortedPaths(fileText map[string]string) []string {
	var paths []string
	for path := range fileText {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (e *evidenceExtractor) render() string {
	keys := make([]string, 0, len(e.entries))
	for key := range e.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	total := 0
	for _, key := range keys {
		entry := e.entries[key]
		if len(entry.lines) == 0 {
			continue
		}
		b.WriteString("- ")
		b.WriteString(key)
		b.WriteString("\n")
		for _, line := range entry.lines {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
			total += len(line)
			if total > evidenceMaxBytes {
				b.WriteString("    (evidence truncated)\n")
				return b.String()
			}
		}
	}
	return b.String()
}

func evidenceBraceDepthDelta(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}
