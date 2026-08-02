package issueopscompletion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	completioncontract "agent-harness/internal/contract/issueopscompletion"
	completiondomain "agent-harness/internal/domain/issueopscompletion"
)

type Environment struct{}

func NewEnvironment() Environment { return Environment{} }

func (Environment) VerifyArtifact(record completioncontract.RecordSnapshot, requestedURL string) error {
	return completiondomain.ValidateArtifact(record.Clone(), requestedURL)
}

func (Environment) PathsMatch(left, right string) bool {
	leftPath, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(leftPath); resolveErr == nil {
		leftPath = resolved
	}
	rightPath, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(rightPath); resolveErr == nil {
		rightPath = resolved
	}
	return filepath.Clean(leftPath) == filepath.Clean(rightPath)
}

func (Environment) CurrentHead(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (Environment) VerifyReport(root, path string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("Turing report must exist in the canonical worktree")
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Turing report must be inside the canonical worktree")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Turing report must be a regular file")
	}
	return resolved, nil
}
