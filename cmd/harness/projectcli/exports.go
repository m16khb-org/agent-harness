package projectcli

func Run(args []string) error {
	return runProject(args)
}

func RunBootstrap(args []string) error {
	return runProjectBootstrap(args)
}

func RunDocs(args []string) error {
	return runProjectDocs(args)
}

func RunRouteDocs(args []string) error {
	return runProjectRouteDocs(args)
}

func RunRecord(args []string) error {
	return runProjectAppend(args)
}

func RunCommitSuggest(args []string) error {
	return runProjectCommitSuggest(args)
}

func RunLintDiagnose(args []string) error {
	return runProjectLintDiagnose(args)
}
