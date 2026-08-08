package verifyloop

import (
	summarytdeps "agent-harness/cmd/harness/selfworkflow/summary"
	failurecauseadapter "agent-harness/internal/adapter/failurecause"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	summarytdeps.Classify = failurecauseadapter.Classify
}
