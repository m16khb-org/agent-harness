package mcpcli

import (
	basicclit2d "agent-harness/cmd/harness/basiccli"
	commitsuggestadapter "agent-harness/internal/adapter/commitsuggest"
	guardadapter "agent-harness/internal/adapter/guard"
	lintdiagnoseadapter "agent-harness/internal/adapter/lintdiagnose"
	traceadapter "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	SuggestCommit = commitsuggestadapter.SuggestCommit
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = traceadapter.TraceAnalyze
}
