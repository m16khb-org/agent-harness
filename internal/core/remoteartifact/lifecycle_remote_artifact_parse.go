package remoteartifact

import (
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

type remoteArtifactCommand struct {
	provider        string
	kind            string
	action          string
	createFromIssue bool
	title           string
	body            string
	bodyFilePath    string
	labels          []string
	assignees       []string
	headBranch      string
	baseBranch      string
	// target은 편집 대상 식별자다(첫 비플래그 positional 인자). 번호와 URL 두
	// 형태가 그대로 들어오며, 어느 쪽인지 해석하는 책임은 소비자에게 있다.
	target string
}

func parseGHRemoteArtifactCommand(command string, repo string) (remoteArtifactCommand, bool) {
	tokens := commandparse.SplitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		cli := searchrouting.SearchTokenName(tokens[i])
		provider := ""
		switch cli {
		case "gh":
			provider = "github"
		case "glab":
			provider = "gitlab"
		default:
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(tokens[i+1]))
		action := strings.ToLower(strings.TrimSpace(tokens[i+2]))
		if kind != "issue" && kind != "pr" && kind != "mr" {
			continue
		}
		if action == "for" || action == "new-for" || action == "create-for" {
			artifactCreateFromIssue := true
			action = "create"
			artifact := remoteArtifactCommand{provider: provider, kind: kind, action: action, createFromIssue: artifactCreateFromIssue}
			args := tokens[i+3:]
			if remoteArtifactHelpRequested(args) {
				return remoteArtifactCommand{}, false
			}
			parseRemoteArtifactArgs(&artifact, repo, args)
			fillRemoteArtifactInlineBodyFile(&artifact, command)
			return artifact, true
		}
		if action != "create" && action != "edit" && action != "update" {
			continue
		}
		artifact := remoteArtifactCommand{provider: provider, kind: kind, action: action}
		args := tokens[i+3:]
		if remoteArtifactHelpRequested(args) {
			return remoteArtifactCommand{}, false
		}
		parseRemoteArtifactArgs(&artifact, repo, args)
		fillRemoteArtifactInlineBodyFile(&artifact, command)
		return artifact, true
	}
	return remoteArtifactCommand{}, false
}

func remoteArtifactHelpRequested(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--help", "-h":
			return true
		}
	}
	return false
}
