package main

import "agent-harness/cmd/harness/projectcli"

func runProject(args []string) error {
	return projectcli.Run(args)
}

func runProjectBootstrap(args []string) error {
	return projectcli.RunBootstrap(args)
}

func runProjectDocs(args []string) error {
	return projectcli.RunDocs(args)
}

func runProjectRouteDocs(args []string) error {
	return projectcli.RunRouteDocs(args)
}

func runProjectRecord(args []string) error {
	return projectcli.RunRecord(args)
}

func runProjectCommitSuggest(args []string) error {
	return projectcli.RunCommitSuggest(args)
}

func runProjectLintDiagnose(args []string) error {
	return projectcli.RunLintDiagnose(args)
}
