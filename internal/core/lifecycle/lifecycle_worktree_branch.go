package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"
)

type issueOpsBranchCreation struct {
	Branch    string
	SourceRef string
}

func localIssueOpsBranchCreation(command string) issueOpsBranchCreation {
	tokens := splitCommandTokens(command)
	for i, token := range tokens {
		if searchTokenName(token) != "git" || i+1 >= len(tokens) {
			continue
		}
		switch searchTokenName(tokens[i+1]) {
		case "checkout":
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j] == "-b" || tokens[j] == "-B" {
					if j+1 < len(tokens) {
						return issueOpsBranchCreation{
							Branch:    strings.TrimSpace(tokens[j+1]),
							SourceRef: nextBranchSourceRef(tokens, j+2),
						}
					}
					return issueOpsBranchCreation{}
				}
			}
		case "switch":
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j] == "-c" || tokens[j] == "-C" || strings.HasPrefix(tokens[j], "--create") {
					if strings.Contains(tokens[j], "=") {
						_, value, _ := strings.Cut(tokens[j], "=")
						return issueOpsBranchCreation{
							Branch:    strings.TrimSpace(value),
							SourceRef: nextBranchSourceRef(tokens, j+1),
						}
					}
					if j+1 < len(tokens) {
						return issueOpsBranchCreation{
							Branch:    strings.TrimSpace(tokens[j+1]),
							SourceRef: nextBranchSourceRef(tokens, j+2),
						}
					}
					return issueOpsBranchCreation{}
				}
			}
		case "worktree":
			if i+2 < len(tokens) && searchTokenName(tokens[i+2]) == "add" {
				return localIssueOpsWorktreeBranchCreation(tokens[i+3:])
			}
		}
	}
	return issueOpsBranchCreation{}
}

func localIssueOpsWorktreeBranchCreation(args []string) issueOpsBranchCreation {
	branch := ""
	pathSeen := false
	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		if token == "" {
			continue
		}
		if token == "--" {
			if !pathSeen && i+1 < len(args) {
				pathSeen = true
				i++
			}
			if branch != "" {
				return issueOpsBranchCreation{Branch: branch, SourceRef: nextBranchSourceRef(args, i+1)}
			}
			return issueOpsBranchCreation{}
		}
		if strings.HasPrefix(token, "-") {
			if token == "-b" || token == "-B" {
				if i+1 < len(args) {
					branch = strings.TrimSpace(args[i+1])
					i++
				}
			}
			continue
		}
		if !pathSeen {
			pathSeen = true
			continue
		}
		if branch != "" {
			return issueOpsBranchCreation{Branch: branch, SourceRef: token}
		}
	}
	if branch != "" {
		return issueOpsBranchCreation{Branch: branch}
	}
	return issueOpsBranchCreation{}
}

func nextBranchSourceRef(tokens []string, start int) string {
	for i := start; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])
		if token == "" || strings.HasPrefix(token, "-") {
			continue
		}
		return token
	}
	return ""
}

func issueOpsBranchCreationSourceReason(branch string) string {
	return fmt.Sprintf("IssueOps branch creation must include an explicit source ref chosen by the user; ask the user which source branch or commit to branch from, then rerun with a source ref such as git switch -c %s origin/main", branch)
}

func shellTokenLooksDynamic(token string) bool {
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	return strings.Contains(token, "$") || strings.Contains(token, "`")
}

func issueOpsWorktreePreparationCommand(command string) bool {
	tokens := splitCommandTokens(command)
	for i, token := range tokens {
		if searchTokenName(token) != "git" || i+2 >= len(tokens) {
			continue
		}
		if searchTokenName(tokens[i+1]) != "worktree" || searchTokenName(tokens[i+2]) != "add" {
			continue
		}
		for _, value := range gitWorktreeAddTargets(tokens[i+3:]) {
			if isInsideWorktreesPath(resolveHookTargetPath("", value)) || strings.Contains(filepath.ToSlash(value), ".worktrees/") {
				return true
			}
		}
	}
	return false
}
