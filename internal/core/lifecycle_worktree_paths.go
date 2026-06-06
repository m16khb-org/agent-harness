package core

import (
	"os"
	"path/filepath"
	"strings"
)

func worktreeGuardTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	if repo := cleanAbsPath(req.Repo); repo != "" {
		targets = append(targets, repo)
	}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

func worktreeGuardEditTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && isShellTool(req.Tool) {
		for _, path := range shellCommandWorktreeGuardPaths(req.Repo, req.Command) {
			if target := resolveHookTargetPath(req.Repo, path); target != "" {
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		if repo := cleanAbsPath(req.Repo); repo != "" {
			targets = append(targets, repo)
		}
	}
	return targets
}

func shellCommandWorktreeGuardPaths(repo, command string) []string {
	tokens := splitCommandTokens(command)
	out := []string{}
	seen := map[string]bool{}
	currentDir := cleanAbsPath(repo)
	for i, token := range tokens {
		switch token {
		case "cd":
			if i+1 < len(tokens) {
				if path := resolveShellWorktreeGuardPath(currentDir, tokens[i+1]); path != "" {
					addWorktreeGuardPath(&out, seen, path)
					currentDir = path
				}
			}
		case "-C":
			if i > 0 && searchTokenName(tokens[i-1]) == "git" && i+1 < len(tokens) {
				addWorktreeGuardPath(&out, seen, resolveShellWorktreeGuardPath(currentDir, tokens[i+1]))
			}
		case "add":
			if i > 1 && searchTokenName(tokens[i-2]) == "git" && searchTokenName(tokens[i-1]) == "worktree" {
				for _, value := range gitWorktreeAddTargets(tokens[i+1:]) {
					addWorktreeGuardPath(&out, seen, resolveShellWorktreeGuardPath(currentDir, value))
				}
			}
		case ">", ">>", "1>", "1>>", "2>", "2>>":
			if i+1 < len(tokens) {
				addWorktreeGuardPath(&out, seen, resolveShellWorktreeGuardPath(currentDir, tokens[i+1]))
			}
		default:
			for _, prefix := range []string{">>", ">", "1>>", "1>", "2>>", "2>"} {
				if strings.HasPrefix(token, prefix) && len(token) > len(prefix) {
					addWorktreeGuardPath(&out, seen, resolveShellWorktreeGuardPath(currentDir, strings.TrimPrefix(token, prefix)))
					break
				}
			}
		}
	}
	return out
}

func resolveShellWorktreeGuardPath(currentDir, value string) string {
	path := strings.TrimSpace(value)
	if path == "" || strings.HasPrefix(path, "-") || strings.Contains(path, "$(") || strings.Contains(path, "`") {
		return ""
	}
	if filepath.IsAbs(path) {
		return cleanAbsPath(path)
	}
	base := cleanAbsPath(currentDir)
	if base == "" {
		return path
	}
	return cleanAbsPath(filepath.Join(base, path))
}

func gitWorktreeAddTargets(args []string) []string {
	out := []string{}
	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		if token == "" {
			continue
		}
		if token == "--" {
			if i+1 < len(args) {
				out = append(out, args[i+1])
			}
			return out
		}
		if strings.HasPrefix(token, "-") {
			if token == "-b" || token == "-B" {
				i++
			}
			continue
		}
		out = append(out, token)
		return out
	}
	return out
}

func addWorktreeGuardPath(out *[]string, seen map[string]bool, value string) {
	path := strings.TrimSpace(value)
	if path == "" || strings.HasPrefix(path, "-") || strings.Contains(path, "$(") || strings.Contains(path, "`") {
		return
	}
	if seen[path] {
		return
	}
	seen[path] = true
	*out = append(*out, path)
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

func isInsideWorktreesPath(target string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(target), "/") {
		if strings.HasSuffix(segment, ".worktrees") {
			return true
		}
	}
	return false
}

func resolveHookTargetPath(repo, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return cleanAbsPath(p)
	}
	base := cleanAbsPath(repo)
	if base == "" {
		return ""
	}
	return cleanAbsPath(filepath.Join(base, p))
}

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
