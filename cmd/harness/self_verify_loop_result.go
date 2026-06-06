package main

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
	return SelfAugmentResult{
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
}

func emitSelfVerifyLoopStart(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if progress == nil {
		return
	}
	progress.emit(SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
	})
}

func emitSelfVerifyLoopEnd(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if progress == nil {
		return
	}
	event := SelfVerifyProgressEvent{
		Event:      "loop_end",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
		OK:         boolPtr(ok),
	}
	if errorText != "" {
		event.Error = errorText
	}
	progress.emit(event)
}
