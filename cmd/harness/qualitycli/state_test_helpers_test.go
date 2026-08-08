package qualitycli

import (
	augmentlessonpkg "agent-harness/cmd/harness/selfworkflow/augmentlesson"
	augmentplanpkg "agent-harness/cmd/harness/selfworkflow/augmentplan"
	candidateexportpkg "agent-harness/cmd/harness/selfworkflow/candidateexport"
	historycomparepkg "agent-harness/cmd/harness/selfworkflow/historycompare"
	stateiopkg "agent-harness/cmd/harness/selfworkflow/stateio"
	statestore "agent-harness/internal/adapter/outbound/state"
	"testing"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {
	augmentlessonpkg.StateDir = statestore.StateDir
	augmentlessonpkg.StatePrunePrefix = statestore.StatePrunePrefix
	augmentlessonpkg.StateWrite = statestore.StateWrite
	augmentplanpkg.StateList = statestore.StateList
	augmentplanpkg.StateRead = statestore.StateRead
	candidateexportpkg.StateDir = statestore.StateDir
	candidateexportpkg.StateWrite = statestore.StateWrite
	historycomparepkg.StateDelete = statestore.StateDelete
	historycomparepkg.StateDir = statestore.StateDir
	historycomparepkg.StateList = statestore.StateList
	historycomparepkg.StateRead = statestore.StateRead
	stateiopkg.NormalizeStateKey = statestore.NormalizeStateKey
	stateiopkg.StateDir = statestore.StateDir
	stateiopkg.StateRead = statestore.StateRead
	stateiopkg.StateWrite = statestore.StateWrite
	stateiopkg.WriteStateRecord = statestore.WriteStateRecord
}

// configureTestStateStore는 SNR baseline 테스트가 쓰는 주입 헬퍼다.
func configureTestStateStore(t *testing.T) {
	t.Helper()
	deps := hostDeps
	deps.StateRead = statestore.StateRead
	deps.StateWrite = statestore.StateWrite
	Configure(deps)
	t.Cleanup(Reset)
}
