package worktreepath

import (
	"os"
	"path/filepath"
	"strings"
)

func GitBranchFromHead(repo string) string {
	root := CleanAbs(repo)
	if root == "" {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	headPath := filepath.Join(gitPath, "HEAD")
	if info, err := os.Stat(gitPath); err == nil && !info.IsDir() {
		if b, err := os.ReadFile(gitPath); err == nil {
			line := strings.TrimSpace(string(b))
			if rest, ok := strings.CutPrefix(line, "gitdir:"); ok {
				resolved := strings.TrimSpace(rest)
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(root, resolved)
				}
				headPath = filepath.Join(resolved, "HEAD")
			}
		}
	}
	b, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if rest, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return strings.TrimSpace(rest)
	}
	return ""
}

// SourceCheckout resolves a linked worktree's public .git pointer without
// spawning Git. A main checkout resolves to itself.
func SourceCheckout(repo string) string {
	root := CleanAbs(repo)
	if root == "" {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return root
	}
	b, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(b))
	gitDir, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return ""
	}
	gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	worktreesDir := filepath.Dir(gitDir)
	commonGitDir := filepath.Dir(worktreesDir)
	if filepath.Base(worktreesDir) != "worktrees" || filepath.Base(commonGitDir) != ".git" {
		return ""
	}
	return filepath.Dir(commonGitDir)
}

func IsInsideWorktreesPath(target string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(target), "/") {
		if strings.HasSuffix(segment, ".worktrees") {
			return true
		}
	}
	return false
}

func ResolveHookTargetPath(repo, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return CleanAbs(p)
	}
	base := CleanAbs(repo)
	if base == "" {
		return ""
	}
	return CleanAbs(filepath.Join(base, p))
}

func CleanAbs(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return filepath.Clean(p)
		}
		p = abs
	}
	return filepath.Clean(p)
}

func Within(path, root string) bool {
	p := CleanAbs(path)
	r := CleanAbs(root)
	if p == "" || r == "" {
		return false
	}
	if p == r {
		return true
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
