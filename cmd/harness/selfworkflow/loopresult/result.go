package loopresult

import (
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
)

func New(iterations int, baseSeed int64, targetScore float64, root string) model.SelfAugmentResult {
	return model.SelfAugmentResult{
		LoopKind:    "self_verification",
		KoreanName:  model.SelfVerificationKoreanName,
		Iterations:  iterations,
		BaseSeed:    baseSeed,
		TargetScore: targetScore,
		HarnessRoot: root,
		InspiredBy:  "/Users/sample/workspace/eye-tracking-scroll/scripts/self-augment.js",
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

func EmitStart(reporter *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if reporter == nil {
		return
	}
	reporter.Emit(progress.SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
	})
}

func EmitEnd(reporter *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if reporter == nil {
		return
	}
	event := progress.SelfVerifyProgressEvent{
		Event:      "loop_end",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
		OK:         boolPtr(ok),
	}
	if errorText != "" {
		event.Error = errorText
	}
	reporter.Emit(event)
}

func boolPtr(value bool) *bool {
	return &value
}
