package providerutil

import (
	"reflect"
	"testing"
)

func TestMissingStringsTrimsSkipsEmptyAndPreservesWantedOrder(t *testing.T) {
	got := MissingStrings(
		[]string{" bug ", "", "octocat", "feature", "bug"},
		[]string{"bug", " other "},
	)
	want := []string{"octocat", "feature"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingStrings() = %#v, want %#v", got, want)
	}
}

func TestFirstNonEmptyReturnsFirstTrimmedValue(t *testing.T) {
	if got := FirstNonEmpty("", " \t", " https://example.test/item/1 ", "later"); got != "https://example.test/item/1" {
		t.Fatalf("FirstNonEmpty() = %q", got)
	}
}
