package statuscli

import (
	guardcontract "issueops/internal/contract/guard"
)

// 이 연산은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	GuardCheck func(req guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult
)
