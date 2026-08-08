package statuscli

import (
	basicclit2d "agent-harness/cmd/harness/basiccli"
	guardadapter "agent-harness/internal/adapter/guard"
	traceadapter "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	GuardCheck = guardadapter.GuardCheck
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = traceadapter.TraceAnalyze
}
