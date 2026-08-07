package reviewfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agent-harness/internal/adapter/core"
)

func ExtraPrompt(repo, promptFile string) (string, error) {
	if promptFile != "" {
		b, err := os.ReadFile(promptFile)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if b, err := os.ReadFile(filepath.Join(repo, ".agent-harness", "OPEN_API_SPEC.md")); err == nil {
		return string(b), nil
	}
	return "", nil
}

func Diff(repo string, files []string, diffFile string) (string, error) {
	if diffFile != "" {
		b, err := os.ReadFile(diffFile)
		return string(b), err
	}
	args := append([]string{"diff", "--cached", "--"}, files...)
	code, out, stderr := core.GitCmd(repo, args...)
	if code != 0 {
		return "", fmt.Errorf("git diff failed: %s", stderr)
	}
	return out, nil
}

func Input(repo string, files []string, diffFile string, all bool) (string, error) {
	if all && diffFile == "" {
		return FullContent(repo, files)
	}
	return Diff(repo, files, diffFile)
}

func FullContent(repo string, files []string) (string, error) {
	var b strings.Builder
	for _, file := range files {
		clean := filepath.Clean(file)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return "", fmt.Errorf("unsafe file path %q", file)
		}
		content, err := os.ReadFile(filepath.Join(repo, clean))
		if err != nil {
			return "", err
		}
		b.WriteString("\n--- FILE: ")
		b.WriteString(clean)
		b.WriteString(" ---\n")
		b.Write(content)
		if len(content) == 0 || content[len(content)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

func Staged(repo string) []string {
	code, out, _ := core.GitCmd(repo, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--")
	if code != 0 {
		return nil
	}
	return Normalize(repo, splitLines(out))
}

func Tracked(repo string) []string {
	code, out, _ := core.GitCmd(repo, "ls-files")
	if code != 0 {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		file := strings.TrimSpace(line)
		if file == "" || !IsCandidate(file) {
			continue
		}
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func Normalize(repo string, files []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" || !IsCandidate(file) {
			continue
		}
		if filepath.IsAbs(file) {
			if rel, err := filepath.Rel(repo, file); err == nil {
				file = rel
			}
		}
		file = filepath.ToSlash(filepath.Clean(file))
		if file == "." || strings.HasPrefix(file, "../") || seen[file] {
			continue
		}
		seen[file] = true
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

var candidateRe = regexp.MustCompile(`(?i)(controller|dto|route|router|handler|endpoint|openapi|swagger|api|schema|proto)`)

func IsCandidate(file string) bool {
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".txt" {
		return false
	}
	if base == "package.json" || strings.HasSuffix(base, "lock") {
		return false
	}
	return candidateRe.MatchString(file)
}

func splitLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
