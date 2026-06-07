package lifecycle

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
)

func shellCommandWorktreeGuardPaths(repo, command string) []string {
	tokens := commandparse.SplitCommandTokens(command)
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
