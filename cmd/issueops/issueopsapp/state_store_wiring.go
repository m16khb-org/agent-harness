package issueopsapp

import (
	benchmarkcmd "issueops/cmd/issueops/issueopscli/benchmarkcmd"
	mcpcli "issueops/cmd/issueops/mcpcli"
	augmentlesson "issueops/cmd/issueops/selfworkflow/augmentlesson"
	augmentplan "issueops/cmd/issueops/selfworkflow/augmentplan"
	candidateexport "issueops/cmd/issueops/selfworkflow/candidateexport"
	historycompare "issueops/cmd/issueops/selfworkflow/historycompare"
	stateio "issueops/cmd/issueops/selfworkflow/stateio"
	statuscli "issueops/cmd/issueops/statuscli"
	stateroundtrip "issueops/cmd/issueops/validationcli/stateroundtrip"
	statestore "issueops/internal/adapter/outbound/state"
)

// configureStateStores는 issueops state 접근을 설치한다.
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
