package selfworkflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

var ErrSelfVerificationGateFailed = errors.New("self-verification quality gate failed")

type SelfVerifyLoopDeps struct {
	StepDeps   SelfVerifyStepDeps
	FailedStep func(string, error) StepResult
	PrintStep  func(StepResult)
	Printf     func(string, ...any) (int, error)
}

func SelfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	return SelfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil, deps)
}

func SelfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *SelfVerifyProgressReporter, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	deps = deps.withDefaults()
	started := time.Now()
	result := NewSelfVerifyLoopResult(iterations, baseSeed, targetScore)
	if progress != nil {
		progress.SetStarted(started)
		EmitSelfVerifyLoopStart(progress, result.LoopKind, iterations, baseSeed)
	}
	if iterations < 1 {
		err := fmt.Errorf("self-verification requires at least 1 iteration")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = SummarizeSelfVerification(result, targetScore)
		EmitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, err.Error())
		return result, err
	}

	for iteration := 1; iteration <= iterations; iteration++ {
		seed := baseSeed + int64(iteration) - 1
		if verbose {
			_, _ = deps.Printf("\n=== Self-verification iteration %d/%d seed=%d ===\n", iteration, iterations, seed)
		}
		run := SelfAugmentIteration{Iteration: iteration, Seed: seed}
		tempDir, err := os.MkdirTemp("", "agent-harness-self-verify-*")
		if err != nil {
			step := deps.FailedStep("create temp workspace", err)
			run.Steps = append(run.Steps, step)
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.Summary = SummarizeSelfVerification(result, targetScore)
			EmitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, err.Error())
			return result, err
		}
		tempBin := filepath.Join(tempDir, "harness")

		var goTestStep StepResult
		steps := PlannedSelfVerifySteps(result.HarnessRoot, tempBin, seed, &goTestStep, deps.StepDeps)

		if progress != nil {
			progress.Emit(SelfVerifyProgressEvent{
				Event:      "iteration_start",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				StepCount:  len(steps),
			})
		}
		for index, plannedStep := range steps {
			if progress != nil {
				progress.Emit(SelfVerifyProgressEvent{
					Event:      "step_start",
					LoopKind:   result.LoopKind,
					Iteration:  iteration,
					Iterations: iterations,
					Seed:       seed,
					StepIndex:  index + 1,
					StepCount:  len(steps),
					Step:       plannedStep.Label,
				})
			}
			step := plannedStep.Run()
			run.Steps = append(run.Steps, step)
			if progress != nil {
				progress.EmitStepEnd(result.LoopKind, iteration, iterations, seed, index+1, len(steps), step)
			}
			if verbose {
				deps.PrintStep(step)
			}
			if !step.OK {
				_ = os.RemoveAll(tempDir)
				result.Runs = append(result.Runs, run)
				result.ElapsedMS = time.Since(started).Milliseconds()
				result.OK = false
				result.Summary = SummarizeSelfVerification(result, targetScore)
				EmitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, fmt.Sprintf("%s failed: %s", step.Label, step.Error))
				return result, fmt.Errorf("%w: %s failed: %s", ErrSelfVerificationGateFailed, step.Label, step.Error)
			}
		}
		_ = os.RemoveAll(tempDir)
		result.Runs = append(result.Runs, run)
		if progress != nil {
			progress.Emit(SelfVerifyProgressEvent{
				Event:      "iteration_end",
				LoopKind:   result.LoopKind,
				Iteration:  iteration,
				Iterations: iterations,
				Seed:       seed,
				OK:         boolPtr(true),
				StepCount:  len(steps),
			})
		}
	}

	result.OK = true
	result.ElapsedMS = time.Since(started).Milliseconds()
	result.Summary = SummarizeSelfVerification(result, targetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if verbose {
		_, _ = deps.Printf("\nSelf-verification pipeline passed %d iterations in %.1fs.\n", iterations, float64(result.ElapsedMS)/1000)
	}
	EmitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, result.OK, "")
	return result, nil
}

func (deps SelfVerifyLoopDeps) withDefaults() SelfVerifyLoopDeps {
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
