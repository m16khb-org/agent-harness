package projectcli

import (
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
)

// 이 연산은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	DiagnoseCommand func(req lintdiagnosecontract.LintDiagnoseRequest) (lintdiagnosecontract.LintDiagnoseResult, error)
)
