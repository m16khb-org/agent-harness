package selfworkflow

import (
	"io"

	"agent-harness/cmd/harness/selfworkflow/progress"
)

type SelfVerifyProgressEvent = progress.SelfVerifyProgressEvent
type SelfVerifyProgressReporter = progress.SelfVerifyProgressReporter

func NewSelfVerifyProgressReporter(mode string, writer io.Writer) (*SelfVerifyProgressReporter, error) {
	return progress.NewSelfVerifyProgressReporter(mode, writer)
}

func boolPtr(value bool) *bool {
	return &value
}
