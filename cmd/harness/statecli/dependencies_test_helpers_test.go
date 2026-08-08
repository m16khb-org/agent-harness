package statecli

import (
	statestore "agent-harness/internal/adapter/outbound/state"
)

// testDependencies는 실제 state store를 조립한다. fitness graph는 test import를
// 수집하지 않으므로, CLI 동작은 production wiring과 같은 구현으로 검증한다.
func testDependencies() Dependencies {
	return Dependencies{
		Write:    statestore.StateWrite,
		Read:     statestore.StateRead,
		List:     statestore.StateList,
		Prune:    statestore.StatePrune,
		Doctor:   statestore.StateDoctor,
		Maintain: statestore.StateMaintain,
	}
}
