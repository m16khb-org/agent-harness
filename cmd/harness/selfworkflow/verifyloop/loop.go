package verifyloop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/selfworkflow/loopresult"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
	"agent-harness/cmd/harness/selfworkflow/steps"
	"agent-harness/cmd/harness/selfworkflow/summary"
)

var ErrSelfVerificationGateFailed = errors.New("self-verification quality gate failed")

type Deps struct {
	HarnessRoot func() string
	StepDeps    steps.SelfVerifyStepDeps
	FailedStep  func(string, error) commandstep.StepResult
	PrintStep   func(commandstep.StepResult)
	Printf      func(string, ...any) (int, error)
	// CollectAllSteps, when true, keeps running the remaining steps of an
	// iteration after a step fails (instead of fail-fast), so EVERY failing gate
	// is surfaced for concurrent regression diagnosis. Default false preserves
	// the fail-fast behavior. It NEVER weakens the gate: the iteration and the
	// overall verdict still fail, and ErrSelfVerificationGateFailed is returned.
	CollectAllSteps bool
}

func SelfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool, deps Deps) (model.SelfAugmentResult, error) {
	return SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, deps)
}

func SelfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, reporter *progress.SelfVerifyProgressReporter, deps Deps) (model.SelfAugmentResult, error) {
	deps = deps.withDefaults()
	started := time.Now()
	result := loopresult.New(iterations, baseSeed, targetScore, deps.HarnessRoot())
	if reporter != nil {
		reporter.SetStarted(started)
		loopresult.EmitStart(reporter, result.LoopKind, iterations, baseSeed)
	}
	if iterations < 1 {
		err := fmt.Errorf("self-verification requires at least 1 iteration")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summary.SummarizeSelfVerification(result, targetScore)
		loopresult.EmitEnd(reporter, result.LoopKind, iterations, baseSeed, false, err.Error())
		return result, err
	}

	for iteration := 1; iteration <= iterations; iteration++ {
		seed := baseSeed + int64(iteration) - 1
		if verbose {
			_, _ = deps.Printf("\n=== Self-verification iteration %d/%d seed=%d ===\n", iteration, iterations, seed)
		}
		run := model.SelfAugmentIteration{Iteration: iteration, Seed: seed}
		tempDir, err := os.MkdirTemp("", "agent-harness-self-verify-*")
		if err != nil {
			step := deps.FailedStep("create temp workspace", err)
			run.Steps = append(run.Steps, step)
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.Summary = summary.SummarizeSelfVerification(result, targetScore)
			loopresult.EmitEnd(reporter, result.LoopKind, iterations, baseSeed, false, err.Error())
			return result, err
		}
		tempBin := filepath.Join(tempDir, "harness")

		var goTestStep commandstep.StepResult
		plannedSteps := steps.PlannedSelfVerifySteps(result.HarnessRoot, tempBin, seed, &goTestStep, deps.StepDeps)

		if reporter != nil {
			reporter.Emit(progress.SelfVerifyProgressEvent{
				Event:      "iteration_start",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				StepCount:  len(plannedSteps),
			})
		}
		iterationFailed := false
		var firstFailure commandstep.StepResult
		for index, plannedStep := range plannedSteps {
			if reporter != nil {
				reporter.Emit(progress.SelfVerifyProgressEvent{
					Event:      "step_start",
					LoopKind:   result.LoopKind,
					Iteration:  iteration,
					Iterations: iterations,
					Seed:       seed,
					StepIndex:  index + 1,
					StepCount:  len(plannedSteps),
					Step:       plannedStep.Label,
				})
			}
			step := plannedStep.Run()
			run.Steps = append(run.Steps, step)
			if reporter != nil {
				reporter.EmitStepEnd(result.LoopKind, iteration, iterations, seed, index+1, len(plannedSteps), step)
			}
			if verbose {
				deps.PrintStep(step)
			}
			if !step.OK {
				if !iterationFailed {
					firstFailure = step
				}
				iterationFailed = true
				if !deps.CollectAllSteps {
					// fail-fast: stop at the first failed gate (default).
					break
				}
				// collect-all-steps: keep running so every failing gate is
				// surfaced. Each label runs exactly once, so the goal scorer
				// cannot be masked by a later same-label success.
			}
		}
		_ = os.RemoveAll(tempDir)
		if iterationFailed {
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.OK = false
			result.Summary = summary.SummarizeSelfVerification(result, targetScore)
			loopresult.EmitEnd(reporter, result.LoopKind, iterations, baseSeed, false, fmt.Sprintf("%s failed: %s", firstFailure.Label, firstFailure.Error))
			return result, fmt.Errorf("%w: %s failed: %s", ErrSelfVerificationGateFailed, firstFailure.Label, firstFailure.Error)
		}
		result.Runs = append(result.Runs, run)
		if reporter != nil {
			reporter.Emit(progress.SelfVerifyProgressEvent{
				Event:      "iteration_end",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				OK:         boolPtr(true),
				StepCount:  len(plannedSteps),
			})
		}
	}

	result.OK = true
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Summary = summary.SummarizeSelfVerification(result, targetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if verbose {
		_, _ = deps.Printf("\nSelf-verification pipeline passed %d iterations in %.1fs.\n", iterations, float64(result.ElapsedMS)/1000)
	}
	loopresult.EmitEnd(reporter, result.LoopKind, iterations, baseSeed, result.OK, "")
	return result, nil
}

func (deps Deps) withDefaults() Deps {
	if deps.HarnessRoot == nil {
		deps.HarnessRoot = func() string { return "." }
	}
	if deps.FailedStep == nil {
		deps.FailedStep = commandstep.FailedStep
	}
	if deps.PrintStep == nil {
		deps.PrintStep = commandstep.PrintStep
	}
	if deps.Printf == nil {
		deps.Printf = fmt.Printf
	}
	return deps
}

func boolPtr(value bool) *bool {
	return &value
}
