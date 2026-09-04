package toolconformance

import (
	failurecauseadapter "issueops/internal/adapter/failurecause"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	ClassifyFailureCause = failurecauseadapter.Classify
}
