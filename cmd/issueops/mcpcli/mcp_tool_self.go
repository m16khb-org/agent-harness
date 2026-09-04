package mcpcli

import (
	"time"

	"issueops/cmd/issueops/mcpcli/argmap"
	"issueops/cmd/issueops/selfworkflow"
)

func handleSelfLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "self_augment":
		selfworkflow.Version = Version
		selfworkflow.IssueOpsRoot = IssueOpsRoot
		result := selfworkflow.PlanSelfAugmentation(selfworkflow.SelfAugmentPlanRequest{
			Cycles:      argmap.Int(call.Arguments, "cycles", 1),
			TargetScore: argmap.Float(call.Arguments, "target_score", selfworkflow.DefaultLoopTargetScoreExclusive),
		})
		if argmap.Bool(call.Arguments, "save_state") {
			if err := selfworkflow.SaveSelfAugmentPlan(&result, argmap.StringDefault(call.Arguments, "state_key", "self-augment-latest")); err != nil {
				return mcpToolFailure(newProtocolError(-32000, "Self-augmentation plan save failed", result))
			}
		}
		return mcpToolPayload(result)
	case "self_augment_lesson":
		selfworkflow.Version = Version
		selfworkflow.IssueOpsRoot = IssueOpsRoot
		result, err := selfworkflow.SaveSelfAugmentLesson(selfworkflow.SelfAugmentLessonRequest{
			CandidateID: argmap.String(call.Arguments, "candidate_id"),
			Lesson:      argmap.String(call.Arguments, "lesson"),
			NextAction:  argmap.String(call.Arguments, "next_action"),
			Source:      argmap.StringDefault(call.Arguments, "source", "self-augment"),
			Severity:    argmap.StringDefault(call.Arguments, "severity", "info"),
			StateKey:    argmap.String(call.Arguments, "state_key"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-augmentation lesson save failed", result))
		}
		return mcpToolPayload(result)
	case "self_verify":
		seed := argmap.Int64(call.Arguments, "seed", time.Now().Unix())
		targetScore := argmap.Float(call.Arguments, "target_score", selfworkflow.DefaultLoopTargetScoreExclusive)
		result, err := SelfVerify(selfworkflow.SelfVerifyRequest{
			BaseSeed:    seed,
			TargetScore: targetScore,
		})
		if argmap.Bool(call.Arguments, "save_state") {
			saveErr := selfworkflow.SaveSelfVerificationSummary(&result, argmap.StringDefault(call.Arguments, "state_key", "self-verify-latest"))
			if err == nil && saveErr != nil {
				err = saveErr
			}
		}
		if err != nil && !isSelfVerificationGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "Self-verification failed", result))
		}
		return mcpToolPayload(result)
	case "self_verify_candidates":
		selfworkflow.IssueOpsRoot = IssueOpsRoot
		result := selfworkflow.ExportSelfVerificationCandidates()
		if argmap.Bool(call.Arguments, "save_state") {
			if err := selfworkflow.SaveSelfVerificationCandidateExport(&result, argmap.StringDefault(call.Arguments, "state_key", "self-verify-candidates-latest")); err != nil {
				return mcpToolFailure(newProtocolError(-32000, "Self-verify candidate export save failed", result))
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
			return mcpToolFailure(newProtocolError(-32602, "Self-verify history failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "self_verify_compare", "self_augment_compare":
		result, err := selfworkflow.CompareSelfAugmentSummaries(
			argmap.String(call.Arguments, "baseline_key"),
			argmap.String(call.Arguments, "candidate_key"),
			argmap.Float(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-verify compare failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "self_verify_promote", "self_augment_promote":
		result, err := selfworkflow.PromoteSelfAugmentBaseline(
			argmap.String(call.Arguments, "from_key"),
			argmap.String(call.Arguments, "baseline_key"),
			argmap.Bool(call.Arguments, "confirm"),
			argmap.Bool(call.Arguments, "allow_failed_source"),
		)
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-verify promote failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
