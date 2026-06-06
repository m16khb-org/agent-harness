package harnessapp

import (
	"io"
	"time"

	"agent-harness/cmd/harness/selfworkflow"
)

type SelfVerifyProgressEvent = selfworkflow.SelfVerifyProgressEvent

type selfVerifyProgressReporter struct {
	inner *selfworkflow.SelfVerifyProgressReporter
}

func newSelfVerifyProgressReporter(mode string, writer io.Writer) (*selfVerifyProgressReporter, error) {
	reporter, err := selfworkflow.NewSelfVerifyProgressReporter(mode, writer)
	if reporter == nil || err != nil {
		return nil, err
	}
	return &selfVerifyProgressReporter{inner: reporter}, nil
}

func (r *selfVerifyProgressReporter) emit(event SelfVerifyProgressEvent) {
	if r == nil {
		return
	}
	r.inner.Emit(event)
}

func (r *selfVerifyProgressReporter) emitStepEnd(loopKind string, iteration, iterations int, seed int64, stepIndex, stepCount int, step StepResult) {
	if r == nil {
		return
	}
	r.inner.EmitStepEnd(loopKind, iteration, iterations, seed, stepIndex, stepCount, step)
}

func (r *selfVerifyProgressReporter) setStarted(started time.Time) {
	if r == nil {
		return
	}
	r.inner.SetStarted(started)
}
