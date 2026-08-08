package harnessapp

import (
	"agent-harness/internal/adapter/docs"
	statestore "agent-harness/internal/adapter/outbound/state"
	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/adapter/projectdocs"
	"os"

	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/installcli"
	"agent-harness/cmd/harness/loopcli"
	"agent-harness/cmd/harness/projectcli"
	"agent-harness/cmd/harness/qualitycli"
	"agent-harness/cmd/harness/statecli"
	"agent-harness/cmd/harness/statuscli"
	"agent-harness/cmd/harness/webfetchcli"
	"agent-harness/cmd/harness/workercli"
	"agent-harness/internal/adapter/operationalhealth"
	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/port"
)

type (
	HarnessStatus              = statuscli.Status
	SelfVerifyStatus           = statuscli.SelfVerificationStatus
	VerifyWorkResult           = statuscli.WorkResult
	VerifyWorkEvidenceItem     = statuscli.WorkEvidenceItem
	VerifyWorkSuggestedCommand = statuscli.WorkSuggestedCommand
)

func wireBasicCLIDeps() {
	configureDocsReaders()
	configureStateStores()
	configureIssueOpsRuntime()
	configureTail8()
	configureDoctorLoopGate()
	configureRemoteArtifactRules()
	configureHookPrompts()
	configureInstallPlans()
	configureStateDatabases()
	configureTail6()
	configureTail5()
	configureAdapterTail()
	configureTailCapabilities2()
	configureTailCapabilities()
	configureIssueOpsReaders()
	configureInstallReaders()
	configureProjectDocReaders()
	configurePolicyAndGitObservers()
	configureAdapterStateAccess()
	configureWorkerJobs()
	configureRepoPathResolvers()
	configureProjectBootstrap()
	configureHookCLILifecycle()
	configureDoctorHookPromptLifecycle()
	configureToolConformance()
	configureDoctorRunner()
	configureIssueOpsBenchmark()
	configureIssueOpsCleanup()
	configureIssueOpsRemote()
	configureIssueOpsOrphanAndLoopGate()
	configureLifecycleIssueOps()
	configureIssueOpsLeaseNextCommands()
	configureIssueOpsExecutionRunners()
	configureIssueOpsCLIRuntime()
	operationalCollector := operationalhealth.Collector{Git: operationalhealth.ExecGitRunner{}, Orca: orca.New()}
	basiccli.Configure(basiccli.Deps{
		GitPreflight:             preflight.GitPreflight,
		HarnessRoot:              harnessRoot,
		ResolveTarget:            resolveTarget,
		Version:                  version,
		InspectHarness:           inspectHarness,
		CheckDaemonStatus:        checkDaemonStatus,
		CollectOperationalHealth: operationalCollector.Collect,
		DocsIndex:                docs.DocsIndex,
	})
	installcli.Configure(installDependencies())
	qualitycli.Configure(qualitycli.Deps{
		HarnessRoot: harnessRoot,
		Version:     version,
		PrintJSON:   printJSON,
		StateRead:   statestore.StateRead,
		StateWrite:  statestore.StateWrite,
	})
	statuscli.Configure(statuscli.Deps{
		AnalyzeProjectSignals: projectdocs.AnalyzeProjectSignals,
		HarnessRoot:           harnessRoot,
		ResolveTarget:         resolveTarget,
		Version:               version,
		InspectHarness:        inspectHarness,
		CheckDaemonStatus:     checkDaemonStatus,
		GitPreflight:          preflight.GitPreflight,
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

func runInstall(args []string) error {
	return installcli.RunInstall(args)
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
	return statecli.Run(stateDependencies(), args)
}

func runStateWrite(args []string) error {
	return statecli.RunWrite(stateDependencies(), args)
}

func runStateRead(args []string) error {
	return statecli.RunRead(stateDependencies(), args)
}

func runStateList(args []string) error {
	return statecli.RunList(stateDependencies(), args)
}

func runStatePrune(args []string) error {
	return statecli.RunPrune(stateDependencies(), args)
}

func runStateDoctor(args []string) error {
	return statecli.RunDoctor(stateDependencies(), args)
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

func runLoop(args []string) error {
	return loopcli.Run(loopDependencies(), args)
}

func runWebFetch(args []string) error {
	return webfetchcli.Run(args)
}

func runWorkerEnqueue(args []string) error {
	return workercli.RunEnqueue(args)
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
