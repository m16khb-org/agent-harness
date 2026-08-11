package issueopsauthorization

import (
	"path/filepath"
	"strings"
)

type CanonicalPaths struct{}

func (CanonicalPaths) Same(left, right string) bool {
	left, err := canonicalPath(left)
	if err != nil {
		return false
	}
	right, err = canonicalPath(right)
	return err == nil && left == right
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute), nil
}
