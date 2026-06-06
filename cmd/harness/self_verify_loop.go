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
		steps := []selfVerifyPlannedStep{
			{Label: "harness invariants", Run: func() StepResult { return validateHarnessInvariants(result.HarnessRoot) }},
			{Label: "go test", Run: func() StepResult {
				goTestStep = runCommandStep(result.HarnessRoot, "go test", 120*time.Second, "", "go", "test", "./...", "-count=1")
				return goTestStep
			}},
			{Label: "contract golden tests", Run: func() StepResult {
				return cachedContractGoldenStep(goTestStep)
			}},
			{Label: "risk QA tier", Run: func() StepResult { return validateRiskQATier(result.HarnessRoot) }},
			{Label: "go build", Run: func() StepResult {
				return runCommandStep(result.HarnessRoot, "go build", 120*time.Second, "", "go", "build", "-o", tempBin, "./cmd/harness")
			}},
			{Label: "inspect smoke", Run: func() StepResult { return validateInspect(tempBin, result.HarnessRoot) }},
			{Label: "docs index smoke", Run: func() StepResult { return validateDocsIndex(tempBin, result.HarnessRoot) }},
			{Label: "candidate export", Run: func() StepResult { return validateSelfVerifyCandidateExport(tempBin, result.HarnessRoot, seed) }},
			{Label: "step budget baseline", Run: func() StepResult { return validateStepBudgetBaseline(tempBin, result.HarnessRoot, seed) }},
			{Label: "install dry-run smoke", Run: func() StepResult { return validateInstallDryRunSmoke(tempBin, result.HarnessRoot, seed) }},
			{Label: "command policy smoke", Run: func() StepResult { return validateCommandPolicy(tempBin, result.HarnessRoot) }},
			{Label: "command audit smoke", Run: func() StepResult { return validateCommandAudit(tempBin, result.HarnessRoot, seed) }},
			{Label: "contract check", Run: func() StepResult { return validateContractCheck(tempBin, result.HarnessRoot) }},
			{Label: "worker lifecycle smoke", Run: func() StepResult { return validateWorkerLifecycle(tempBin, result.HarnessRoot, seed) }},
			{Label: "MCP smoke", Run: func() StepResult { return validateMCP(tempBin, result.HarnessRoot) }},
			{Label: "state roundtrip", Run: func() StepResult { return validateStateRoundtrip(tempBin, result.HarnessRoot, seed) }},
			{Label: "parallel isolation", Run: func() StepResult { return validateParallelTempIsolation(tempBin, result.HarnessRoot, seed) }},
			{Label: "daemon resilience", Run: func() StepResult { return validateDaemonRestartResilience(tempBin, result.HarnessRoot, seed) }},
			{Label: "preflight fuzz", Run: func() StepResult { return validatePreflightFuzz(tempBin, result.HarnessRoot, seed) }},
			{Label: "native integration", Run: func() StepResult { return validateNativeIntegration(result.HarnessRoot) }},
			{Label: "redaction audit", Run: func() StepResult { return validateRedactionAudit(result.HarnessRoot) }},
			{Label: "QA gate", Run: func() StepResult { return validateQAGate(result.HarnessRoot) }},
		}

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

type selfVerifyPlannedStep struct {
	Label string
	Run   func() StepResult
}

func cachedContractGoldenStep(goTestStep StepResult) StepResult {
	if goTestStep.OK {
		return StepResult{
			Label:      "contract golden tests",
			Command:    "covered by go test ./... -count=1",
			OK:         true,
			DurationMS: 0,
			Stdout:     "contract golden tests already executed by full go test suite",
		}
	}
	return runCommandStep(harnessRoot(), "contract golden tests", 120*time.Second, "", "go", "test", "./cmd/harness", "-run", "Golden", "-count=1")
}
