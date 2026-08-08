package vcsissue

import (
	issueopsstatepkg "agent-harness/internal/adapter/issueops"
	lifecyclestatepkg "agent-harness/internal/adapter/lifecycle"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// production wiring과 같은 state store를 설치한다. 이 package는 hook 경로에서
// issueops와 lifecycle을 거쳐 state에 닿는다.
func init() {
	issueopsstatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.StateDir = statestore.StateDir
	lifecyclestatepkg.WithKeyLock = statestore.WithKeyLock
}
