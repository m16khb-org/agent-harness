package issueops

import (
	"os"
	"path/filepath"
	"strings"
)

func cleanAbsPath(path string) string {
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

func pathWithin(path, root string) bool {
	p := cleanAbsPath(path)
	r := cleanAbsPath(root)
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

func isInsideWorktreesPath(target string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(target), "/") {
		if strings.HasSuffix(segment, ".worktrees") {
			return true
		}
	}
	return false
}

func gitBranchFromHead(repo string) string {
	root := cleanAbsPath(repo)
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
