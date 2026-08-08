package issueopscli

import (
	"agent-harness/internal/port"
)

// 이 연산들은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	Resolve func(name string) (port.IssueProvider, error)
)
