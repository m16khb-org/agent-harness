package basiccli

import (
	guardadapter "agent-harness/internal/adapter/guard"
	traceadapter "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	GuardCheck = guardadapter.GuardCheck
	TraceAnalyze = traceadapter.TraceAnalyze
}
