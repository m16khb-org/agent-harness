package statuscli

import (
	basicclit2d "issueops/cmd/issueops/basiccli"
	guardadapter "issueops/internal/adapter/guard"
	traceadapter "issueops/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	GuardCheck = guardadapter.GuardCheck
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = traceadapter.TraceAnalyze
}
