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
}

type Request struct {
	BaseSeed        int64
	TargetScore     float64
	Verbose         bool
	Reporter        *progress.SelfVerifyProgressReporter
	CollectAllSteps bool
}

func SelfVerify(request Request, deps Deps) (model.SelfAugmentResult, error) {
	const iterations = 1
	deps = deps.withDefaults()
	started := time.Now()
	result := loopresult.New(iterations, request.BaseSeed, request.TargetScore, deps.HarnessRoot())
	if request.Reporter != nil {
		request.Reporter.SetStarted(started)
		loopresult.EmitStart(request.Reporter, result.LoopKind, iterations, request.BaseSeed)
	}

	const iteration = 1
	seed := request.BaseSeed
	if request.Verbose {
		_, _ = deps.Printf("\n=== Self-verification evidence pass seed=%d ===\n", seed)
	}
	run := model.SelfAugmentIteration{Iteration: iteration, Seed: seed}
	tempDir, err := os.MkdirTemp("", "agent-harness-self-verify-*")
	if err != nil {
		step := deps.FailedStep("create temp workspace", err)
		run.Steps = append(run.Steps, step)
		result.Runs = append(result.Runs, run)
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summary.SummarizeSelfVerification(result, request.TargetScore)
		loopresult.EmitEnd(request.Reporter, result.LoopKind, iterations, request.BaseSeed, false, err.Error())
		return result, err
	}
	defer os.RemoveAll(tempDir)
	tempBin := filepath.Join(tempDir, "harness")

	var goTestStep commandstep.StepResult
	plannedSteps := steps.PlannedSelfVerifySteps(
		result.HarnessRoot,
		tempBin,
		seed,
		&goTestStep,
		deps.StepDeps,
	)
	if request.Reporter != nil {
		request.Reporter.Emit(progress.SelfVerifyProgressEvent{
			Event:      "iteration_start",
			LoopKind:   result.LoopKind,
			Iteration:  iteration,
			Iterations: iterations,
			Seed:       seed,
			StepCount:  len(plannedSteps),
		})
	}
	failed := false
	var firstFailure commandstep.StepResult
	for index, plannedStep := range plannedSteps {
		if request.Reporter != nil {
			request.Reporter.Emit(progress.SelfVerifyProgressEvent{
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
		if request.Reporter != nil {
			request.Reporter.EmitStepEnd(
				result.LoopKind,
				iteration,
				iterations,
				seed,
				index+1,
				len(plannedSteps),
				step,
			)
		}
		if request.Verbose {
			deps.PrintStep(step)
		}
		if !step.OK {
			if !failed {
				firstFailure = step
			}
			failed = true
			if !request.CollectAllSteps {
				break
			}
		}
	}
	result.Runs = append(result.Runs, run)
	if failed {
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.OK = false
		result.Summary = summary.SummarizeSelfVerification(result, request.TargetScore)
		message := fmt.Sprintf("%s failed: %s", firstFailure.Label, firstFailure.Error)
		loopresult.EmitEnd(
			request.Reporter,
			result.LoopKind,
			iterations,
			request.BaseSeed,
			false,
			message,
		)
		return result, fmt.Errorf("%w: %s", ErrSelfVerificationGateFailed, message)
	}
	if request.Reporter != nil {
		request.Reporter.Emit(progress.SelfVerifyProgressEvent{
			Event:      "iteration_end",
			LoopKind:   result.LoopKind,
			Iteration:  iteration,
			Iterations: iterations,
			Seed:       seed,
			OK:         boolPtr(true),
			StepCount:  len(plannedSteps),
		})
	}
	result.OK = true
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Summary = summary.SummarizeSelfVerification(result, request.TargetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if request.Verbose {
		_, _ = deps.Printf(
			"\nSelf-verification pipeline passed one evidence pass in %.1fs.\n",
			float64(result.ElapsedMS)/1000,
		)
	}
	loopresult.EmitEnd(
		request.Reporter,
		result.LoopKind,
		iterations,
		request.BaseSeed,
		result.OK,
		"",
	)
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
