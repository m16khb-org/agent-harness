package harnessapp

import (
	audit "agent-harness/internal/adapter/audit"
	channel "agent-harness/internal/adapter/channel"
	doctor "agent-harness/internal/adapter/doctor"
	issueops "agent-harness/internal/adapter/issueops"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	looprun "agent-harness/internal/adapter/looprun"
	statestore "agent-harness/internal/adapter/outbound/state"
	trace "agent-harness/internal/adapter/trace"
)

// configureAdapterStateAccess는 outbound state를 쓰는 adapter들에 접근자를 설치한다.
//
// adapter가 다른 adapter의 package 함수를 직접 부르면 capability 경계를 넘는 결합이
// 된다. state store를 아는 곳은 composition root 하나여야 한다.
func configureAdapterStateAccess() {
	audit.StateDir = statestore.StateDir
	audit.WithKeyLock = statestore.WithKeyLock
	doctor.StateDir = statestore.StateDir
	doctor.StateDoctor = statestore.StateDoctor
	issueops.StateDir = statestore.StateDir
	lifecycle.StateDir = statestore.StateDir
	lifecycle.WithKeyLock = statestore.WithKeyLock
	looprun.StateDir = statestore.StateDir
	channel.StateDir = statestore.StateDir
	trace.StateRead = statestore.StateRead
}
