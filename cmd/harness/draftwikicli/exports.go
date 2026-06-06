package draftwikicli

func RunProjectDraftWiki(args []string) error {
	return runProjectDraftWiki(args)
}

func DraftWikiQueueMaterial(repo, input, material string, stdinFlag bool) (string, error) {
	return draftWikiQueueMaterial(repo, input, material, stdinFlag)
}

func RunProjectDraftWikiSuggest(args []string) error {
	return runProjectDraftWikiSuggest(args)
}

func ParseDraftWikiPathFlags(name string, args []string) (path, repo string, jsonOut bool, err error) {
	return parseDraftWikiPathFlags(name, args)
}
