package historycompare

import (
	stateiopkg "agent-harness/cmd/harness/selfworkflow/stateio"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {
	StateDelete = statestore.StateDelete
	StateDir = statestore.StateDir
	StateList = statestore.StateList
	StateRead = statestore.StateRead
	stateiopkg.NormalizeStateKey = statestore.NormalizeStateKey
	stateiopkg.StateDir = statestore.StateDir
	stateiopkg.StateRead = statestore.StateRead
	stateiopkg.StateWrite = statestore.StateWrite
	stateiopkg.WriteStateRecord = statestore.WriteStateRecord
}
