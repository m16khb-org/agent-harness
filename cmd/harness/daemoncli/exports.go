package daemoncli

type Paths = daemonPaths
type Status = daemonStatus

func RunDaemon(args []string) error {
	return runDaemon(args)
}

func RunMCPProxy() error {
	return runMCPProxy()
}

func CurrentDaemonPaths() (Paths, error) {
	return currentDaemonPaths()
}

func CheckDaemonStatus() Status {
	return checkDaemonStatus()
}

func StopDaemon() (Status, error) {
	return stopDaemon()
}

func EnsureDaemonRunning() (Status, error) {
	return ensureDaemonRunning()
}

func DaemonStatusForMCP() Status {
	return daemonStatusForMCP()
}

func RunDaemonServer() error {
	return runDaemonServer()
}
