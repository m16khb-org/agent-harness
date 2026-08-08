package basiccli

import (
	tracecontract "agent-harness/internal/contract/trace"
)

// 이 연산은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	TraceAnalyze func(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error)
)
