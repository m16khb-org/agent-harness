package historycompare

import (
	statecontract "agent-harness/internal/contract/state"
)

// harness state 접근은 composition root가 설치한다. transport는 state를 어디에
// 어떻게 저장하는지 알지 않는다.
var (
	StateDelete func(key string) error
	StateDir    func() string
	StateList   func() (statecontract.StateListResult, error)
	StateRead   func(key string) (statecontract.StateResult, error)
)
