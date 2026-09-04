package projectdoc

import "testing"

func TestOptionalVCSProjectDocIsAllowedButNotRequired(t *testing.T) {
	if containsProjectDocName(ProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must not become a required project doc")
	}
	if !containsProjectDocName(AllowedProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must be readable and writable on demand")
	}
	if got, err := NormalizeRelPath(".issueops/VCS.md"); err != nil || got != ".issueops/VCS.md" {
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

func TestNormalizeRelPathAllowsFamilyModuleDocs(t *testing.T) {
	for _, rel := range []string{"adr/overview.md", "adr/2026-08-20-some-decision.md", "testing/overview.md", "operations/guides/overview.md", "cautions/2026-08-20-risk.md"} {
		got, err := NormalizeRelPath(rel)
		if err != nil {
			t.Fatalf("family module path %q rejected: %v", rel, err)
		}
		if want := ".issueops/" + rel; got != want {
			t.Fatalf("NormalizeRelPath(%q) = %q, want %q", rel, got, want)
		}
	}
	for _, rel := range []string{"adr/../ADR.md", "adr//x.md", "adr/record.txt", "adr/", "unknown/x.md", "VCS/../ADR.md"} {
		if _, err := NormalizeRelPath(rel); err == nil {
			t.Fatalf("invalid module path %q must be rejected", rel)
		}
	}
}
