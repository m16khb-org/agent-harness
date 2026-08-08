package hookcatalog

import (
	installcontract "agent-harness/internal/contract/install"
)

// native runtime 진단과 skill 목록 조회는 파일시스템을 읽는다. 그 구현은
// composition root가 설치한다.
var (
	NativeRuntimeDiagnosticMessage func(diagnostic installcontract.NativeRuntimeDiagnostic, err error) (string, bool)
)
