package operationalhealth

import (
	"strings"
	"time"
)

func parseTaskCompletedAt(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
}
