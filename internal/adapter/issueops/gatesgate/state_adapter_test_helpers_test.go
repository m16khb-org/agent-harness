package gatesgate

import (
	issueopsstatepkg "agent-harness/internal/adapter/issueops"
	looprunstatepkg "agent-harness/internal/adapter/looprun"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// production wiring과 같은 state store를 설치한다. issueops/looprun StateDir은
// composition root가 채우는 함수 변수라 여기서 프로덕션 구현으로 연결한다.
// gatesgate는 loopgate를 합성하므로 looprun 상태도 열 수 있어야 한다.
func init() {
	issueopsstatepkg.StateDir = statestore.StateDir
	looprunstatepkg.StateDir = statestore.StateDir
}
