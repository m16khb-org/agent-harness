package harnessapp

import (
	"io"

	"agent-harness/cmd/harness/daemoncli"
)

type daemonPaths = daemoncli.Paths
type daemonStatus = daemoncli.Status

func configureDaemonCLI() {
	daemoncli.HarnessRoot = harnessRoot
	daemoncli.ServeMCPStream = func(input io.Reader, output io.Writer, diagnostics io.Writer) error {
		return serveMCPStream(input, output, diagnostics)
	}
}

func runDaemon(args []string) error {
	configureDaemonCLI()
	return daemoncli.RunDaemon(args)
}

func runMCPProxy() error {
	configureDaemonCLI()
	return daemoncli.RunMCPProxy()
}

func currentDaemonPaths() (daemonPaths, error) {
	return daemoncli.CurrentDaemonPaths()
}

func checkDaemonStatus() daemonStatus {
	return daemoncli.CheckDaemonStatus()
}

func stopDaemon() (daemonStatus, error) {
	return daemoncli.StopDaemon()
}

func ensureDaemonRunning() (daemonStatus, error) {
	configureDaemonCLI()
	return daemoncli.EnsureDaemonRunning()
}

func daemonStatusForMCP() daemonStatus {
	return daemoncli.DaemonStatusForMCP()
}

func runDaemonServer() error {
	configureDaemonCLI()
	return daemoncli.RunDaemonServer()
}
