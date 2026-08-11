package issueopsinventory

import (
	"path/filepath"
	"strings"
	"time"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type CleanPath struct{}

func (CleanPath) Normalize(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
	}
	return filepath.Clean(path)
}
