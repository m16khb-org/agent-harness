package issuebody

import (
	"strings"
	"testing"
)

func TestMergeIssueBodySectionIdempotent(t *testing.T) {
	sec := RenderDevilsAdvocateSection([]string{"gold-plating", "schedule optimism", "  "}, "2026-07-01T00:00:00Z")
	body := "original body\n"

	once := MergeIssueBodySection(body, sec)
	if !strings.Contains(once, "gold-plating") || !strings.HasPrefix(once, "original body") {
		t.Fatalf("append failed: %q", once)
	}
	if strings.Contains(once, "- \n") {
		t.Fatalf("blank finding should be dropped: %q", once)
	}

	sec2 := RenderDevilsAdvocateSection([]string{"new finding"}, "2026-07-02T00:00:00Z")
	twice := MergeIssueBodySection(once, sec2)
	if strings.Count(twice, startMarker) != 1 || strings.Count(twice, endMarker) != 1 {
		t.Fatalf("re-merge must not duplicate the block: %q", twice)
	}
	if strings.Contains(twice, "gold-plating") || !strings.Contains(twice, "new finding") {
		t.Fatalf("re-merge must replace the block content: %q", twice)
	}
	if !strings.HasPrefix(twice, "original body") {
		t.Fatalf("surrounding body must round-trip: %q", twice)
	}
}

func TestMergeIssueBodySectionEmptyBody(t *testing.T) {
	sec := RenderDevilsAdvocateSection([]string{"x"}, "t")
	got := MergeIssueBodySection("", sec)
	if !strings.Contains(got, "x") || !strings.HasPrefix(got, startMarker) {
		t.Fatalf("empty body should become just the section: %q", got)
	}
}
