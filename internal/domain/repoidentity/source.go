package repoidentity

import (
	"path/filepath"
	"strings"
)

// SourceRoot maps a Git common directory back to the primary checkout while
// preserving the caller's lexical path prefix (for example /var on macOS).
func SourceRoot(path, commonDir string) string {
	path = cleanAbsolute(path)
	if path == "" {
		return ""
	}
	commonDir = strings.TrimSpace(commonDir)
	if commonDir == "" {
		return path
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(path, commonDir)
	}
	commonDir = filepath.Clean(commonDir)
	if filepath.Base(commonDir) != ".git" {
		return path
	}
	return filepath.Dir(commonDir)
}

func cleanAbsolute(path string) string {
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
