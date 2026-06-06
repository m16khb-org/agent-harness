package main

import "agent-harness/cmd/harness/draftwikicli"

func runProjectDraftWiki(args []string) error {
	return draftwikicli.RunProjectDraftWiki(args)
}

func draftWikiQueueMaterial(repo, input, material string, stdinFlag bool) (string, error) {
	return draftwikicli.DraftWikiQueueMaterial(repo, input, material, stdinFlag)
}

func runProjectDraftWikiSuggest(args []string) error {
	return draftwikicli.RunProjectDraftWikiSuggest(args)
}

func parseDraftWikiPathFlags(name string, args []string) (path, repo string, jsonOut bool, err error) {
	return draftwikicli.ParseDraftWikiPathFlags(name, args)
}
