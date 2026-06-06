package core

import "agent-harness/internal/core/lintdiagnose"

type LintDiagnoseRequest = lintdiagnose.LintDiagnoseRequest
type LintDiagnoseResult = lintdiagnose.LintDiagnoseResult

func DiagnoseCommand(req LintDiagnoseRequest) (LintDiagnoseResult, error) {
	return lintdiagnose.DiagnoseCommand(req)
}

func buildLintDiagnosePrompt(exitCode int, logTail string) string {
	return lintdiagnose.BuildPrompt(exitCode, logTail)
}
