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

func wireHostCLIDeps() {
	updatecli.Configure(updatecli.Deps{HarnessRoot: harnessRoot})
	resetUpdateFacadeDeps()
}

func configureHookCLI() {
	hookcli.ResolveTarget = resolveTarget
}

func runHook(args []string) error {
	configureHookCLI()
	return hookcli.RunHook(args)
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
