package intentdesign

import (
	"testing"
)

func TestCleanTextValues(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"empty", nil, nil},
		{"single", []string{"hello"}, []string{"hello"}},
		{"trims whitespace", []string{"  hello  "}, []string{"hello"}},
		{"deduplicates", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"filters empty", []string{"", "a", ""}, []string{"a"}},
		{"filters null byte", []string{"a\x00b", "c"}, []string{"c"}},
		{"multiple non-goals", []string{"no auth", "no db"}, []string{"no auth", "no db"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanTextValues(tt.input)
			if !stringSlicesEqual(got, tt.expect) {
				t.Errorf("CleanTextValues(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestHasDesignReviewEvidence(t *testing.T) {
	tests := []struct {
		values   []string
		expected bool
	}{
		{[]string{"design review checked alternatives and risks"}, true},
		{[]string{"design audit complete"}, true},
		{[]string{"design evaluated"}, true},
		{[]string{"설계 검토 완료"}, true},
		{[]string{"설계 검수 완료"}, true},
		{[]string{"no evidence here"}, false},
		{nil, false},
		{[]string{""}, false},
		{[]string{"code review done", "design review done"}, true},
	}
	for _, tt := range tests {
		got := HasDesignReviewEvidence(tt.values)
		if got != tt.expected {
			t.Errorf("HasDesignReviewEvidence(%v) = %v, want %v", tt.values, got, tt.expected)
		}
	}
}

func TestMateriallyDifferentIntent(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		interpreted string
		expect     bool
	}{
		{"completely different", "add login", "build authentication system with session management", true},
		{"same text different", "add login button to the page", "implement a login button on the main page", true},
		{"very similar", "add login feature for users to sign in", "add login feature for users to sign in with email", true},
		{"too short raw", "add", "add login feature for users", true},
		{"too short interpreted", "add login feature for users", "add", true},
		{"identical", "add login feature for all users", "add login feature for all users", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := materiallyDifferentIntent(tt.raw, tt.interpreted)
			if got != tt.expect {
				t.Errorf("materiallyDifferentIntent(%q, %q) = %v, want %v", tt.raw, tt.interpreted, got, tt.expect)
			}
		})
	}
}

func TestIntentStopWord(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"the", true},
		{"a", true},
		{"an", true},
		{"please", true},
		{"좀", true},
		{"해주세요", true},
		{"login", false},
		{"feature", false},
		{"add", false},
	}
	for _, tt := range tests {
		got := intentStopWord(tt.token)
		if got != tt.expected {
			t.Errorf("intentStopWord(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestIntentTokenSet(t *testing.T) {
	got := intentTokenSet("add login feature for users to sign in")
	expected := []string{"add", "login", "feature", "for", "users", "to", "sign", "in"}
	for _, token := range expected {
		if !got[token] {
			t.Errorf("expected token %q in set", token)
		}
	}
	// Stop words should be excluded
	if got["the"] {
		t.Error("stop word 'the' should be excluded")
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
