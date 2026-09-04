package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const nativeBinaryName = "issueops"

// ResolveStableNativeRoot maps a linked Git worktree to the source checkout
// that owns the common .git directory. Normal checkouts remain unchanged.
func ResolveStableNativeRoot(root string) (string, error) {
	root = absClean(root)
	if root == "" {
		return "", fmt.Errorf("native install root is required")
	}
	gitPath := filepath.Join(root, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return root, nil
		}
		return "", fmt.Errorf("resolve native install root %s: %w", root, err)
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return root, nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("native install git metadata must be a directory or regular gitdir file: %s", gitPath)
	}

	gitdirValue, err := readSingleMetadataValue(gitPath, "gitdir:")
	if err != nil {
		return "", err
	}
	gitdir := resolveMetadataPath(root, gitdirValue)
	if info, err := os.Lstat(gitdir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		if err != nil {
			return "", fmt.Errorf("resolve worktree gitdir %s: %w", gitdir, err)
		}
		return "", fmt.Errorf("worktree gitdir must be a physical directory: %s", gitdir)
	}

	commondirValue, err := readSingleMetadataValue(filepath.Join(gitdir, "commondir"), "")
	if err != nil {
		return "", err
	}
	commondir := resolveMetadataPath(gitdir, commondirValue)
	commonInfo, err := os.Lstat(commondir)
	if err != nil {
		return "", fmt.Errorf("resolve worktree commondir %s: %w", commondir, err)
	}
	if !commonInfo.IsDir() || commonInfo.Mode()&os.ModeSymlink != 0 || filepath.Base(commondir) != ".git" {
		return "", fmt.Errorf("worktree commondir must be a physical .git directory: %s", commondir)
	}
	return filepath.Dir(commondir), nil
}

// ValidateStableNativeRuntime requires hooks to execute the installer-owned
// binary under the stable source checkout.
func ValidateStableNativeRuntime(root, binPath string) error {
	stableRoot, err := ResolveStableNativeRoot(root)
	if err != nil {
		return err
	}
	observed := absClean(binPath)
	relative, err := filepath.Rel(stableRoot, observed)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		expected := filepath.Join(stableRoot, "bin", nativeBinaryName)
		return fmt.Errorf("native runtime must stay inside the stable source root: observed=%s expected=%s", observed, expected)
	}
	return nil
}

func readSingleMetadataValue(path, prefix string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read git metadata %s: %w", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		return "", fmt.Errorf("git metadata must contain exactly one value: %s", path)
	}
	value := strings.TrimSpace(lines[0])
	if prefix != "" {
		if !strings.HasPrefix(value, prefix) {
			return "", fmt.Errorf("gitdir metadata is malformed: %s", path)
		}
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	if value == "" {
		return "", fmt.Errorf("git metadata value is empty: %s", path)
	}
	return value, nil
}

func resolveMetadataPath(base, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}
