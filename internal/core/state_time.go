package core

import (
	"fmt"
	"time"
)

func parseStateTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty state timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}
