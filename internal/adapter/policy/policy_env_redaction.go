package policy

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var diagnosticURLPattern = regexp.MustCompile(`https?://[^\s]+`)

func cleanEnvAllowlist(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func CleanEnvAllowlist(items []string) []string {
	return cleanEnvAllowlist(items)
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func redactArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = redactFreeform(arg)
	}
	return out
}

func RedactArgv(argv []string) []string {
	return redactArgv(argv)
}

func redactFreeform(s string) string {
	if secretLikeArg(s) {
		return "<redacted>"
	}
	return s
}

func RedactFreeform(s string) string {
	return redactFreeform(s)
}

func RedactDiagnostic(s string) string {
	if redacted := redactFreeform(s); redacted != s {
		return redacted
	}
	return diagnosticURLPattern.ReplaceAllString(s, "[REDACTED_URL]")
}

func secretLikeArg(arg string) bool {
	return secretArgRe.MatchString(arg) || secretPathRe.MatchString(filepath.ToSlash(arg))
}
