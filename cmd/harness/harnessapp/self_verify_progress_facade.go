package harnessapp

import (
	"agent-harness/cmd/harness/selfworkflow"
)

type SelfVerifyProgressEvent = selfworkflow.SelfVerifyProgressEvent

type selfVerifyProgressReporter struct {
	inner *selfworkflow.SelfVerifyProgressReporter
}
