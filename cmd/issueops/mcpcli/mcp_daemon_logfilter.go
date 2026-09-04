package mcpcli

import (
	"context"
	"io"
	"log/slog"

	daemonlog "issueops/internal/domain/daemonlog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 데몬 logFile로 가는 go-sdk 세션 루틴 이벤트(run/connect/initialized/
// disconnected)는 연결당 반복되는 volume이다. 실측 2026-08-21: 27만 줄 중
// 99.9%. 데몬 경로에서만 DEBUG로 강등한다(stdout stdio 모드는 그대로).

// daemonDiagnosticsLogger는 데몬 logFile에 쓰는 로거다.
func daemonDiagnosticsLogger(diagnostics io.Writer) *slog.Logger {
	return slog.New(daemonlog.NewFilteringHandler(slog.NewTextHandler(diagnostics, nil)))
}

// ServeMCPStreamContextWithDaemonLogger는 데몬 logFile에 세션 루틴
// 이벤트를 DEBUG로 강등하는 로거를 쓰는 스트림 서버를 실행한다.
func ServeMCPStreamContextWithDaemonLogger(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return serveMCPStreamSDKWithLogger(ctx, input, output, daemonDiagnosticsLogger(diagnostics), deps)
}

// serveMCPStreamSDKWithLogger는 지정한 로거를 쓰는 SDK 스트림 서버를
// 실행한다(transport 선택 규칙은 serveMCPStreamSDK와 동일).
func serveMCPStreamSDKWithLogger(ctx context.Context, input io.Reader, output io.Writer, logger *slog.Logger, deps MCPDependencies) error {
	server := initSDKServerWithLogger(deps, logger)
	if rwc, ok := input.(io.ReadWriteCloser); ok && io.Writer(rwc) == output {
		return server.Run(ctx, &mcp.IOTransport{Reader: rwc, Writer: rwc})
	}
	reader := io.ReadCloser(io.NopCloser(input))
	if closer, ok := input.(io.ReadCloser); ok {
		reader = closer
	}
	return server.Run(ctx, &mcp.IOTransport{Reader: reader, Writer: writeCloser{output}})
}
