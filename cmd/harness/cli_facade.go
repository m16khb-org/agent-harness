package main

import (
	"os"

	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/draftwikicli"
	"agent-harness/cmd/harness/installcli"
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/cmd/harness/policycli"
	"agent-harness/cmd/harness/projectcli"
	"agent-harness/cmd/harness/riskqa"
	"agent-harness/cmd/harness/statecli"
	"agent-harness/cmd/harness/statuscli"
	"agent-harness/cmd/harness/workercli"
	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

const shellPathRCMarker = installcli.ShellPathRCMarker

type issueOpsWorktreeToolPrepareResult = issueopscli.WorktreeToolPrepareResult
type RiskQATierPlan = riskqa.RiskQATierPlan

type riskQATierDeps struct {
	plan func(string) RiskQATierPlan
	run  func(root string, command string) StepResult
}

type (
	HarnessStatus              = statuscli.Status
	SelfVerifyStatus           = statuscli.SelfVerificationStatus
	VerifyWorkResult           = statuscli.WorkResult
	VerifyWorkEvidenceItem     = statuscli.WorkEvidenceItem
	VerifyWorkSuggestedCommand = statuscli.WorkSuggestedCommand
)

func init() {
	basiccli.HarnessRoot = harnessRoot
	basiccli.ResolveTarget = resolveTarget
	basiccli.Version = version
	basiccli.InspectHarness = inspectHarness
	installcli.HarnessRoot = harnessRoot
	policycli.ResolveTarget = resolveTarget
	statuscli.HarnessRoot = harnessRoot
	statuscli.ResolveTarget = resolveTarget
	statuscli.Version = version
	statuscli.InspectHarness = inspectHarness
	statuscli.CheckDaemonStatus = checkDaemonStatus
	workercli.ResolveTarget = resolveTarget
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

func runIssueOps(args []string) error {
	return issueopscli.RunIssueOps(args)
}

func prepareIssueOpsWorktreeTools(record core.IssueOpsRecord) (issueOpsWorktreeToolPrepareResult, error) {
	return issueopscli.PrepareWorktreeTools(record)
}

func verifyIssueOpsChildIssueBeforeLink(childURL string) error {
	return issueopscli.VerifyChildIssueBeforeLink(childURL)
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return issueopscli.CleanupMerged(id, requested)
}

func verifyIssueOpsRemoteArtifactLive(req core.IssueOpsRemoteArtifactVerificationRequest) error {
	return issueopscli.VerifyRemoteArtifactLive(req)
}

func runPolicy(args []string) error {
	return policycli.Run(args)
}

func runPolicyCheck(args []string) error {
	return policycli.RunCheck(args)
}

func runPolicyFakeRun(args []string) error {
	return policycli.RunFakeRun(args)
}

func runPolicyRun(args []string) error {
	return policycli.RunReadOnly(args)
}

func runPolicyAudit(args []string) error {
	return policycli.RunAudit(args)
}

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	return policycli.ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (core.CommandPolicyRequest, bool, bool, error) {
	return policycli.ParseRunFlags(args)
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

func validateRiskQATier(root string) StepResult {
	return riskqa.Validate(root)
}

func validateRiskQATierWithDeps(root string, deps riskQATierDeps) StepResult {
	return riskqa.ValidateWithDeps(root, riskqa.Deps{Plan: deps.plan, Run: deps.run})
}

func planRiskQATier(root string) RiskQATierPlan {
	return riskqa.Plan(root)
}

func planRiskQATierFromPaths(paths []string) RiskQATierPlan {
	return riskqa.PlanFromPaths(paths)
}

func parseGitStatusPath(line string) string {
	return riskqa.ParseGitStatusPath(line)
}

func riskQATierPlanJSON(plan RiskQATierPlan) string {
	return riskqa.PlanJSON(plan)
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
