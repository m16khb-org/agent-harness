package main

import "agent-harness/cmd/harness/updatecli"

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
