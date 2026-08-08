package qagate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var secretMaterialPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{name: "private_key", re: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{name: "aws_access_key_id", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{name: "github_token", re: regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`)},
	{name: "openai_token", re: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{name: "secret_assignment", re: regexp.MustCompile(`(?i)\b(token|secret|password|api[_-]?key|access[_-]?key)\s*[:=]\s*["']?([^\s"',}]+)`)},
}

func ValidateRedactionAudit(root string) StepResult {
	return validateRedactionAuditWithDeps(root, docsValidationDeps{})
}

func validateRedactionAuditWithDeps(root string, deps docsValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	errs := []string{}
	for _, path := range redactionAuditFilesWithDeps(root, deps) {
		b, err := deps.readFile(path)
		if err != nil {
			errs = append(errs, "read redaction audit file "+path+": "+err.Error())
			continue
		}
		rel, err := deps.rel(root, path)
		if err != nil {
			rel = path
		}
		for _, finding := range findUnredactedSecretLike(string(b)) {
			errs = append(errs, filepath.ToSlash(rel)+": "+finding)
		}
	}
	return assertionStep("redaction audit", started, errs)
}

func redactionAuditFiles(root string) []string {
	return redactionAuditFilesWithDeps(root, docsValidationDeps{})
}

func redactionAuditFilesWithDeps(root string, deps docsValidationDeps) []string {
	deps = deps.withDefaults()
	seen := map[string]bool{}
	out := []string{}
	add := func(path string) {
		if path == "" || seen[path] || !deps.exists(path) {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, path := range deps.listDocs(root) {
		add(path)
	}
	for _, pattern := range []string{
		filepath.Join(root, "cmd", "harness", "testdata", "*"),
		filepath.Join(root, "internal", "adapter", "testdata", "*"),
		filepath.Join(root, "skills", "*", "SKILL.md"),
		filepath.Join(root, "skills", "*", "agents", "openai.yaml"),
	} {
		matches, _ := deps.glob(pattern)
		for _, match := range matches {
			add(match)
		}
	}
	sort.Strings(out)
	return out
}

func FindUnredactedSecretLike(text string) []string {
	findings := []string{}
	for lineNo, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || lineContainsAllowedSecretPlaceholder(line) {
			continue
		}
		for _, pattern := range secretMaterialPatterns {
			if pattern.re.MatchString(line) {
				findings = append(findings, fmt.Sprintf("line %d contains %s", lineNo+1, pattern.name))
			}
		}
	}
	return findings
}

func findUnredactedSecretLike(text string) []string {
	return FindUnredactedSecretLike(text)
}

func lineContainsAllowedSecretPlaceholder(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"redacted", "placeholder", "example", "fake", "dummy", "sample", "$secret", "$token", "<secret", "<token", "..."} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
