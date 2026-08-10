package operationalhealth

import (
	"strings"
	"time"
)

const legacyTaskCompletedAtLayout = "2006-01-02 15:04:05"

func parseTaskCompletedAt(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw)); err == nil {
		return parsed, nil
	}
	if len(raw) != len(legacyTaskCompletedAtLayout) {
		return time.Time{}, &time.ParseError{Layout: legacyTaskCompletedAtLayout, Value: raw}
	}
	return time.ParseInLocation(legacyTaskCompletedAtLayout, raw, time.UTC)
}
