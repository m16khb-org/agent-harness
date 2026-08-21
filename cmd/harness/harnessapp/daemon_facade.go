package harnessapp

import (
	"context"
	"io"

	"agent-harness/cmd/harness/daemoncli"
	"agent-harness/cmd/harness/mcpcli"
)

type daemonStatus = daemoncli.Status

func configureDaemonCLI() {
	daemoncli.HarnessRoot = harnessRoot
	daemoncli.ServeMCPStream = func(input io.Reader, output io.Writer, diagnostics io.Writer) error {
		return serveMCPStream(input, output, diagnostics)
	}
	daemoncli.ServeMCPStreamContext = func(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
		// 데몬 logFile로 가는 go-sdk 세션 루틴 이벤트(run/connect/
		// initialized)는 연결당 반복되는 volume이다. 실측 2026-08-21:
		// 27만 줄 중 99.9%. 데몬 경로에서만 DEBUG로 강등한다(stdout stdio
		// 모드의 diagnostics는 그대로 둔다).
		return mcpcli.ServeMCPStreamContextWithDaemonLogger(ctx, input, output, diagnostics, issueOpsMCPDependencies())
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
