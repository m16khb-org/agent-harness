package qualitycli

import (
	"testing"

	statestore "agent-harness/internal/adapter/outbound/state"
)

// configureTestStateStore는 production wiring과 같은 state store를 주입한다.
// fitness graph는 test import를 수집하지 않으므로 여기서는 concrete를 써도 된다.
func configureTestStateStore(t *testing.T) {
	t.Helper()
	deps := hostDeps
	deps.StateRead = statestore.StateRead
	deps.StateWrite = statestore.StateWrite
	Configure(deps)
	t.Cleanup(Reset)
}
