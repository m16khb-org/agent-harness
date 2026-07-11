package worktreepath

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

func ShellCommandGuardPaths(repo, command string) []string {
	tokens := commandparse.SplitCommandTokens(command)
	out := []string{}
	seen := map[string]bool{}
	currentDir := CleanAbs(repo)
	for i, token := range tokens {
		addGitRepositoryOverridePath(&out, seen, currentDir, token)
		name := searchrouting.SearchTokenName(token)
		if (name == "git" || name == "go") && i+1 < len(tokens) {
			for j := i + 1; j < len(tokens); j++ {
				switch {
				case tokens[j] == "-C" && j+1 < len(tokens):
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, tokens[j+1]))
					j++
				case strings.HasPrefix(tokens[j], "-C="):
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(tokens[j], "-C=")))
				case name == "git" && (tokens[j] == "--git-dir" || tokens[j] == "--work-tree") && j+1 < len(tokens):
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, tokens[j+1]))
					j++
				case name == "git" && strings.HasPrefix(tokens[j], "--git-dir="):
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(tokens[j], "--git-dir=")))
				case name == "git" && strings.HasPrefix(tokens[j], "--work-tree="):
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(tokens[j], "--work-tree=")))
				case !strings.HasPrefix(tokens[j], "-"):
					j = len(tokens)
				}
			}
		}
		switch name {
		case "gofmt":
			if containsToken(tokens[i+1:], "-w") {
				addNonFlagOperands(&out, seen, currentDir, tokens[i+1:])
			}
		case "go":
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j] == "-o" && j+1 < len(tokens) {
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, tokens[j+1]))
					j++
				} else if strings.HasPrefix(tokens[j], "-o=") {
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(tokens[j], "-o=")))
				}
			}
		case "cp", "mv", "touch", "rm", "mkdir", "install", "truncate", "rsync":
			addNonFlagOperands(&out, seen, currentDir, tokens[i+1:])
		case "dd":
			for _, value := range tokens[i+1:] {
				if strings.HasPrefix(value, "of=") {
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(value, "of=")))
				}
			}
		case "bash", "sh", "zsh":
			for j := i + 1; j+1 < len(tokens); j++ {
				if shellEvalFlag(tokens[j]) {
					for _, nested := range ShellCommandGuardPaths(currentDir, tokens[j+1]) {
						addGuardPath(&out, seen, nested)
					}
					break
				}
			}
		}
		switch token {
		case "cd":
			if i+1 < len(tokens) {
				if path := resolveShellGuardPath(currentDir, tokens[i+1]); path != "" {
					addGuardPath(&out, seen, path)
					currentDir = path
				}
			}
		case "-C":
			if i > 0 && searchrouting.SearchTokenName(tokens[i-1]) == "git" && i+1 < len(tokens) {
				addGuardPath(&out, seen, resolveShellGuardPath(currentDir, tokens[i+1]))
			}
		case "add":
			if i > 1 && searchrouting.SearchTokenName(tokens[i-2]) == "git" && searchrouting.SearchTokenName(tokens[i-1]) == "worktree" {
				for _, value := range gitWorktreeAddTargets(tokens[i+1:]) {
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, value))
				}
			}
		case ">", ">>", "1>", "1>>", "2>", "2>>":
			if i+1 < len(tokens) {
				addGuardPath(&out, seen, resolveShellGuardPath(currentDir, tokens[i+1]))
			}
		default:
			for _, prefix := range []string{">>", ">", "1>>", "1>", "2>>", "2>"} {
				if strings.HasPrefix(token, prefix) && len(token) > len(prefix) {
					addGuardPath(&out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(token, prefix)))
					break
				}
			}
		}
		addConservativePathOperand(&out, seen, currentDir, token)
	}
	return out
}

func addGitRepositoryOverridePath(out *[]string, seen map[string]bool, currentDir, token string) {
	for _, prefix := range []string{"GIT_DIR=", "GIT_WORK_TREE=", "--git-dir=", "--work-tree="} {
		if strings.HasPrefix(token, prefix) {
			addGuardPath(out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(token, prefix)))
			return
		}
	}
}

func addConservativePathOperand(out *[]string, seen map[string]bool, currentDir, token string) {
	value := strings.TrimSpace(token)
	if value == "" {
		return
	}
	for _, prefix := range []string{"GIT_DIR=", "GIT_WORK_TREE="} {
		if strings.HasPrefix(value, prefix) {
			return
		}
	}
	for _, prefix := range []string{">>", ">", "1>>", "1>", "2>>", "2>"} {
		if strings.HasPrefix(value, prefix) {
			return
		}
	}
	for _, prefix := range []string{"--prefix=", "--directory=", "--chdir=", "--output=", "-C=", "-o="} {
		if strings.HasPrefix(value, prefix) {
			addGuardPath(out, seen, resolveShellGuardPath(currentDir, strings.TrimPrefix(value, prefix)))
			return
		}
	}
	if strings.HasPrefix(value, "-") || strings.Contains(value, "://") {
		return
	}
	if filepath.IsAbs(value) || value == "." || value == ".." || value == ".git" || strings.HasPrefix(value, "."+string(filepath.Separator)) || strings.HasPrefix(value, ".."+string(filepath.Separator)) || strings.Contains(value, string(filepath.Separator)) {
		addGuardPath(out, seen, resolveShellGuardPath(currentDir, value))
	}
}

func addNonFlagOperands(out *[]string, seen map[string]bool, currentDir string, tokens []string) {
	for i := 0; i < len(tokens); i++ {
		value := tokens[i]
		if value == "--" {
			for _, operand := range tokens[i+1:] {
				addGuardPath(out, seen, resolveShellGuardPath(currentDir, operand))
			}
			return
		}
		if strings.HasPrefix(value, "-") {
			if value == "-o" || value == "-C" || value == "-m" || value == "-t" || value == "-s" {
				i++
			}
			continue
		}
		addGuardPath(out, seen, resolveShellGuardPath(currentDir, value))
	}
}

func containsToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func shellEvalFlag(value string) bool {
	if !strings.HasPrefix(value, "-") || strings.HasPrefix(value, "--") {
		return false
	}
	return strings.Contains(strings.TrimPrefix(value, "-"), "c")
}

func IssueOpsPreparationCommand(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" || i+2 >= len(tokens) {
			continue
		}
		if searchrouting.SearchTokenName(tokens[i+1]) != "worktree" || searchrouting.SearchTokenName(tokens[i+2]) != "add" {
			continue
		}
		for _, value := range gitWorktreeAddTargets(tokens[i+3:]) {
			if IsInsideWorktreesPath(ResolveHookTargetPath("", value)) || strings.Contains(filepath.ToSlash(value), ".worktrees/") {
				return true
			}
		}
	}
	return false
}

func resolveShellGuardPath(currentDir, value string) string {
	path := strings.TrimSpace(value)
	if path == "" || strings.HasPrefix(path, "-") || strings.Contains(path, "$(") || strings.Contains(path, "`") {
		return ""
	}
	if filepath.IsAbs(path) {
		return CleanAbs(path)
	}
	base := CleanAbs(currentDir)
	if base == "" {
		return path
	}
	return CleanAbs(filepath.Join(base, path))
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

func addGuardPath(out *[]string, seen map[string]bool, value string) {
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
