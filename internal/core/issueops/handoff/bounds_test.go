package handoff

import (
	"fmt"
	"strings"
	"testing"
)

func TestCanonicalBaselineIDsBoundsAndCanonicalization(t *testing.T) {
	values := make([]string, 0, MaxBaselineIDs+2)
	for i := MaxBaselineIDs - 1; i >= 0; i-- {
		values = append(values, fmt.Sprintf("id-%03d", i))
	}
	values = append(values, " id-000 ", "id-001")
	got, err := CanonicalBaselineIDs("task", values)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxBaselineIDs || got[0] != "id-000" || got[len(got)-1] != fmt.Sprintf("id-%03d", MaxBaselineIDs-1) {
		t.Fatalf("canonical baseline = len %d first %q last %q", len(got), got[0], got[len(got)-1])
	}
	overflow := append(append([]string(nil), got...), "overflow")
	if _, err := CanonicalBaselineIDs("task", overflow); err == nil {
		t.Fatal("count overflow must fail instead of truncating")
	}
	if _, err := CanonicalBaselineIDs("task", []string{strings.Repeat("x", MaxExternalIDBytes+1)}); err == nil {
		t.Fatal("id length overflow must fail")
	}
}

func TestCanonicalBaselineIDsUsesPerKindAndTotalByteBounds(t *testing.T) {
	longWorktreeID := "worktree-uuid::/" + strings.Repeat("deep/", 70)
	if len(longWorktreeID) <= 256 {
		t.Fatal("fixture must exercise a valid worktree locator above 256 bytes")
	}
	if _, err := CanonicalBaselineIDs("worktree", []string{longWorktreeID}); err != nil {
		t.Fatalf("valid path-derived worktree ID rejected: %v", err)
	}
	if _, err := CanonicalBaselineIDs("terminal", []string{longWorktreeID}); err == nil {
		t.Fatal("terminal IDs must retain a smaller per-kind bound")
	}

	const expectedTotalByteBudget = 256 * 1024
	items := make([]string, 0, MaxBaselineIDs)
	for i := 0; i < MaxBaselineIDs; i++ {
		items = append(items, fmt.Sprintf("%04d-%s", i, strings.Repeat("w", expectedTotalByteBudget/MaxBaselineIDs)))
	}
	if _, err := CanonicalBaselineIDs("worktree", items); err == nil {
		t.Fatal("aggregate byte overflow must fail instead of truncating")
	}
}
