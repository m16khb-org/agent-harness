package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agent-harness/internal/core"
)

func apiDocReviewExtraPrompt(options apiDocReviewOptions) (string, error) {
	if options.PromptFile != "" {
		b, err := os.ReadFile(options.PromptFile)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if b, err := os.ReadFile(filepath.Join(options.Repo, ".agent-harness", "OPEN_API_SPEC.md")); err == nil {
		return string(b), nil
	}
	return "", nil
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
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

func apiDocInput(repo string, files []string, diffFile string, all bool) (string, error) {
	if all && diffFile == "" {
		return apiDocFullContent(repo, files)
	}
	return apiDocDiff(repo, files, diffFile)
}

func apiDocFullContent(repo string, files []string) (string, error) {
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

func stagedAPIDocFiles(repo string) []string {
	code, out, _ := core.GitCmd(repo, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--")
	if code != 0 {
		return nil
	}
	return normalizeAPIDocFiles(repo, splitLines(out))
}

func trackedAPIDocFiles(repo string) []string {
	code, out, _ := core.GitCmd(repo, "ls-files")
	if code != 0 {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		file := strings.TrimSpace(line)
		if file == "" || !isAPIDocCandidate(file) {
			continue
		}
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" || !isAPIDocCandidate(file) {
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

var apiDocCandidateRe = regexp.MustCompile(`(?i)(controller|dto|route|router|handler|endpoint|openapi|swagger|api|schema|proto)`)

func isAPIDocCandidate(file string) bool {
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".txt" {
		return false
	}
	if base == "package.json" || strings.HasSuffix(base, "lock") {
		return false
	}
	return apiDocCandidateRe.MatchString(file)
}
