package commandparse

import "testing"

func TestParseRemotePullRequestCreateRecognizesSupportedShapes(t *testing.T) {
	tests := []struct {
		name     string
		argv     []string
		provider string
		kind     string
		base     string
		hasFlag  bool
	}{
		{
			name:     "glab separate target flag",
			argv:     []string{"glab", "mr", "create", "--source-branch", "feat/x", "--target-branch", "parent/work"},
			provider: "gitlab", kind: "mr", base: "parent/work", hasFlag: true,
		},
		{
			name:     "glab joined target flag",
			argv:     []string{"glab", "mr", "create", "--target-branch=parent/work"},
			provider: "gitlab", kind: "mr", base: "parent/work", hasFlag: true,
		},
		{
			name:     "gh separate base flag",
			argv:     []string{"gh", "pr", "create", "--base", "parent/work", "--title", "t"},
			provider: "github", kind: "pr", base: "parent/work", hasFlag: true,
		},
		{
			name:     "gh joined base flag",
			argv:     []string{"gh", "pr", "create", "--base=parent/work"},
			provider: "github", kind: "pr", base: "parent/work", hasFlag: true,
		},
		{
			name:     "gh short base flag",
			argv:     []string{"gh", "pr", "create", "-B", "parent/work"},
			provider: "github", kind: "pr", base: "parent/work", hasFlag: true,
		},
		{
			name:     "create without any target flag",
			argv:     []string{"glab", "mr", "create", "--fill"},
			provider: "gitlab", kind: "mr", base: "", hasFlag: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, ok := ParseRemotePullRequestCreate(tc.argv)
			if !ok {
				t.Fatalf("ParseRemotePullRequestCreate(%v) ok = false, want true", tc.argv)
			}
			if parsed.Provider != tc.provider {
				t.Errorf("Provider = %q, want %q", parsed.Provider, tc.provider)
			}
			if parsed.Kind != tc.kind {
				t.Errorf("Kind = %q, want %q", parsed.Kind, tc.kind)
			}
			if parsed.BaseBranch != tc.base {
				t.Errorf("BaseBranch = %q, want %q", parsed.BaseBranch, tc.base)
			}
			if parsed.HasBaseFlag != tc.hasFlag {
				t.Errorf("HasBaseFlag = %v, want %v", parsed.HasBaseFlag, tc.hasFlag)
			}
		})
	}
}

func TestParseRemotePullRequestCreateIgnoresUnrelatedCommands(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{"too short", []string{"glab", "mr"}},
		{"not create", []string{"glab", "mr", "view", "1"}},
		{"gh merge", []string{"gh", "pr", "merge", "494"}},
		{"issue create", []string{"glab", "issue", "create", "--title", "t"}},
		{"unknown cli", []string{"git", "mr", "create"}},
		{"gh with gitlab kind", []string{"gh", "mr", "create"}},
		{"glab with github kind", []string{"glab", "pr", "create"}},
		{"empty argv", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := ParseRemotePullRequestCreate(tc.argv); ok {
				t.Fatalf("ParseRemotePullRequestCreate(%v) ok = true, want false", tc.argv)
			}
		})
	}
}
