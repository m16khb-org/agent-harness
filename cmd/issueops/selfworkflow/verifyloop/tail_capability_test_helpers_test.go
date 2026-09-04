package verifyloop

import (
	summarytdeps "issueops/cmd/issueops/selfworkflow/summary"
	failurecauseadapter "issueops/internal/adapter/failurecause"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	summarytdeps.Classify = failurecauseadapter.Classify
}
