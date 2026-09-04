package updatecli

type (
	DaemonProcess   = daemonProcess
	MCPProxyProcess = mcpProxyProcess
)

func RunUpdate(args []string) error {
	return runUpdate(args)
}

func RunBootstrap(args []string) error {
	return runBootstrap(args)
}

func RunInstallScriptCommand(commandName string, args []string) error {
	return runInstallScriptCommand(commandName, args)
}

func RunInstallScriptExec(script string, args ...string) error {
	return runInstallScriptExec(script, args...)
}

func RefreshRunningDaemonAfterInstall() (bool, error) {
	return refreshRunningDaemonAfterInstall()
}

func TerminateStaleDaemonProcesses() (int, error) {
	return terminateStaleDaemonProcesses()
}

func ParseDaemonProcess(line, binary string) (DaemonProcess, bool) {
	return parseDaemonProcess(line, binary)
}

func ListDaemonProcesses() ([]DaemonProcess, error) {
	return listDaemonProcesses()
}

func TerminateDaemonProcess(pid int) error {
	return terminateDaemonProcess(pid)
}

func RefreshRunningMCPProxiesAfterInstall() (int, error) {
	return refreshRunningMCPProxiesAfterInstall()
}

func CleanupMCPProxies(dryRun bool) (MCPCleanupResult, error) {
	return cleanupMCPProxies(dryRun)
}

func ParseMCPProxyProcess(line, binary string) (MCPProxyProcess, bool) {
	return parseMCPProxyProcess(line, binary)
}

func ListMCPProxyProcesses() ([]MCPProxyProcess, error) {
	return listMCPProxyProcesses()
}

func TerminateMCPProxyProcess(pid int) error {
	return terminateMCPProxyProcess(pid)
}

func SetInstallScriptCommandRunner(fn func(string, ...string) error) {
	installScriptCommandRunner = fn
}

func SetPostInstallDaemonRefresh(fn func() (bool, error)) {
	postInstallDaemonRefresh = fn
}

func SetPostInstallMCPProxyRefresh(fn func() (int, error)) {
	postInstallMCPProxyRefresh = fn
}

func SetDaemonProcessLister(fn func() ([]DaemonProcess, error)) {
	daemonProcessLister = func() ([]daemonProcess, error) {
		processes, err := fn()
		return processes, err
	}
}

func SetDaemonProcessTerminator(fn func(int) error) {
	daemonProcessTerminator = fn
}

func SetMCPProxyProcessLister(fn func() ([]MCPProxyProcess, error)) {
	mcpProxyProcessLister = func() ([]mcpProxyProcess, error) {
		processes, err := fn()
		return processes, err
	}
}

func SetMCPProxyTerminator(fn func(int) error) {
	mcpProxyTerminator = fn
}
