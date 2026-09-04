package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow"
)

type SelfVerifyProgressEvent = selfworkflow.SelfVerifyProgressEvent

type selfVerifyProgressReporter struct {
	inner *selfworkflow.SelfVerifyProgressReporter
}
