package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func selfVerify(iterations int, baseSeed int64, targetScore float64, verbose bool) (SelfAugmentResult, error) {
	return selfVerifyWithProgress(iterations, baseSeed, targetScore, verbose, nil)
}

func selfVerifyWithProgress(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *selfVerifyProgressReporter) (SelfAugmentResult, error) {
	started := time.Now()
	result := newSelfVerifyLoopResult(iterations, baseSeed, targetScore)
	if progress != nil {
		progress.started = started
		emitSelfVerifyLoopStart(progress, result.LoopKind, iterations, baseSeed)
	}
	if iterations < 1 {
		err := fmt.Errorf("self-verification requires at least 1 iteration")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summarizeSelfVerification(result, targetScore)
		emitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, err.Error())
		return result, err
	}

	for iteration := 1; iteration <= iterations; iteration++ {
		seed := baseSeed + int64(iteration) - 1
		if verbose {
			fmt.Printf("\n=== Self-verification iteration %d/%d seed=%d ===\n", iteration, iterations, seed)
		}
		run := SelfAugmentIteration{Iteration: iteration, Seed: seed}
		tempDir, err := os.MkdirTemp("", "agent-harness-self-verify-*")
		if err != nil {
			step := failedStep("create temp workspace", err)
			run.Steps = append(run.Steps, step)
			result.Runs = append(result.Runs, run)
			result.ElapsedMS = time.Since(started).Milliseconds()
			result.Summary = summarizeSelfVerification(result, targetScore)
			emitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, err.Error())
			return result, err
		}
		tempBin := filepath.Join(tempDir, "harness")

		var goTestStep StepResult
		steps := plannedSelfVerifySteps(result.HarnessRoot, tempBin, seed, &goTestStep)

		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
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
				progress.emit(SelfVerifyProgressEvent{
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
				progress.emitStepEnd(result.LoopKind, iteration, iterations, seed, index+1, len(steps), step)
			}
			if verbose {
				printStep(step)
			}
			if !step.OK {
				_ = os.RemoveAll(tempDir)
				result.Runs = append(result.Runs, run)
				result.ElapsedMS = time.Since(started).Milliseconds()
				result.OK = false
				result.Summary = summarizeSelfVerification(result, targetScore)
				emitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, false, fmt.Sprintf("%s failed: %s", step.Label, step.Error))
				return result, fmt.Errorf("%w: %s failed: %s", errSelfVerificationGateFailed, step.Label, step.Error)
			}
		}
		_ = os.RemoveAll(tempDir)
		result.Runs = append(result.Runs, run)
		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
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
	result.Summary = summarizeSelfVerification(result, targetScore)
	result.TerminationEligible = result.Summary.TerminationEligible
	result.OK = result.TerminationEligible
	if verbose {
		fmt.Printf("\nSelf-verification pipeline passed %d iterations in %.1fs.\n", iterations, float64(result.ElapsedMS)/1000)
	}
	emitSelfVerifyLoopEnd(progress, result.LoopKind, iterations, baseSeed, result.OK, "")
	return result, nil
}
