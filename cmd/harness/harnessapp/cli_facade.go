package harnessapp

import (
	"os"

	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/draftwikicli"
	"agent-harness/cmd/harness/installcli"
	"agent-harness/cmd/harness/projectcli"
	"agent-harness/cmd/harness/qualitycli"
	"agent-harness/cmd/harness/statecli"
	"agent-harness/cmd/harness/statuscli"
	"agent-harness/cmd/harness/webfetchcli"
	"agent-harness/cmd/harness/workercli"
	"agent-harness/internal/port"
)

const shellPathRCMarker = installcli.ShellPathRCMarker

type (
	HarnessStatus              = statuscli.Status
	SelfVerifyStatus           = statuscli.SelfVerificationStatus
	VerifyWorkResult           = statuscli.WorkResult
	VerifyWorkEvidenceItem     = statuscli.WorkEvidenceItem
	VerifyWorkSuggestedCommand = statuscli.WorkSuggestedCommand
)

func wireBasicCLIDeps() {
	basiccli.Configure(basiccli.Deps{
		HarnessRoot:    harnessRoot,
		ResolveTarget:  resolveTarget,
		Version:        version,
		InspectHarness: inspectHarness,
	})
	installcli.Configure(installcli.Deps{HarnessRoot: harnessRoot})
	qualitycli.Configure(qualitycli.Deps{
		HarnessRoot: harnessRoot,
		Version:     version,
		PrintJSON:   printJSON,
	})
	statuscli.Configure(statuscli.Deps{
		HarnessRoot:       harnessRoot,
		ResolveTarget:     resolveTarget,
		Version:           version,
		InspectHarness:    inspectHarness,
		CheckDaemonStatus: checkDaemonStatus,
	})
	workercli.Configure(workercli.Deps{ResolveTarget: resolveTarget})
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

func runQuality(args []string) error {
	return qualitycli.Run(args)
}

func runQualityInspectWithDeps(args []string, deps qualitycli.InspectDeps) error {
	return qualitycli.RunInspectWithDeps(args, deps)
}

func runInspect(args []string) error {
	return basiccli.RunInspect(args)
}

func runDoctor(args []string) error {
	return basiccli.RunDoctor(args)
}

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

func runInstall(args []string) error {
	return installcli.RunInstall(args)
}

func runInstallNative(args []string) error {
	return installcli.RunInstallNative(args)
}

func runInstallCommand(commandName string, args []string) error {
	return installcli.RunInstallCommand(commandName, args)
}

func validateInteractiveInstallInput(stdin *os.File) error {
	return installcli.ValidateInteractiveInput(stdin)
}

func printInstallNativeResult(result port.NativeInstallResult) {
	installcli.PrintNativeResult(result)
}

func preferredShellRC(home string) string {
	return installcli.PreferredShellRC(home)
}

func appendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	return installcli.AppendShellPathLinePlan(path, dryRun)
}

func shellRCAlreadyAddsLocalBin(path, home string) bool {
	return installcli.ShellRCAlreadyAddsLocalBin(path, home)
}

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

func runState(args []string) error {
	return statecli.Run(args)
}

func runStateWrite(args []string) error {
	return statecli.RunWrite(args)
}

func runStateRead(args []string) error {
	return statecli.RunRead(args)
}

func runStateList(args []string) error {
	return statecli.RunList(args)
}

func runStatePrune(args []string) error {
	return statecli.RunPrune(args)
}

func runStateDoctor(args []string) error {
	return statecli.RunDoctor(args)
}

func runStateMigrate(args []string) error {
	return statecli.RunMigrate(args)
}

func runStatus(args []string) error {
	return statuscli.RunStatus(args)
}

func buildHarnessStatus(repo string) HarnessStatus {
	return statuscli.BuildStatus(repo)
}

func runVerifyWork(args []string) error {
	return statuscli.RunVerifyWork(args)
}

func buildVerifyWork(repo string, all bool, argv []string) VerifyWorkResult {
	return statuscli.BuildVerifyWork(repo, all, argv)
}

func runWorker(args []string) error {
	return workercli.Run(args)
}

func runWebFetch(args []string) error {
	return webfetchcli.Run(args)
}

func runWorkerEnqueue(args []string) error {
	return workercli.RunEnqueue(args)
}

func runWorkerDraftWiki(args []string) error {
	return workercli.RunDraftWiki(args)
}

func runWorkerRun(args []string) error {
	return workercli.RunReadOnly(args)
}

func runWorkerStatus(args []string) error {
	return workercli.RunStatus(args)
}

func runWorkerList(args []string) error {
	return workercli.RunList(args)
}

func runWorkerCancel(args []string) error {
	return workercli.RunCancel(args)
}
