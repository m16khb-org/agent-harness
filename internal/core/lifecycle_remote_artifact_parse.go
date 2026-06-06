package core

import "strings"

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
}

func parseGHRemoteArtifactCommand(command string, repo string) (remoteArtifactCommand, bool) {
	tokens := splitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		cli := searchTokenName(tokens[i])
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

func remoteArtifactHelpRequested(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--help", "-h":
			return true
		}
	}
	return false
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
