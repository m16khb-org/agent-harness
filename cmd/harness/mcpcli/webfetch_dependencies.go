package mcpcli

import (
	webfetchcontract "agent-harness/internal/contract/webfetch"
	"context"
)

// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	Fetch func(ctx context.Context, request webfetchcontract.Request) (webfetchcontract.Result, error)
)
