package hookcli

import (
	doctorstatepkg "agent-harness/internal/adapter/doctor"
	draftwikistatepkg "agent-harness/internal/adapter/draftwiki"
	hookfailurestatepkg "agent-harness/internal/adapter/hookfailure"
	hookmetricsstatepkg "agent-harness/internal/adapter/hookmetrics"
	issueopsstatepkg "agent-harness/internal/adapter/issueops"
	lifecyclestatepkg "agent-harness/internal/adapter/lifecycle"
	compactstatepkg "agent-harness/internal/adapter/lifecycle/compact"
	docupkeepstatepkg "agent-harness/internal/adapter/lifecycle/docupkeep"
	looprunstatepkg "agent-harness/internal/adapter/looprun"
	statestore "agent-harness/internal/adapter/outbound/state"
	tracestatepkg "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {
	doctorstatepkg.StateDir = statestore.StateDir
	doctorstatepkg.StateDoctor = statestore.StateDoctor
	draftwikistatepkg.StateDir = statestore.StateDir
	hookfailurestatepkg.StateDir = statestore.StateDir
	hookfailurestatepkg.WithKeyLock = statestore.WithKeyLock
	hookmetricsstatepkg.StateDir = statestore.StateDir
	hookmetricsstatepkg.WithKeyLock = statestore.WithKeyLock
	issueopsstatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.WithKeyLock = statestore.WithKeyLock
	compactstatepkg.WithKeyLock = statestore.WithKeyLock
	docupkeepstatepkg.WithKeyLock = statestore.WithKeyLock
	looprunstatepkg.StateDir = statestore.StateDir
	tracestatepkg.StateRead = statestore.StateRead
}
