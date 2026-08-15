package pioneerskill

import (
	"slices"
	"testing"
)

func TestCanonicalNamesAndMissingAreDeterministic(t *testing.T) {
	want := []string{
		"berners-lee",
		"boehm",
		"brooks",
		"codd",
		"dijkstra",
		"engelbart",
		"hopper",
		"karpathy",
		"shannon",
		"torvalds",
		"turing",
		"von-neumann",
	}
	names := Names()
	if !slices.Equal(names, want) {
		t.Fatalf("Names()=%v want=%v", names, want)
	}
	names[0] = "mutated"
	if Names()[0] != want[0] {
		t.Fatal("Names returned mutable catalog storage")
	}
	missing := Missing([]string{"turing", "boehm", "turing", "unknown"})
	wantMissing := []string{
		"berners-lee",
		"brooks",
		"codd",
		"dijkstra",
		"engelbart",
		"hopper",
		"karpathy",
		"shannon",
		"torvalds",
		"von-neumann",
	}
	if !slices.Equal(missing, wantMissing) {
		t.Fatalf("Missing()=%v want=%v", missing, wantMissing)
	}
}
