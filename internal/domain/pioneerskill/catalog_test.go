package pioneerskill

import (
	"slices"
	"testing"
)

func TestCanonicalNamesAndMissingAreDeterministic(t *testing.T) {
	want := []string{
		"web-research",
		"requirements-analysis",
		"design-review",
		"database-design",
		"algorithm-optimization",
		"meeting-notes",
		"debugging",
		"prompt-engineering",
		"code-quality-metrics",
		"git-operations",
		"verified-execution",
		"implementation-planning",
	}
	names := Names()
	if !slices.Equal(names, want) {
		t.Fatalf("Names()=%v want=%v", names, want)
	}
	names[0] = "mutated"
	if Names()[0] != want[0] {
		t.Fatal("Names returned mutable catalog storage")
	}
	missing := Missing([]string{"verified-execution", "requirements-analysis", "verified-execution", "unknown"})
	wantMissing := []string{
		"web-research",
		"design-review",
		"database-design",
		"algorithm-optimization",
		"meeting-notes",
		"debugging",
		"prompt-engineering",
		"code-quality-metrics",
		"git-operations",
		"implementation-planning",
	}
	if !slices.Equal(missing, wantMissing) {
		t.Fatalf("Missing()=%v want=%v", missing, wantMissing)
	}
}
