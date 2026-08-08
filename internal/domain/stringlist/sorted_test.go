package stringlist

import "testing"

func TestUniqueSortedDropsEmptyDuplicatesAndSorts(t *testing.T) {
	got := UniqueSorted([]string{"beta", "", "alpha", "beta", "gamma", "alpha"})
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}
