package worktreeguard

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

type BranchCreation struct {
	Branch    string
	SourceRef string
}

func LocalIssueOpsBranchCreation(command string) BranchCreation {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" || i+1 >= len(tokens) {
			continue
		}
		switch searchrouting.SearchTokenName(tokens[i+1]) {
		case "checkout":
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j] == "-b" || tokens[j] == "-B" {
					if j+1 < len(tokens) {
						return BranchCreation{
							Branch:    strings.TrimSpace(tokens[j+1]),
							SourceRef: nextBranchSourceRef(tokens, j+2),
						}
					}
					return BranchCreation{}
				}
			}
		case "switch":
			for j := i + 2; j < len(tokens); j++ {
				if tokens[j] == "-c" || tokens[j] == "-C" || strings.HasPrefix(tokens[j], "--create") {
					if strings.Contains(tokens[j], "=") {
						_, value, _ := strings.Cut(tokens[j], "=")
						return BranchCreation{
							Branch:    strings.TrimSpace(value),
							SourceRef: nextBranchSourceRef(tokens, j+1),
						}
					}
					if j+1 < len(tokens) {
						return BranchCreation{
							Branch:    strings.TrimSpace(tokens[j+1]),
							SourceRef: nextBranchSourceRef(tokens, j+2),
						}
					}
					return BranchCreation{}
				}
			}
		case "worktree":
			if i+2 < len(tokens) && searchrouting.SearchTokenName(tokens[i+2]) == "add" {
				return localIssueOpsWorktreeBranchCreation(tokens[i+3:])
			}
		}
	}
	return BranchCreation{}
}

func localIssueOpsWorktreeBranchCreation(args []string) BranchCreation {
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
				return BranchCreation{Branch: branch, SourceRef: nextBranchSourceRef(args, i+1)}
			}
			return BranchCreation{}
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
			return BranchCreation{Branch: branch, SourceRef: token}
		}
	}
	if branch != "" {
		return BranchCreation{Branch: branch}
	}
	return BranchCreation{}
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

func IssueOpsBranchCreationSourceReason(branch string) string {
	return fmt.Sprintf("IssueOps branch creation must include an explicit source ref chosen by the user; ask the user which source branch or commit to branch from, then rerun with a source ref such as git switch -c %s origin/main", branch)
}

func ShellTokenLooksDynamic(token string) bool {
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	return strings.Contains(token, "$") || strings.Contains(token, "`")
}
