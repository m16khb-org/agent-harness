package riskqa

import "testing"

func TestParseGitStatusPath(t *testing.T) {
	tests := map[string]string{
		" M cmd/issueops/main.go":              "cmd/issueops/main.go",
		"?? internal/adapter/new_test.go":      "internal/adapter/new_test.go",
		"R  old/path.go -> internal/core/x.go": "internal/core/x.go",
		"":                                     "",
	}
	for line, want := range tests {
		if got := ParseGitStatusPath(line); got != want {
			t.Fatalf("ParseGitStatusPath(%q)=%q want %q", line, got, want)
		}
	}
}
