package basiccli

import (
	failurecauseadapter "issueops/internal/adapter/failurecause"
	tracetdeps "issueops/internal/adapter/trace"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	tracetdeps.Classify = failurecauseadapter.Classify
}
