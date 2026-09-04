package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"issueops/cmd/issueops/commandstep"
)

type StepResult = commandstep.StepResult

func NewSelfVerifyProgressReporter(mode string, writer io.Writer) (*SelfVerifyProgressReporter, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" || mode == "none" {
		return nil, nil
	}
	if mode != "jsonl" {
		return nil, fmt.Errorf("unsupported self-verify progress mode %q; use none or jsonl", mode)
	}
	if writer == nil {
		writer = io.Discard
	}
	return &SelfVerifyProgressReporter{mode: mode, writer: writer, started: time.Now()}, nil
}

func (r *SelfVerifyProgressReporter) Emit(event SelfVerifyProgressEvent) {
	if r == nil || r.mode == "" {
		return
	}
	if event.ElapsedMS == 0 {
		event.ElapsedMS = time.Since(r.started).Milliseconds()
	}
	if event.LastSuccess == "" {
		event.LastSuccess = r.lastSuccess
	}
	b, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(r.writer, `{"event":"progress_error","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(r.writer, string(b))
}

func (r *SelfVerifyProgressReporter) EmitStepEnd(loopKind string, iteration, iterations int, seed int64, stepIndex, stepCount int, step StepResult) {
	if r == nil {
		return
	}
	lastSuccess := r.lastSuccess
	if step.OK {
		lastSuccess = step.Label
	}
	event := SelfVerifyProgressEvent{
		Event:       "step_end",
		LoopKind:    loopKind,
		Iteration:   iteration,
		Iterations:  iterations,
		Seed:        seed,
		StepIndex:   stepIndex,
		StepCount:   stepCount,
		Step:        step.Label,
		OK:          boolPtr(step.OK),
		DurationMS:  step.DurationMS,
		LastSuccess: lastSuccess,
		Error:       step.Error,
	}
	r.Emit(event)
	if step.OK {
		r.lastSuccess = step.Label
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func (r *SelfVerifyProgressReporter) SetStarted(started time.Time) {
	if r == nil {
		return
	}
	r.started = started
}
