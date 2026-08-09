package operationalhealth

import (
	"testing"
	"time"
)

func TestParseTaskCompletedAtPreservesRFC3339NanoBehavior(t *testing.T) {
	want := time.Date(2026, 8, 9, 15, 24, 1, 123456789, time.FixedZone("KST", 9*60*60))
	got, err := parseTaskCompletedAt("  " + want.Format(time.RFC3339Nano) + "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Fatalf("timestamp=%s want=%s", got, want)
	}
}
