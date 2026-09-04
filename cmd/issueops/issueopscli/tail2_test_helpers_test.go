package issueopscli

import (
	mcpclit2d "issueops/cmd/issueops/mcpcli"
	commitsuggestadapter "issueops/internal/adapter/commitsuggest"
	lintdiagnoseadapter "issueops/internal/adapter/lintdiagnose"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	mcpclit2d.DiagnoseCommand = lintdiagnoseadapter.DiagnoseCommand
	mcpclit2d.SuggestCommit = commitsuggestadapter.SuggestCommit
}
