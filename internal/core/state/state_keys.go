package state

import (
	"fmt"
	"regexp"
	"strings"
)

var stateKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func NormalizeStateKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("state key is required")
	}
	if strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) || !stateKeyRe.MatchString(key) {
		return "", fmt.Errorf("invalid state key %q; use [A-Za-z0-9._-] without path separators or '..', max 128 chars", key)
	}
	return key, nil
}
