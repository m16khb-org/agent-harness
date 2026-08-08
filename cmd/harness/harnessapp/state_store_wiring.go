package harnessapp

import (
	hookcatalog "agent-harness/cmd/harness/hookcli/hookcatalog"
	benchmarkcmd "agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	mcpcli "agent-harness/cmd/harness/mcpcli"
	augmentlesson "agent-harness/cmd/harness/selfworkflow/augmentlesson"
	augmentplan "agent-harness/cmd/harness/selfworkflow/augmentplan"
	candidateexport "agent-harness/cmd/harness/selfworkflow/candidateexport"
	historycompare "agent-harness/cmd/harness/selfworkflow/historycompare"
	stateio "agent-harness/cmd/harness/selfworkflow/stateio"
	statuscli "agent-harness/cmd/harness/statuscli"
	stateroundtrip "agent-harness/cmd/harness/validationcli/stateroundtrip"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// configureStateStores는 harness state 접근을 설치한다.
//
// state를 어디에 저장하고 어떻게 잠그는지는 하나의 구현이고, 그 선택은
// composition root의 결정이다. CLI/MCP transport와 self-workflow는 key와 결과
// 형식만 안다.
func configureStateStores() {
	augmentlesson.StateDir = statestore.StateDir
	augmentlesson.StatePrunePrefix = statestore.StatePrunePrefix
	augmentlesson.StateWrite = statestore.StateWrite
	augmentplan.StateList = statestore.StateList
	augmentplan.StateRead = statestore.StateRead
	benchmarkcmd.StateDir = statestore.StateDir
	candidateexport.StateDir = statestore.StateDir
	candidateexport.StateWrite = statestore.StateWrite
	historycompare.StateDelete = statestore.StateDelete
	historycompare.StateDir = statestore.StateDir
	historycompare.StateList = statestore.StateList
	historycompare.StateRead = statestore.StateRead
	hookcatalog.MaybeMaintainStateStores = statestore.MaybeMaintainStateStores
	mcpcli.StateDoctor = statestore.StateDoctor
	mcpcli.StateList = statestore.StateList
	mcpcli.StateMaintain = statestore.StateMaintain
	mcpcli.StatePrune = statestore.StatePrune
	mcpcli.StateRead = statestore.StateRead
	mcpcli.StateWrite = statestore.StateWrite
	stateio.NormalizeStateKey = statestore.NormalizeStateKey
	stateio.StateDir = statestore.StateDir
	stateio.StateRead = statestore.StateRead
	stateio.StateWrite = statestore.StateWrite
	stateio.WriteStateRecord = statestore.WriteStateRecord
	stateroundtrip.StateRead = statestore.StateRead
	stateroundtrip.WriteStateRecord = statestore.WriteStateRecord
	statuscli.StateList = statestore.StateList
}
