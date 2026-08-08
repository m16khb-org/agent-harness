package state

import (
	"testing"
	"time"

	statecontract "agent-harness/internal/contract/state"
)

func TestSelectPrunePreservesPrefixAgeAndCountRules(t *testing.T) {
	records := []statecontract.StateListEntry{
		{Key: "other-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-old", UpdatedAt: "2026-01-01T00:00:00Z"},
		{Key: "trace-newer", UpdatedAt: "2026-01-03T00:00:00Z"},
		{Key: "trace-newest", UpdatedAt: "2026-01-04T00:00:00Z"},
	}
	pruned, kept := SelectPrune(records, "trace-", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), 1)
	if got, want := entryKeys(pruned), []string{"trace-newer", "trace-old"}; !sameStrings(got, want) {
		t.Fatalf("pruned=%v want %v", got, want)
	}
	if got, want := entryKeys(kept), []string{"other-old", "trace-newest"}; !sameStrings(got, want) {
		t.Fatalf("kept=%v want %v", got, want)
	}
}

func entryKeys(entries []statecontract.StateListEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
