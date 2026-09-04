package mcpcli

import (
	statecontract "issueops/internal/contract/state"
	"time"
)

// issueops state 접근은 composition root가 설치한다. transport는 state를 어디에
// 어떻게 저장하는지 알지 않는다.
var (
	StateDoctor   func() (statecontract.StateDoctorResult, error)
	StateList     func() (statecontract.StateListResult, error)
	StateMaintain func() (statecontract.StateMaintainResult, error)
	StatePrune    func(maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error)
	StateRead     func(key string) (statecontract.StateResult, error)
	StateWrite    func(key, content string) (statecontract.StateResult, error)
)
