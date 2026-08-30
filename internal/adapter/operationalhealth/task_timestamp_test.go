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

func TestParseTaskCompletedAtParsesLegacyUTC(t *testing.T) {
	want := time.Date(2026, 8, 3, 22, 35, 17, 0, time.UTC)
	got, err := parseTaskCompletedAt("2026-08-03 22:35:17")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("timestamp=%s location=%s want=%s UTC", got, got.Location(), want)
	}
}

func TestParseTaskCompletedAtRejectsUnsupportedLegacyFormats(t *testing.T) {
	for _, raw := range []string{
		"2026/08/03 22:35:17",
		"2026-08-03 22:35:17Z",
		"2026-08-03 22:35:17.1",
		"2026-08-03 22:35:17,1",
		" 2026-08-03 22:35:17",
		"2026-08-03 22:35:17 ",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := parseTaskCompletedAt(raw); err == nil {
				t.Fatalf("parseTaskCompletedAt(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestTaskCompletedAtCompatibilityExpiresAfterContractV1(t *testing.T) {
	const legacy = "2026-08-03 22:35:17"
	if _, err := parseTaskCompletedAtForVersion(legacy, 1); err != nil {
		t.Fatalf("contract v1 legacy timestamp: %v", err)
	}
	if _, err := parseTaskCompletedAtForVersion(legacy, 2); err == nil {
		t.Fatal("contract v2 accepted the legacy timestamp")
	}
	if _, err := parseTaskCompletedAtForVersion("2026-08-03T22:35:17Z", 2); err != nil {
		t.Fatalf("contract v2 rejected RFC3339: %v", err)
	}
}
