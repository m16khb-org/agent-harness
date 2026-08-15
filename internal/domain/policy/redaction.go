package policy

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var secretPathRe = regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)
var secretArgRe = regexp.MustCompile(`(?i)((token|password|passwd|secret|api[_-]?key|credential|authorization)=|authorization[[:space:]]*:[[:space:]]*bearer[[:space:]]+[^[:space:]]+)`)

var diagnosticURLPattern = regexp.MustCompile(`https?://[^\s]+`)

const maxBoundedDiagnosticBytes = 4096

func CleanEnvAllowlist(items []string) []string {
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

func ValidEnvName(name string) bool {
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

func RedactArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = RedactFreeform(arg)
	}
	return out
}

func RedactFreeform(s string) string {
	if SecretLikeArg(s) {
		return "<redacted>"
	}
	return s
}

func RedactDiagnostic(s string) string {
	if redacted := RedactFreeform(s); redacted != s {
		return redacted
	}
	return diagnosticURLPattern.ReplaceAllString(s, "[REDACTED_URL]")
}

func BoundedDiagnostic(value string, limit int) string {
	if limit <= 0 || limit > maxBoundedDiagnosticBytes {
		limit = maxBoundedDiagnosticBytes
	}
	value = strings.TrimSpace(RedactDiagnostic(value))
	if len(value) > limit {
		value = value[:limit] + "...[truncated]"
	}
	return value
}

func SecretLikeArg(arg string) bool {
	return secretArgRe.MatchString(arg) || secretPathRe.MatchString(filepath.ToSlash(arg))
}
