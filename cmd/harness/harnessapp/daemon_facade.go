package harnessapp

import (
	"context"
	"io"

	"agent-harness/cmd/harness/daemoncli"
)

type daemonStatus = daemoncli.Status

func configureDaemonCLI() {
	daemoncli.HarnessRoot = harnessRoot
	daemoncli.ServeMCPStream = func(input io.Reader, output io.Writer, diagnostics io.Writer) error {
		return serveMCPStream(input, output, diagnostics)
	}
	daemoncli.ServeMCPStreamContext = func(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
		return serveMCPStreamContext(ctx, input, output, diagnostics)
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

func checkDaemonStatus() daemonStatus {
	return daemoncli.CheckDaemonStatus()
}

func daemonStatusForMCP() daemonStatus {
	return daemoncli.DaemonStatusForMCP()
}
