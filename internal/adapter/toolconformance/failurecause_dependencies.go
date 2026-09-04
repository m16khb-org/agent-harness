package toolconformance

import (
	failurecausecontract "issueops/internal/contract/failurecause"
)

// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	ClassifyFailureCause func(failed bool, items []failurecausecontract.Evidence) failurecausecontract.Result
)
