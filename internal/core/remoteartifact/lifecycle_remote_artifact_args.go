package remoteartifact

import "strings"

func parseRemoteArtifactArgs(artifact *remoteArtifactCommand, repo string, args []string) {
	for j := 0; j < len(args); j++ {
		arg := args[j]
		switch {
		case arg == "--title" || arg == "-t":
			if j+1 < len(args) {
				artifact.title = args[j+1]
				j++
			}
		case strings.HasPrefix(arg, "--title="):
			artifact.title = strings.TrimPrefix(arg, "--title=")
		case arg == "--body" || arg == "-b" || arg == "--description" || arg == "-d":
			if j+1 < len(args) {
				artifact.body = args[j+1]
				j++
			}
		case strings.HasPrefix(arg, "--body="):
			artifact.body = strings.TrimPrefix(arg, "--body=")
		case strings.HasPrefix(arg, "--description="):
			artifact.body = strings.TrimPrefix(arg, "--description=")
		case arg == "--body-file" || arg == "-F" || arg == "--description-file":
			if j+1 < len(args) {
				artifact.bodyFilePath = args[j+1]
				artifact.body = readRemoteArtifactBodyFile(repo, artifact.bodyFilePath)
				j++
			}
		case strings.HasPrefix(arg, "--body-file="):
			artifact.bodyFilePath = strings.TrimPrefix(arg, "--body-file=")
			artifact.body = readRemoteArtifactBodyFile(repo, artifact.bodyFilePath)
		case strings.HasPrefix(arg, "--description-file="):
			artifact.bodyFilePath = strings.TrimPrefix(arg, "--description-file=")
			artifact.body = readRemoteArtifactBodyFile(repo, artifact.bodyFilePath)
		case arg == "--head" || arg == "-H" || arg == "--source-branch":
			if j+1 < len(args) {
				artifact.headBranch = args[j+1]
				j++
			}
		case strings.HasPrefix(arg, "--head="):
			artifact.headBranch = strings.TrimPrefix(arg, "--head=")
		case strings.HasPrefix(arg, "--source-branch="):
			artifact.headBranch = strings.TrimPrefix(arg, "--source-branch=")
		case arg == "--base" || arg == "-B" || arg == "--target-branch":
			if j+1 < len(args) {
				artifact.baseBranch = args[j+1]
				j++
			}
		case strings.HasPrefix(arg, "--base="):
			artifact.baseBranch = strings.TrimPrefix(arg, "--base=")
		case strings.HasPrefix(arg, "--target-branch="):
			artifact.baseBranch = strings.TrimPrefix(arg, "--target-branch=")
		case arg == "--label" || arg == "-l" || arg == "--labels" || arg == "--add-label":
			if j+1 < len(args) {
				artifact.labels = appendRemoteArtifactLabels(artifact.labels, args[j+1])
				j++
			}
		case arg == "--copy-issue-labels":
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, "copied-from-linked-issue")
		case arg == "--with-labels":
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, "copied-from-linked-issue")
		case arg == "--related-issue" || arg == "-i":
			if artifact.provider == "gitlab" && artifact.kind == "mr" {
				artifact.createFromIssue = true
			}
			if j+1 < len(args) {
				j++
			}
		case strings.HasPrefix(arg, "--related-issue="):
			if artifact.provider == "gitlab" && artifact.kind == "mr" {
				artifact.createFromIssue = true
			}
		case strings.HasPrefix(arg, "--label="):
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--label="))
		case strings.HasPrefix(arg, "--labels="):
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--labels="))
		case strings.HasPrefix(arg, "--add-label="):
			artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--add-label="))
		case arg == "--assignee" || arg == "-a" || arg == "--assignees" || arg == "--add-assignee" || arg == "--assignee-id" || arg == "--assignee-ids":
			if j+1 < len(args) {
				artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, args[j+1])
				j++
			}
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
		}
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
