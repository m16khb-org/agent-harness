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
	result := SelfAugmentResult{
		LoopKind:    "self_verification",
		KoreanName:  selfVerificationKoreanName,
		Iterations:  iterations,
		BaseSeed:    baseSeed,
		TargetScore: targetScore,
		HarnessRoot: harnessRoot(),
		InspiredBy:  "/Users/habin/workspace/eye-tracking-scroll/scripts/self-augment.js",
		LoopContract: []string{
			"quick mode runs one deterministic evidence pass before the final LLM gate",
			"full mode requires at least 10 seeded iterations before the final LLM gate",
			"tests and QA are first-class stages, not optional follow-ups",
			"seeded per-iteration randomized git preflight fuzz",
			"repeat core invariant, tests, risk-tier QA, build, CLI/MCP schema and response contract golden, CLI, docs, command policy, MCP, state, and native integration smoke checks",
			"terminate only when every concrete goal score is greater than target_score",
			"fail fast on the first failed step and report goal scores for recovery",
		},
	}
	if progress != nil {
		progress.started = started
		progress.emit(SelfVerifyProgressEvent{
			Event:      "loop_start",
			LoopKind:   result.LoopKind,
			Iterations: iterations,
			Seed:       baseSeed,
		})
	}
	if iterations < 1 {
		err := fmt.Errorf("self-verification requires at least 1 iteration")
		result.ElapsedMS = time.Since(started).Milliseconds()
		result.Summary = summarizeSelfVerification(result, targetScore)
		if progress != nil {
			progress.emit(SelfVerifyProgressEvent{
				Event:      "loop_end",
				LoopKind:   result.LoopKind,
				Iterations: iterations,
				Seed:       baseSeed,
				OK:         boolPtr(false),
				Error:      err.Error(),
			})
		}
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
			if progress != nil {
				progress.emit(SelfVerifyProgressEvent{
					Event:      "loop_end",
					LoopKind:   result.LoopKind,
					Iterations: iterations,
					Seed:       baseSeed,
					OK:         boolPtr(false),
					Error:      err.Error(),
				})
			}
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
				if progress != nil {
					progress.emit(SelfVerifyProgressEvent{
						Event:      "loop_end",
						LoopKind:   result.LoopKind,
						Iterations: iterations,
						Seed:       baseSeed,
						OK:         boolPtr(false),
						Error:      fmt.Sprintf("%s failed: %s", step.Label, step.Error),
					})
				}
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
	if progress != nil {
		progress.emit(SelfVerifyProgressEvent{
			Event:      "loop_end",
			LoopKind:   result.LoopKind,
			Iterations: iterations,
			Seed:       baseSeed,
			OK:         boolPtr(result.OK),
		})
	}
	return result, nil
}
