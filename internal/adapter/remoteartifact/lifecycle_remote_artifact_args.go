package remoteartifact

import "strings"

func parseRemoteArtifactArgs(artifact *remoteArtifactCommand, repo string, args []string) {
	for j := 0; j < len(args); j++ {
		j = parseRemoteArtifactArg(artifact, repo, args, j)
	}
}

func parseRemoteArtifactArg(artifact *remoteArtifactCommand, repo string, args []string, index int) int {
	arg := args[index]
	switch {
	case arg == "--title" || arg == "-t":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.title = value
		})
	case strings.HasPrefix(arg, "--title="):
		artifact.title = strings.TrimPrefix(arg, "--title=")
	case arg == "--body" || arg == "-b" || arg == "--description" || arg == "-d":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.body = value
		})
	case strings.HasPrefix(arg, "--body="):
		artifact.body = strings.TrimPrefix(arg, "--body=")
	case strings.HasPrefix(arg, "--description="):
		artifact.body = strings.TrimPrefix(arg, "--description=")
	case arg == "--body-file" || arg == "-F" || arg == "--description-file":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			setRemoteArtifactBodyFile(artifact, repo, value)
		})
	case strings.HasPrefix(arg, "--body-file="):
		setRemoteArtifactBodyFile(artifact, repo, strings.TrimPrefix(arg, "--body-file="))
	case strings.HasPrefix(arg, "--description-file="):
		setRemoteArtifactBodyFile(artifact, repo, strings.TrimPrefix(arg, "--description-file="))
	case arg == "--head" || arg == "-H" || arg == "--source-branch":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.headBranch = value
		})
	case strings.HasPrefix(arg, "--head="):
		artifact.headBranch = strings.TrimPrefix(arg, "--head=")
	case strings.HasPrefix(arg, "--source-branch="):
		artifact.headBranch = strings.TrimPrefix(arg, "--source-branch=")
	case arg == "--base" || arg == "-B" || arg == "--target-branch":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.baseBranch = value
		})
	case strings.HasPrefix(arg, "--base="):
		artifact.baseBranch = strings.TrimPrefix(arg, "--base=")
	case strings.HasPrefix(arg, "--target-branch="):
		artifact.baseBranch = strings.TrimPrefix(arg, "--target-branch=")
	case arg == "--label" || arg == "-l" || arg == "--labels" || arg == "--add-label":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, value)
		})
	case arg == "--copy-issue-labels" || arg == "--with-labels":
		artifact.labels = appendRemoteArtifactLabels(artifact.labels, "copied-from-linked-issue")
	case arg == "--related-issue" || arg == "-i":
		setRemoteArtifactCreateFromIssue(artifact)
		if index+1 < len(args) {
			return index + 1
		}
	case strings.HasPrefix(arg, "--related-issue="):
		setRemoteArtifactCreateFromIssue(artifact)
	case strings.HasPrefix(arg, "--label="):
		artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--label="))
	case strings.HasPrefix(arg, "--labels="):
		artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--labels="))
	case strings.HasPrefix(arg, "--add-label="):
		artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--add-label="))
	case arg == "--assignee" || arg == "-a" || arg == "--assignees" || arg == "--add-assignee" || arg == "--assignee-id" || arg == "--assignee-ids":
		return consumeRemoteArtifactValue(args, index, func(value string) {
			artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, value)
		})
	case strings.HasPrefix(arg, "--assignee="):
		artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignee="))
	case strings.HasPrefix(arg, "--assignees="):
		artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignees="))
	case strings.HasPrefix(arg, "--add-assignee="):
		artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--add-assignee="))
	case strings.HasPrefix(arg, "--assignee-id="):
		artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignee-id="))
	case strings.HasPrefix(arg, "--assignee-ids="):
		artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignee-ids="))
	case !strings.HasPrefix(arg, "-") && strings.TrimSpace(arg) != "" && artifact.target == "":
		// 첫 비플래그 positional 인자는 대상 식별자다(예: 이슈 번호 또는 URL).
		// 봉인 보호 가드가 편집 대상을 알아야 하므로 여기서 보존한다.
		artifact.target = strings.TrimSpace(arg)
	}
	return index
}

func consumeRemoteArtifactValue(args []string, index int, set func(string)) int {
	if index+1 >= len(args) {
		return index
	}
	set(args[index+1])
	return index + 1
}

func setRemoteArtifactBodyFile(artifact *remoteArtifactCommand, repo, path string) {
	artifact.bodyFilePath = path
	artifact.body = readRemoteArtifactBodyFile(repo, artifact.bodyFilePath)
}

func setRemoteArtifactCreateFromIssue(artifact *remoteArtifactCommand) {
	if artifact.provider == "gitlab" && artifact.kind == "mr" {
		artifact.createFromIssue = true
	}
}

func appendRemoteArtifactLabels(labels []string, raw string) []string {
	return appendRemoteArtifactListValues(labels, raw)
}

func appendRemoteArtifactListValues(values []string, raw string) []string {
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
