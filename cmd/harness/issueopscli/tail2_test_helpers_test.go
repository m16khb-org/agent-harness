package issueopscli

import (
	mcpclit2d "agent-harness/cmd/harness/mcpcli"
	commitsuggestadapter "agent-harness/internal/adapter/commitsuggest"
	lintdiagnoseadapter "agent-harness/internal/adapter/lintdiagnose"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	mcpclit2d.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	mcpclit2d.SuggestCommit = commitsuggestadapter.SuggestCommit
}
