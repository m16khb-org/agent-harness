package adapter

import (
	"path/filepath"
	"strings"
)

type FilesystemPathMatcher struct{}

func (FilesystemPathMatcher) Matches(left, right string) bool {
	leftPath, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	rightPath, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(leftPath); err == nil {
		leftPath = resolved
	}
	if resolved, err := filepath.EvalSymlinks(rightPath); err == nil {
		rightPath = resolved
	}
	return filepath.Clean(leftPath) == filepath.Clean(rightPath)
}
