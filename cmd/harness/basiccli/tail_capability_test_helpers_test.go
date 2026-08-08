package basiccli

import (
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
	tracetdeps "agent-harness/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	tracetdeps.Classify = failurecauseadapter.Classify
}
