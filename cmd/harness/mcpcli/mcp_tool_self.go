package mcpcli

import (
	"fmt"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/cmd/harness/selfworkflow"
)

func handleSelfLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "self_augment":
		selfworkflow.Version = Version
		selfworkflow.HarnessRoot = HarnessRoot
		result := selfworkflow.PlanSelfAugmentation(selfworkflow.SelfAugmentPlanRequest{
			Cycles:      argmap.Int(call.Arguments, "cycles", 1),
			TargetScore: argmap.Float(call.Arguments, "target_score", selfworkflow.DefaultLoopTargetScoreExclusive),
		})
		if argmap.Bool(call.Arguments, "save_state") {
			if err := selfworkflow.SaveSelfAugmentPlan(&result, argmap.StringDefault(call.Arguments, "state_key", "self-augment-latest")); err != nil {
				return mcpToolFailure(&RPCError{Code: -32000, Message: "Self-augmentation plan save failed", Data: result})
			}
		}
		return mcpToolPayload(result)
	case "self_augment_lesson":
		selfworkflow.Version = Version
		selfworkflow.HarnessRoot = HarnessRoot
		result, err := selfworkflow.SaveSelfAugmentLesson(selfworkflow.SelfAugmentLessonRequest{
			CandidateID: argmap.String(call.Arguments, "candidate_id"),
			Lesson:      argmap.String(call.Arguments, "lesson"),
			NextAction:  argmap.String(call.Arguments, "next_action"),
			Source:      argmap.StringDefault(call.Arguments, "source", "self-augment"),
			Severity:    argmap.StringDefault(call.Arguments, "severity", "info"),
			StateKey:    argmap.String(call.Arguments, "state_key"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Self-augmentation lesson save failed", Data: result})
		}
		return mcpToolPayload(result)
	case "self_verify":
		runMode, modeErr := resolveSelfVerifyRunMode(argmap.Bool(call.Arguments, "full"), argmap.Set(call.Arguments, "iterations"), argmap.Int(call.Arguments, "iterations", 10))
		if modeErr != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Self-verification mode invalid", Data: modeErr.Error()})
		}
		seed := argmap.Int64(call.Arguments, "seed", time.Now().Unix())
		targetScore := argmap.Float(call.Arguments, "target_score", selfworkflow.DefaultLoopTargetScoreExclusive)
		result, err := SelfVerify(runMode.Iterations, seed, targetScore, false)
		if argmap.Bool(call.Arguments, "save_state") {
			saveErr := selfworkflow.SaveSelfVerificationSummary(&result, argmap.StringDefault(call.Arguments, "state_key", "self-verify-latest"))
			if err == nil && saveErr != nil {
				err = saveErr
			}
		}
		if err != nil && !isSelfVerificationGateError(err) {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "Self-verification failed", Data: result})
		}
		return mcpToolPayload(result)
	case "self_verify_candidates":
		selfworkflow.HarnessRoot = HarnessRoot
		result := selfworkflow.ExportSelfVerificationCandidates()
		if argmap.Bool(call.Arguments, "save_state") {
			if err := selfworkflow.SaveSelfVerificationCandidateExport(&result, argmap.StringDefault(call.Arguments, "state_key", "self-verify-candidates-latest")); err != nil {
				return mcpToolFailure(&RPCError{Code: -32000, Message: "Self-verify candidate export save failed", Data: result})
			}
		}
		return mcpToolPayload(result)
	case "self_verify_history", "self_augment_history":
		result, err := selfworkflow.SelfAugmentHistory(
			argmap.StringDefault(call.Arguments, "prefix", "self-verify"),
			argmap.Int(call.Arguments, "limit", 20),
			selfworkflow.SelfAugmentHistoryRetentionOptions{
				Limit:          argmap.Int(call.Arguments, "retention_limit", 0),
				PruneRequested: argmap.Bool(call.Arguments, "prune_retention"),
				Confirm:        argmap.Bool(call.Arguments, "confirm"),
			},
		)
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Self-verify history failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "self_verify_compare", "self_augment_compare":
		result, err := selfworkflow.CompareSelfAugmentSummaries(
			argmap.String(call.Arguments, "baseline_key"),
			argmap.String(call.Arguments, "candidate_key"),
			argmap.Float(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Self-verify compare failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "self_verify_promote", "self_augment_promote":
		result, err := selfworkflow.PromoteSelfAugmentBaseline(
			argmap.String(call.Arguments, "from_key"),
			argmap.String(call.Arguments, "baseline_key"),
			argmap.Bool(call.Arguments, "confirm"),
		)
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Self-verify promote failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}

func wrapSelfVerificationGateError(result selfworkflow.SelfAugmentResult, err error) (selfworkflow.SelfAugmentResult, error) {
	if err == nil || !isSelfVerificationGateError(err) {
		return result, err
	}
	return result, fmt.Errorf("%w: %w", ErrSelfVerificationGateFailed, err)
}
