package basiccli

func RunDocs(args []string) error {
	return runDocs(args)
}

func RunDocsWithRoot(args []string, root string) error {
	return runDocsWithRoot(args, root)
}

func RunPreflight(args []string) error {
	return runPreflight(args)
}

func RunTrace(args []string) error {
	return runTrace(args)
}

func RunTraceAnalyze(args []string) error {
	return runTraceAnalyze(args)
}

func RunGuard(args []string) error {
	return runGuard(args)
}

func RunGuardCheck(args []string) error {
	return runGuardCheck(args)
}

func RunInspect(args []string) error {
	return runInspect(args)
}

func RunDoctor(args []string) error {
	return runDoctor(args)
}
