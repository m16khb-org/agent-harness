package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type GuardCheckRequest struct {
	RepoRoot string   `json:"repo_root"`
	Staged   bool     `json:"staged"`
	All      bool     `json:"all"`
	Files    []string `json:"files,omitempty"`
}

type GuardCheckResult struct {
	OK           bool           `json:"ok"`
	RepoRoot     string         `json:"repo_root"`
	Mode         string         `json:"mode"`
	CheckedFiles []string       `json:"checked_files"`
	Findings     []GuardFinding `json:"findings"`
	Summary      GuardSummary   `json:"summary"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type GuardFinding struct {
	Severity    string   `json:"severity"`
	Rule        string   `json:"rule"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type GuardSummary struct {
	Block  int `json:"block"`
	Warn   int `json:"warn"`
	Review int `json:"review"`
	Info   int `json:"info"`
}

type GuardBlockedError struct {
	Findings []GuardFinding
}

func (e GuardBlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

func IsGuardBlocked(err error) bool {
	_, ok := err.(GuardBlockedError)
	return ok
}

var ambiguousTestNameRe = regexp.MustCompile(`(?i)\b(Test(Works|Basic|Test[0-9]*)|it\s*\(\s*["'](works|basic|test)[^"']*["']|test\s*\(\s*["'](works|basic|test)[^"']*["'])`)
var sleepInTestRe = regexp.MustCompile(`(?i)\b(time\.Sleep|Thread\.sleep|sleep\s*\(|setTimeout\s*\()`)
var externalURLRe = regexp.MustCompile(`https?://[^\s"')]+`)
var snapshotAssertionRe = regexp.MustCompile(`(?i)(toMatchSnapshot|assert.*golden|golden mismatch)`)
var newSymbolRe = regexp.MustCompile(`^\s*(?:func\s+|function\s+|def\s+|class\s+|type\s+|interface\s+|const\s+|let\s+|var\s+)([A-Za-z_][A-Za-z0-9_]*)`)

func GuardCheck(req GuardCheckRequest) GuardCheckResult {
	root := absOrOriginal(req.RepoRoot)
	if root == "" {
		root = absOrOriginal(".")
	}
	result := GuardCheckResult{
		OK:           true,
		RepoRoot:     root,
		Mode:         guardMode(req),
		CheckedFiles: []string{},
		Findings:     []GuardFinding{},
		Warnings:     []string{},
	}
	files := guardTargetFiles(root, req)
	result.CheckedFiles = files
	existingSymbols := guardExistingSymbols(root, files)
	hasProdChange := false
	hasTestChange := false
	hasContractSurfaceChange := false
	hasGoldenChange := false
	for _, rel := range files {
		if secretPathRe.MatchString(filepath.ToSlash(rel)) {
			result.Findings = append(result.Findings, GuardFinding{
				Severity: "block",
				Rule:     "secret-like-path",
				File:     rel,
				Message:  "Secret-like paths must not be committed or analyzed as ordinary source.",
			})
			continue
		}
		if isTestPath(rel) {
			hasTestChange = true
		} else if isSourcePath(rel) {
			hasProdChange = true
		}
		if isContractSurfacePath(rel) {
			hasContractSurfaceChange = true
		}
		if strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.Contains(strings.ToLower(rel), "golden") {
			hasGoldenChange = true
		}
		content, ok := guardReadFile(root, rel, req.Staged)
		if !ok {
			continue
		}
		result.Findings = append(result.Findings, guardFileFindings(rel, content, existingSymbols)...)
	}
	if hasProdChange && !hasTestChange {
		result.Findings = append(result.Findings, GuardFinding{
			Severity: "warn",
			Rule:     "prod-change-without-test",
			Message:  "Production source changed without a changed test file; verify this is documentation/config-only or add focused coverage.",
		})
	}
	if hasContractSurfaceChange && !hasGoldenChange {
		result.Findings = append(result.Findings, GuardFinding{
			Severity: "warn",
			Rule:     "contract-surface-without-golden",
			Message:  "CLI/MCP/adapter contract surface changed without a golden/testdata update.",
		})
	}
	result.Findings = dedupeGuardFindings(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		if guardSeverityRank(result.Findings[i].Severity) != guardSeverityRank(result.Findings[j].Severity) {
			return guardSeverityRank(result.Findings[i].Severity) < guardSeverityRank(result.Findings[j].Severity)
		}
		if result.Findings[i].File != result.Findings[j].File {
			return result.Findings[i].File < result.Findings[j].File
		}
		if result.Findings[i].Line != result.Findings[j].Line {
			return result.Findings[i].Line < result.Findings[j].Line
		}
		return result.Findings[i].Rule < result.Findings[j].Rule
	})
	for _, finding := range result.Findings {
		switch finding.Severity {
		case "block":
			result.Summary.Block++
		case "warn":
			result.Summary.Warn++
		case "review":
			result.Summary.Review++
		default:
			result.Summary.Info++
		}
	}
	result.OK = result.Summary.Block == 0
	return result
}

func guardMode(req GuardCheckRequest) string {
	if req.All {
		return "all"
	}
	if req.Staged {
		return "staged"
	}
	if len(req.Files) > 0 {
		return "files"
	}
	return "staged"
}

func guardTargetFiles(root string, req GuardCheckRequest) []string {
	if len(req.Files) > 0 {
		return cleanGuardFiles(root, req.Files)
	}
	if req.All {
		files := []string{}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if name == ".git" || name == "bin" || name == ".cache" || name == ".codex" || name == ".codegraph" || name == ".omx" || name == ".omc" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err == nil && isGuardRelevantPath(rel) {
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		return uniqSorted(files)
	}
	out := splitLines(GitOut(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR"))
	return cleanGuardFiles(root, out)
}

func cleanGuardFiles(root string, files []string) []string {
	out := []string{}
	for _, file := range files {
		file = filepath.ToSlash(strings.TrimSpace(file))
		if file == "" || strings.HasPrefix(file, "../") || filepath.IsAbs(file) || !isGuardRelevantPath(file) {
			continue
		}
		out = append(out, file)
	}
	return uniqSorted(out)
}

func isGuardRelevantPath(rel string) bool {
	if secretPathRe.MatchString(filepath.ToSlash(rel)) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if ext == "" {
		return strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.HasSuffix(rel, "Dockerfile")
	}
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".bash", ".zsh", ".fish", ".yaml", ".yml", ".json", ".toml", ".md", ".sql":
		return true
	default:
		return false
	}
}

func guardReadFile(root, rel string, staged bool) (string, bool) {
	if staged {
		if code, out, _ := GitCmd(root, "show", ":"+rel); code == 0 {
			return out, true
		}
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func guardFileFindings(rel, content string, existingSymbols map[string][]string) []GuardFinding {
	findings := []GuardFinding{}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNo := i + 1
		if isExecutableTestSourcePath(rel) {
			if ambiguousTestNameRe.MatchString(line) {
				findings = append(findings, GuardFinding{Severity: "warn", Rule: "ambiguous-test-name", File: rel, Line: lineNo, Message: "Test name is too generic to communicate the protected contract.", Evidence: strings.TrimSpace(line)})
			}
			if sleepInTestRe.MatchString(line) {
				findings = append(findings, GuardFinding{Severity: "block", Rule: "sleep-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on wall-clock sleep; use deterministic synchronization or fake clocks.", Evidence: strings.TrimSpace(line)})
			}
			for _, url := range externalURLRe.FindAllString(line, -1) {
				if !guardAllowsFixtureURL(url) {
					findings = append(findings, GuardFinding{Severity: "block", Rule: "real-external-service-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on real external services.", Evidence: url})
				}
			}
			if strings.Contains(strings.ToLower(line), "localhost") {
				findings = append(findings, GuardFinding{Severity: "warn", Rule: "localhost-in-test", File: rel, Line: lineNo, Message: "Local service dependencies in tests need explicit isolation and lifecycle control.", Evidence: strings.TrimSpace(line)})
			}
			if snapshotAssertionRe.MatchString(line) {
				findings = append(findings, GuardFinding{Severity: "warn", Rule: "snapshot-test-review", File: rel, Line: lineNo, Message: "Snapshot/golden assertions should be paired with focused contract checks and intentional update notes.", Evidence: strings.TrimSpace(line)})
			}
		}
		if m := newSymbolRe.FindStringSubmatch(line); len(m) == 2 {
			symbol := m[1]
			if reuseFinding, ok := guardReuseFinding(rel, lineNo, symbol, existingSymbols); ok {
				findings = append(findings, reuseFinding)
			}
		}
	}
	if isTestPath(rel) && len(content) > 200_000 {
		findings = append(findings, GuardFinding{Severity: "warn", Rule: "large-test-fixture", File: rel, Message: "Large test files or fixtures can hide weak assertions; prefer small named fixtures."})
	}
	return findings
}

func guardExistingSymbols(root string, targetFiles []string) map[string][]string {
	targets := stringSet(targetFiles...)
	symbols := map[string][]string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "bin" || name == ".cache" || name == ".codex" || name == ".codegraph" || name == ".omx" || name == ".omc" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if targets[rel] || !isSourcePath(rel) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(b), "\n") {
			if m := newSymbolRe.FindStringSubmatch(line); len(m) == 2 {
				key := normalizeGuardSymbol(m[1])
				if key != "" {
					symbols[key] = append(symbols[key], rel)
				}
			}
		}
		return nil
	})
	for key := range symbols {
		symbols[key] = uniqSorted(symbols[key])
	}
	return symbols
}

func guardReuseFinding(rel string, line int, symbol string, existing map[string][]string) (GuardFinding, bool) {
	key := normalizeGuardSymbol(symbol)
	if key == "" || len(existing[key]) == 0 {
		return GuardFinding{}, false
	}
	return GuardFinding{
		Severity: "review",
		Rule:     "reuse-before-new",
		File:     rel,
		Line:     line,
		Message:  "New symbol resembles existing repository code; confirm reuse or record why a new implementation is necessary.",
		Evidence: symbol,
		Suggestions: []string{
			fmt.Sprintf("Review existing candidates: %s", strings.Join(existing[key], ", ")),
			fmt.Sprintf("Search repo for similar helpers: rg %q .", symbol),
		},
	}, true
}

func normalizeGuardSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	var tokens []string
	var current strings.Builder
	var previousLower bool
	for _, r := range symbol {
		if r == '_' || r == '-' {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
			previousLower = false
			continue
		}
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && previousLower && current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
		current.WriteRune(r)
		previousLower = r >= 'a' && r <= 'z'
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToLower(current.String()))
	}
	filtered := []string{}
	for _, token := range tokens {
		token = strings.TrimSuffix(token, "s")
		if len(token) > 2 {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	sort.Strings(filtered)
	b, _ := json.Marshal(filtered)
	return string(b)
}

func isTestPath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(p, "test") || strings.Contains(p, "spec") || strings.Contains(p, "fixture") || strings.Contains(p, "golden")
}

func isExecutableTestSourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if strings.Contains(p, "testdata/") || strings.Contains(p, ".golden.") || strings.Contains(p, "/fixtures/") || strings.Contains(p, "/fixture/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh":
		return strings.Contains(p, "test") || strings.Contains(p, "spec")
	default:
		return false
	}
}

func guardAllowsFixtureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	return host == "example.com" ||
		host == "example.org" ||
		host == "example.net" ||
		host == "example.invalid" ||
		host == "127.0.0.1" ||
		host == "localhost" ||
		(host == "github.com" && strings.HasPrefix(path, "/example/"))
}

func isSourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if isTestPath(p) || strings.HasPrefix(p, ".agent-harness/") || strings.HasPrefix(p, "docs/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".sql":
		return true
	default:
		return false
	}
}

func isContractSurfacePath(rel string) bool {
	p := filepath.ToSlash(rel)
	return strings.HasPrefix(p, "cmd/harness/") || strings.HasPrefix(p, "internal/adapter/") || strings.HasPrefix(p, "internal/core/")
}

func dedupeGuardFindings(findings []GuardFinding) []GuardFinding {
	seen := map[string]bool{}
	out := []GuardFinding{}
	for _, finding := range findings {
		key := finding.Severity + "\x00" + finding.Rule + "\x00" + finding.File + "\x00" + fmt.Sprint(finding.Line) + "\x00" + finding.Evidence
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

func guardSeverityRank(severity string) int {
	switch severity {
	case "block":
		return 0
	case "warn":
		return 1
	case "review":
		return 2
	default:
		return 3
	}
}
