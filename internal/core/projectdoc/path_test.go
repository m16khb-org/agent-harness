package projectdoc

import "testing"

func TestOptionalVCSProjectDocIsAllowedButNotRequired(t *testing.T) {
	if containsProjectDocName(ProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must not become a required project doc")
	}
	if !containsProjectDocName(AllowedProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must be readable and writable on demand")
	}
	if got, err := NormalizeRelPath(".agent-harness/VCS.md"); err != nil || got != ".agent-harness/VCS.md" {
		t.Fatalf("NormalizeRelPath(VCS.md) = %q, %v", got, err)
	}
}

func containsProjectDocName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
