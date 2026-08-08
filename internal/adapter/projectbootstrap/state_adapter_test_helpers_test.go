package projectbootstrap

import (
	issueopsstatepkg "agent-harness/internal/adapter/issueops"
	lifecyclestatepkg "agent-harness/internal/adapter/lifecycle"
	compactstatepkg "agent-harness/internal/adapter/lifecycle/compact"
	docupkeepstatepkg "agent-harness/internal/adapter/lifecycle/docupkeep"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {
	issueopsstatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.WithKeyLock = statestore.WithKeyLock
	compactstatepkg.WithKeyLock = statestore.WithKeyLock
	docupkeepstatepkg.WithKeyLock = statestore.WithKeyLock
}
