package augmentlesson

import (
	statecontract "issueops/internal/contract/state"
	"time"
)

// issueops state 접근은 composition root가 설치한다. transport는 state를 어디에
// 어떻게 저장하는지 알지 않는다.
var (
	StateDir         func() string
	StatePrunePrefix func(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error)
	StateWrite       func(key, content string) (statecontract.StateResult, error)
)
