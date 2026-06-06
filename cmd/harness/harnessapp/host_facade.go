package harnessapp

import (
	"agent-harness/cmd/harness/hookcli"
	"agent-harness/cmd/harness/updatecli"
)

type daemonProcess = updatecli.DaemonProcess
type mcpProxyProcess = updatecli.MCPProxyProcess

var installScriptCommandRunner = updatecli.RunInstallScriptExec
var postInstallDaemonRefresh = updatecli.RefreshRunningDaemonAfterInstall
var postInstallMCPProxyRefresh = updatecli.RefreshRunningMCPProxiesAfterInstall
var daemonProcessLister = updatecli.ListDaemonProcesses
var daemonProcessTerminator = updatecli.TerminateDaemonProcess
var mcpProxyProcessLister = updatecli.ListMCPProxyProcesses
var mcpProxyTerminator = updatecli.TerminateMCPProxyProcess

func init() {
	updatecli.HarnessRoot = harnessRoot
	resetUpdateFacadeDeps()
}

func configureHookCLI() {
	hookcli.ResolveTarget = resolveTarget
}

func runHook(args []string) error {
	configureHookCLI()
	return hookcli.RunHook(args)
}

func runHookUserPrompt(args []string) error {
	configureHookCLI()
	return hookcli.RunHookUserPrompt(args)
}

func runHookPreToolUse(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPreToolUse(args)
}

func runHookPostToolUse(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPostToolUse(args)
}

func runHookPreCompact(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPreCompact(args)
}

func runHookPostCompact(args []string) error {
	configureHookCLI()
	return hookcli.RunHookPostCompact(args)
}

func runHookSessionStart(args []string) error {
	configureHookCLI()
	return hookcli.RunHookSessionStart(args)
}

func runHookStop(args []string) error {
	configureHookCLI()
	return hookcli.RunHookStop(args)
}

func runHookFailures(args []string) error {
	configureHookCLI()
	return hookcli.RunHookFailures(args)
}

func hookArgValue(args []string, flagName string) string {
	return hookcli.HookArgValue(args, flagName)
}

func repoFromHookInput(input []byte) string {
	return hookcli.RepoFromHookInput(input)
}

func sourceFromHookInput(input []byte) string {
	return hookcli.SourceFromHookInput(input)
}

func pathsFromHookInput(input []byte) []string {
	return hookcli.PathsFromHookInput(input)
}

func promptFromHookInput(input []byte) string {
	return hookcli.PromptFromHookInput(input)
}

func isStopHookContinuationPrompt(prompt string) bool {
	return hookcli.IsStopHookContinuationPrompt(prompt)
}

func lastAssistantMessageFromHookInput(input []byte) string {
	return hookcli.LastAssistantMessageFromHookInput(input)
}

func transcriptPathFromHookInput(input []byte) string {
	return hookcli.TranscriptPathFromHookInput(input)
}

func readLastAssistantMessageFromTranscript(path string) string {
	return hookcli.ReadLastAssistantMessageFromTranscript(path)
}

func toolNameFromHookInput(input []byte) string {
	return hookcli.ToolNameFromHookInput(input)
}

func commandFromHookInput(input []byte) string {
	return hookcli.CommandFromHookInput(input)
}

func projectPathFromHookInput(input []byte) string {
	return hookcli.ProjectPathFromHookInput(input)
}

func envBool(name string) bool {
	return hookcli.EnvBool(name)
}

func envFloat(name string) float64 {
	return hookcli.EnvFloat(name)
}

func resetUpdateFacadeDeps() {
	if daemonProcessLister == nil {
		daemonProcessLister = func() ([]daemonProcess, error) { return nil, nil }
	}
	if mcpProxyProcessLister == nil {
		mcpProxyProcessLister = func() ([]mcpProxyProcess, error) { return nil, nil }
	}
	updatecli.SetInstallScriptCommandRunner(installScriptCommandRunner)
	updatecli.SetPostInstallDaemonRefresh(postInstallDaemonRefresh)
	updatecli.SetPostInstallMCPProxyRefresh(postInstallMCPProxyRefresh)
	updatecli.SetDaemonProcessLister(daemonProcessLister)
	updatecli.SetDaemonProcessTerminator(daemonProcessTerminator)
	updatecli.SetMCPProxyProcessLister(mcpProxyProcessLister)
	updatecli.SetMCPProxyTerminator(mcpProxyTerminator)
}

func runUpdate(args []string) error {
	resetUpdateFacadeDeps()
	return updatecli.RunUpdate(args)
}

func runBootstrap(args []string) error {
	resetUpdateFacadeDeps()
	return updatecli.RunBootstrap(args)
}

func runInstallScriptCommand(commandName string, args []string) error {
	resetUpdateFacadeDeps()
	return updatecli.RunInstallScriptCommand(commandName, args)
}

func runInstallScriptExec(script string, args ...string) error {
	return updatecli.RunInstallScriptExec(script, args...)
}

func hasInstallFlag(args []string, name string) bool {
	return updatecli.HasInstallFlag(args, name)
}

func refreshRunningDaemonAfterInstall() (bool, error) {
	resetUpdateFacadeDeps()
	return updatecli.RefreshRunningDaemonAfterInstall()
}

func terminateStaleDaemonProcesses() (int, error) {
	resetUpdateFacadeDeps()
	return updatecli.TerminateStaleDaemonProcesses()
}

func parseDaemonProcess(line, binary string) (daemonProcess, bool) {
	return updatecli.ParseDaemonProcess(line, binary)
}

func listDaemonProcesses() ([]daemonProcess, error) {
	return updatecli.ListDaemonProcesses()
}

func terminateDaemonProcess(pid int) error {
	return updatecli.TerminateDaemonProcess(pid)
}

func refreshRunningMCPProxiesAfterInstall() (int, error) {
	resetUpdateFacadeDeps()
	return updatecli.RefreshRunningMCPProxiesAfterInstall()
}

func parseMCPProxyProcess(line, binary string) (mcpProxyProcess, bool) {
	return updatecli.ParseMCPProxyProcess(line, binary)
}

func listMCPProxyProcesses() ([]mcpProxyProcess, error) {
	return updatecli.ListMCPProxyProcesses()
}

func terminateMCPProxyProcess(pid int) error {
	return updatecli.TerminateMCPProxyProcess(pid)
}
