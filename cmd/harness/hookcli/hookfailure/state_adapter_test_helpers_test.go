package hookfailure

import (
	hookfailurestatepkg "agent-harness/internal/adapter/hookfailure"
	hookmetricsstatepkg "agent-harness/internal/adapter/hookmetrics"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// production wiring과 같은 state store를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다 — 역방향으로 채우면 import 순환이 된다.
func init() {
	hookfailurestatepkg.StateDir = statestore.StateDir
	hookfailurestatepkg.WithKeyLock = statestore.WithKeyLock
	hookmetricsstatepkg.StateDir = statestore.StateDir
	hookmetricsstatepkg.WithKeyLock = statestore.WithKeyLock
}
