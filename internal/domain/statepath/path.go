package statepath

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var keyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func NormalizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("state key is required")
	}
	if strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) || !keyRe.MatchString(key) {
		return "", fmt.Errorf("invalid state key %q; use [A-Za-z0-9._-] without path separators or '..', max 128 chars", key)
	}
	return key, nil
}

func ParseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty state timestamp")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
