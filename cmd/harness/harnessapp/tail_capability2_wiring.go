package harnessapp

import (
	basicclit2deps "agent-harness/cmd/harness/basiccli"
	mcpclit2deps "agent-harness/cmd/harness/mcpcli"
	projectclit2deps "agent-harness/cmd/harness/projectcli"
	statusclit2deps "agent-harness/cmd/harness/statuscli"
	commitsuggestadapter "agent-harness/internal/adapter/commitsuggest"
	guardadapter "agent-harness/internal/adapter/guard"
	lintdiagnoseadapter "agent-harness/internal/adapter/lintdiagnose"
	traceadapter "agent-harness/internal/adapter/trace"
)

// configureTailCapabilities2는 commit 제안, lint 진단, guard 검사, trace 분석을
// 설치한다. 모두 저장소를 읽거나 외부 명령을 부른다.
func configureTailCapabilities2() {
	basicclit2deps.GuardCheck = guardadapter.GuardCheck
	basicclit2deps.TraceAnalyze = traceadapter.TraceAnalyze
	mcpclit2deps.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	mcpclit2deps.SuggestCommit = commitsuggestadapter.SuggestCommit
	projectclit2deps.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	projectclit2deps.SuggestCommit = commitsuggestadapter.SuggestCommit
	statusclit2deps.GuardCheck = guardadapter.GuardCheck
}
