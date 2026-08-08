package harnessapp

import (
	"agent-harness/cmd/harness/statecli"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// stateDependencies는 state CLI에 concrete state store를 조립해 넘긴다.
//
// state 저장소를 아는 곳은 이 composition root 하나여야 한다. CLI가 outbound
// adapter를 직접 부르면 transport와 저장소 구현이 한 package에 묶인다.
func stateDependencies() statecli.Dependencies {
	return statecli.Dependencies{
		Write:    statestore.StateWrite,
		Read:     statestore.StateRead,
		List:     statestore.StateList,
		Prune:    statestore.StatePrune,
		Doctor:   statestore.StateDoctor,
		Maintain: statestore.StateMaintain,
	}
}
