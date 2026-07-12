package remote

import (
	"strings"
	"testing"
)

func TestSplitURLPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/user/repo/issues/1", []string{"user", "repo", "issues", "1"}},
		{"user/repo/issues/1/", []string{"user", "repo", "issues", "1"}},
		{"/", nil},
		{"", nil},
		{"///", nil},
	}
	for _, tt := range tests {
		got := SplitURLPath(tt.path)
		if !stringSlicesEqual(got, tt.want) {
			t.Errorf("SplitURLPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestCleanValues(t *testing.T) {
	tests := []struct {
		input  []string
		expect []string
	}{
		{nil, nil},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]string{"a", "a"}, []string{"a"}},
		{[]string{"", "a", ""}, []string{"a"}},
		{[]string{"a\x00b"}, nil},
	}
	for _, tt := range tests {
		got := CleanValues(tt.input)
		if !stringSlicesEqual(got, tt.expect) {
			t.Errorf("CleanValues(%v) = %v, want %v", tt.input, got, tt.expect)
		}
	}
}

func TestInvalidAssignee(t *testing.T) {
	tests := []struct {
		values   []string
		expected string
	}{
		{[]string{"@me"}, "@me"},
		{[]string{"me"}, "me"},
		{[]string{"self"}, "self"},
		{[]string{"current_user"}, "current_user"},
		{[]string{"valid-user"}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		got := InvalidAssignee(tt.values)
		if got != tt.expected {
			t.Errorf("InvalidAssignee(%v) = %q, want %q", tt.values, got, tt.expected)
		}
	}
}

func TestBoundedIssueOpsText(t *testing.T) {
	tests := []struct {
		input      string
		wantSuffix string
	}{
		{"short text", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := boundedIssueOpsText(tt.input)
		if tt.input == "short text" && got != "short text" {
			t.Errorf("expected %q, got %q", tt.input, got)
		}
	}

	// Test truncation
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	got := boundedIssueOpsText(string(long))
	if len(got) != 400+len("...[truncated]") {
		t.Errorf("expected %d chars, got %d", 400+len("...[truncated]"), len(got))
	}
}

func TestIsDecimalString(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"12a", false},
		{"", false},
		{"1.5", false},
	}
	for _, tt := range tests {
		got := isDecimalString(tt.value)
		if got != tt.want {
			t.Errorf("isDecimalString(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestProviderFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/repo/issues/1", "github"},
		{"https://gitlab.com/user/repo/-/issues/1", "gitlab"},
		{"https://custom.gitlab.io/group/proj/-/issues/1", "gitlab"},
		{"https://unknown.com/issue/1", ""},
		{"", ""},
		{"not-a-url", ""},
	}
	for _, tt := range tests {
		got := ProviderFromURL(tt.url)
		if got != tt.want {
			t.Errorf("ProviderFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestIssueNumber(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/repo/issues/123", "123"},
		{"https://gitlab.com/user/repo/-/issues/456", "456"},
		{"https://example.com/no/issue", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := IssueNumber(tt.url)
		if got != tt.want {
			t.Errorf("IssueNumber(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestProjectKey(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		provider string
		kind     string
		want     string
	}{
		{"github issue", "https://github.com/user/repo/issues/1", "github", "issue", "github.com/user/repo"},
		{"github pr", "https://github.com/user/repo/pull/2", "github", "pr", "github.com/user/repo"},
		{"gitlab issue", "https://gitlab.com/group/subgroup/proj/-/issues/1", "gitlab", "issue", "gitlab.com/group/subgroup/proj"},
		{"gitlab work item", "https://gitlab.com/group/subgroup/proj/-/work_items/2", "gitlab", "issue", "gitlab.com/group/subgroup/proj"},
		{"gitlab mr", "https://gitlab.com/group/proj/-/merge_requests/2", "gitlab", "mr", "gitlab.com/group/proj"},
		{"gitlab issue preserves web port", "https://gitlab.example:8443/group/subgroup/proj/-/issues/1", "gitlab", "issue", "gitlab.example:8443/group/subgroup/proj"},
		{"gitlab mr preserves web port", "https://gitlab.example:8443/group/subgroup/proj/-/merge_requests/2", "gitlab", "mr", "gitlab.example:8443/group/subgroup/proj"},
		{"bad github issue path", "https://github.com/user/issues/1", "github", "issue", ""},
		{"empty", "", "github", "issue", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProjectKey(tt.url, tt.provider, tt.kind)
			if got != tt.want {
				t.Errorf("ProjectKey(%q, %q, %q) = %q, want %q", tt.url, tt.provider, tt.kind, got, tt.want)
			}
		})
	}
}

func TestValidateArtifactMatchesIssueRejectsMissingProjectAuthority(t *testing.T) {
	for _, tt := range []struct {
		issueURL, artifactURL, provider, kind string
	}{
		{"https://gitlab.example/-/issues/16", "https://gitlab.example/group/repo/-/merge_requests/16", "gitlab", "mr"},
		{"https://github.com/acme/issues/16", "https://github.com/acme/repo/pull/16", "github", "pr"},
	} {
		if err := ValidateArtifactMatchesIssue(tt.issueURL, tt.artifactURL, tt.provider, tt.kind); err == nil {
			t.Fatalf("missing project authority matched: issue=%q artifact=%q", tt.issueURL, tt.artifactURL)
		}
	}
}

func TestProjectKeyFromGitRemoteURLNormalizesHTTPSSSHAndSCP(t *testing.T) {
	for _, tt := range []struct {
		name, raw, provider, want string
	}{
		{"github https", "https://github.com/acme/repo.git", "github", "github.com/acme/repo"},
		{"github ssh", "ssh://git@github.com/acme/repo.git", "github", "github.com/acme/repo"},
		{"github scp", "git@github.com:acme/repo.git", "github", "github.com/acme/repo"},
		{"gitlab https port nested", "https://gitlab.example:8443/group/sub/repo.git", "gitlab", "gitlab.example:8443/group/sub/repo"},
		{"gitlab ssh port is transport only", "ssh://git@gitlab.example:2222/group/sub/repo.git", "gitlab", "gitlab.example/group/sub/repo"},
		{"gitlab scp nested", "git@gitlab.example:group/sub/repo.git", "gitlab", "gitlab.example/group/sub/repo"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProjectKeyFromGitRemoteURL(tt.raw, tt.provider)
			if err != nil || got != tt.want {
				t.Fatalf("ProjectKeyFromGitRemoteURL(%q) = %q, %v; want %q", tt.raw, got, err, tt.want)
			}
		})
	}
}

func TestRemoteProjectAuthorityNormalizesDefaultWebPortAndRejectsSSHWebPortAmbiguity(t *testing.T) {
	if got := ProjectKey("https://gitlab.example:443/group/repo/-/issues/1", "gitlab", "issue"); got != "gitlab.example/group/repo" {
		t.Fatalf("default HTTPS port project key = %q", got)
	}
	if err := ValidateGitRemoteMatchesIssue(
		"https://gitlab.example:8443/group/repo/-/issues/1",
		"ssh://git@gitlab.example:2222/group/repo.git",
		"gitlab",
	); err == nil || !strings.Contains(err.Error(), "web port") {
		t.Fatalf("custom SSH port versus web port ambiguity error = %v", err)
	}
	if err := ValidateGitRemoteMatchesIssue(
		"https://gitlab.example:8443/group/repo/-/issues/1",
		"https://gitlab.example:8443/group/repo.git",
		"gitlab",
	); err != nil {
		t.Fatalf("matching HTTPS web authority rejected: %v", err)
	}
}

func TestRemoteProjectAuthorityRejectsSchemeIPv6AndCredentialAmbiguity(t *testing.T) {
	if got := ProjectKey("http://gitlab.example/group/repo/-/issues/1", "gitlab", "issue"); got != "" {
		t.Fatalf("HTTP issue produced supervised project authority %q", got)
	}
	implicit := ProjectKey("https://[2001:db8::1]/group/repo/-/issues/1", "gitlab", "issue")
	explicit := ProjectKey("https://[2001:db8::1]:443/group/repo/-/issues/1", "gitlab", "issue")
	if implicit == "" || implicit != explicit {
		t.Fatalf("IPv6 HTTPS authority mismatch: implicit=%q explicit=%q", implicit, explicit)
	}
	for _, raw := range []string{
		"ssh://git:secret@gitlab.example/group/repo.git",
		"gitlab.example:group/repo.git",
		"git@[2001:db8::1]:group/repo.git",
	} {
		if _, err := ProjectKeyFromGitRemoteURL(raw, "gitlab"); err == nil || strings.Contains(err.Error(), raw) {
			t.Fatalf("ambiguous or credential-bearing remote was not safely rejected: err=%v", err)
		}
	}
}

func TestValidateArtifactURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		provider string
		kind     string
		wantErr  bool
	}{
		{"valid github pr", "https://github.com/user/repo/pull/1", "github", "pr", false},
		{"valid gitlab mr", "https://gitlab.com/group/proj/-/merge_requests/1", "gitlab", "mr", false},
		{"empty url", "", "github", "pr", true},
		{"whitespace url", "https://github.com/user/repo/pull/1\n", "github", "pr", true},
		{"wrong shape for github", "https://gitlab.com/user/repo/-/merge_requests/1", "github", "pr", true},
		{"unsupported provider", "https://bitbucket.org/user/repo/pull/1", "bitbucket", "pr", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactURL(tt.url, tt.provider, tt.kind)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArtifactURL(%q) error=%v, wantErr=%v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateChildMatchesParent(t *testing.T) {
	t.Run("same project", func(t *testing.T) {
		err := ValidateChildMatchesParent(
			"https://github.com/user/repo/issues/1",
			"https://github.com/user/repo/issues/2",
		)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	t.Run("different project", func(t *testing.T) {
		err := ValidateChildMatchesParent(
			"https://github.com/user/repo1/issues/1",
			"https://github.com/user/repo2/issues/2",
		)
		if err == nil {
			t.Error("expected error")
		}
	})
	t.Run("gitlab work item child", func(t *testing.T) {
		err := ValidateChildMatchesParent(
			"https://gitlab.com/group/project/-/issues/1",
			"https://gitlab.com/group/project/-/work_items/2",
		)
		if err != nil {
			t.Errorf("expected GitLab work item child URL to match parent project, got %v", err)
		}
	})
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
