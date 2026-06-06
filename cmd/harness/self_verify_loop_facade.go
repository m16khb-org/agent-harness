package main

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

func boolPtr(value bool) *bool {
	return &value
}

func selfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool) (SelfAugmentResult, error) {
	return selfworkflow.SelfVerify(iterations, baseSeed, targetScore, verbose, selfVerifyLoopDeps())
}

func selfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfVerifyProgressReporter) (SelfAugmentResult, error) {
	if progress == nil {
		return selfworkflow.SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, selfVerifyLoopDeps())
	}
	return selfworkflow.SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, progress.inner, selfVerifyLoopDeps())
}

func selfVerifyLoopDeps() selfworkflow.SelfVerifyLoopDeps {
	return selfworkflow.SelfVerifyLoopDeps{
		StepDeps:   selfVerifyStepDeps(),
		FailedStep: failedStep,
		PrintStep:  printStep,
	}
}

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
	selfworkflow.HarnessRoot = harnessRoot
	return selfworkflow.NewSelfVerifyLoopResult(iterations, baseSeed, targetScore)
}

func emitSelfVerifyLoopStart(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopStart(progress.inner, loopKind, iterations, seed)
}

func emitSelfVerifyLoopEnd(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopEnd(progress.inner, loopKind, iterations, seed, ok, errorText)
}

func saveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfVerificationSummary(result, key)
}

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfAugmentSummary(result, key)
}

func newSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return selfworkflow.NewSelfVerificationSummarySnapshot(result, generatedAt)
}
