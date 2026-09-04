package projectcli

import (
	basicclit2d "issueops/cmd/issueops/basiccli"
	commitsuggestadapter "issueops/internal/adapter/commitsuggest"
	guardadapter "issueops/internal/adapter/guard"
	lintdiagnoseadapter "issueops/internal/adapter/lintdiagnose"
	traceadapter "issueops/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	SuggestCommit = commitsuggestadapter.SuggestCommit
	basicclit2d.GuardCheck = guardadapter.GuardCheck
	basicclit2d.TraceAnalyze = traceadapter.TraceAnalyze
}
