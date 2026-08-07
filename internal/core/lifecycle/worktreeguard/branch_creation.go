package worktreeguard

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/domain/searchrouting"
)

type BranchCreation struct {
	Branch    string
	SourceRef string
}

// BranchSelection describes an existing branch checkout. It intentionally
// excludes `git checkout -- <path>` so ordinary path operations are not
// mistaken for IssueOps topology changes.
type BranchSelection struct {
	Branch  string
	Dynamic bool
}

func LocalIssueOpsBranchSelection(command string) BranchSelection {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" || i+2 >= len(tokens) {
			continue
		}
		subcommand := searchrouting.SearchTokenName(tokens[i+1])
		if subcommand != "checkout" && subcommand != "switch" {
			continue
		}
		args := tokens[i+2:]
		if len(args) == 0 || args[0] == "--" || containsBranchCreateFlag(args) {
			continue
		}
		for _, arg := range args {
			if arg == "--" {
				break
			}
			if strings.HasPrefix(arg, "-") {
				continue
			}
			branch := strings.TrimSpace(arg)
			return BranchSelection{Branch: branch, Dynamic: ShellTokenLooksDynamic(branch)}
		}
	}
	return BranchSelection{}
}

func containsBranchCreateFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-b" || arg == "-B" || arg == "-c" || arg == "-C" || arg == "--create" || strings.HasPrefix(arg, "--create=") {
			return true
		}
	}
	return false
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

func DirectGitWorktreeMutation(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" {
			continue
		}
		worktreeAt := commandparse.CommandAfterDirectoryOption(tokens, i+1)
		if worktreeAt < 0 || worktreeAt >= len(tokens) || searchrouting.SearchTokenName(tokens[worktreeAt]) != "worktree" {
			continue
		}
		actionAt := commandparse.CommandAfterDirectoryOption(tokens, worktreeAt+1)
		if actionAt < 0 || actionAt >= len(tokens) {
			continue
		}
		switch searchrouting.SearchTokenName(tokens[actionAt]) {
		case "add", "lock", "move", "prune", "remove", "repair", "unlock":
			return true
		}
	}
	return false
}

func SealedGitTopologyMutation(command string) bool {
	if strings.TrimSpace(LocalIssueOpsBranchCreation(command).Branch) != "" ||
		strings.TrimSpace(LocalIssueOpsBranchSelection(command).Branch) != "" || DirectGitWorktreeMutation(command) {
		return true
	}
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" {
			continue
		}
		actionAt := commandparse.CommandAfterDirectoryOption(tokens, i+1)
		if actionAt < 0 || actionAt >= len(tokens) {
			continue
		}
		switch searchrouting.SearchTokenName(tokens[actionAt]) {
		case "branch":
			if _, ok := exactMatchingOriginUpstream(tokens[actionAt+1:]); ok {
				continue
			}
			return true
		case "cherry-pick", "merge", "rebase", "reset", "revert":
			return true
		case "push":
			for _, arg := range tokens[actionAt+1:] {
				if arg == "-f" || strings.HasPrefix(arg, "--force") || arg == "--mirror" || arg == "--delete" ||
					strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, ":") {
					return true
				}
			}
		}
	}
	return false
}

// MatchingOriginUpstreamBranch는 origin의 같은 이름 원격 브랜치를 명시적으로
// tracking하는 한 가지 비파괴적 branch 설정에서 로컬 브랜치 이름을 돌려준다.
func MatchingOriginUpstreamBranch(command string) (string, bool) {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "git" {
			continue
		}
		actionAt := commandparse.CommandAfterDirectoryOption(tokens, i+1)
		if actionAt < 0 || actionAt >= len(tokens) || searchrouting.SearchTokenName(tokens[actionAt]) != "branch" {
			continue
		}
		return exactMatchingOriginUpstream(tokens[actionAt+1:])
	}
	return "", false
}

func exactMatchingOriginUpstream(args []string) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	upstream, branch := "", ""
	switch {
	case len(args) == 2 && strings.HasPrefix(args[0], "--set-upstream-to="):
		upstream = strings.TrimPrefix(args[0], "--set-upstream-to=")
		branch = args[1]
	case len(args) == 3 && args[0] == "--set-upstream-to":
		upstream = args[1]
		branch = args[2]
	default:
		return "", false
	}
	upstream = strings.TrimSpace(upstream)
	branch = strings.TrimSpace(branch)
	if upstream == "" || branch == "" || strings.HasPrefix(branch, "-") ||
		ShellTokenLooksDynamic(upstream) || ShellTokenLooksDynamic(branch) {
		return "", false
	}
	for _, prefix := range []string{"origin/", "refs/remotes/origin/"} {
		if strings.HasPrefix(upstream, prefix) {
			return branch, strings.TrimPrefix(upstream, prefix) == branch
		}
	}
	return "", false
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
