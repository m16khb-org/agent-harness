package contractcli

import (
	"context"
	"io"

	mcpcontract "issueops/internal/contract/mcp"
)

// conformance probe는 프로세스 간 stream을 다룬다. 구현은 composition root가 설치한다.
var ServeConformanceProbe func(ctx context.Context, input io.Reader, output io.Writer, config mcpcontract.ConformanceProbeConfig) error
