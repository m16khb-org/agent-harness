package operationalhealth

import (
	"strings"
	"time"
)

const (
	legacyTaskCompletedAtLayout            = "2006-01-02 15:04:05"
	taskCompletedAtCompatibilityContractV1 = 1
)

func parseTaskCompletedAt(raw string) (time.Time, error) {
	return parseTaskCompletedAtForVersion(raw, taskCompletedAtCompatibilityContractV1)
}

func parseTaskCompletedAtForVersion(raw string, contractVersion int) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed, nil
	}
	// Orca payload contract v1 emitted this UTC layout. The payload has no
	// version field, so expiry is a source-controlled migration: after every
	// supported Orca CLI release is observed returning RFC3339Nano completed_at
	// values and its release contract drops the legacy layout, bump this
	// constant and update TestTaskCompletedAtCompatibilityExpiresAfterContractV1
	// in the same change.
	if contractVersion != taskCompletedAtCompatibilityContractV1 {
		return time.Parse(time.RFC3339Nano, trimmed)
	}
	if len(raw) != len(legacyTaskCompletedAtLayout) {
		return time.Time{}, &time.ParseError{Layout: legacyTaskCompletedAtLayout, Value: raw}
	}
	return time.ParseInLocation(legacyTaskCompletedAtLayout, raw, time.UTC)
}
