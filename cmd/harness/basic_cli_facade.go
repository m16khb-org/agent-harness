package main

import "agent-harness/cmd/harness/basiccli"

func init() {
	basiccli.HarnessRoot = harnessRoot
	basiccli.ResolveTarget = resolveTarget
	basiccli.Version = version
	basiccli.InspectHarness = inspectHarness
}

func runDocs(args []string) error {
	return basiccli.RunDocs(args)
}

func runDocsWithRoot(args []string, root string) error {
	return basiccli.RunDocsWithRoot(args, root)
}

func runPreflight(args []string) error {
	return basiccli.RunPreflight(args)
}

func runTrace(args []string) error {
	return basiccli.RunTrace(args)
}

func runTraceAnalyze(args []string) error {
	return basiccli.RunTraceAnalyze(args)
}

func runGuard(args []string) error {
	return basiccli.RunGuard(args)
}

func runGuardCheck(args []string) error {
	return basiccli.RunGuardCheck(args)
}

func runInspect(args []string) error {
	return basiccli.RunInspect(args)
}

func runDoctor(args []string) error {
	return basiccli.RunDoctor(args)
}
