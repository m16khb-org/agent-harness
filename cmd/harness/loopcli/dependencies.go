package loopcli

import (
	loopruncontract "agent-harness/internal/contract/looprun"
)

// Dependencies는 loop CLI가 필요로 하는 loop run 연산을 함수로 받는다.
//
// loop 상태는 harness state에 저장된다. CLI는 flag 해석과 출력만 소유하고,
// 저장소 조립은 composition root가 한다.
type Dependencies struct {
	Start         func(loopruncontract.StartLoopRequest) (loopruncontract.LoopRun, error)
	RecordAttempt func(loopID string, req loopruncontract.RecordAttemptRequest) (loopruncontract.LoopRun, error)
	Stop          func(loopID string, success bool, reason string) (loopruncontract.LoopRun, error)
	Status        func(loopID string) (loopruncontract.StatusResult, error)
	ResolveID     func(repo, name string) (string, error)
}
