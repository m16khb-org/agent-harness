package main

import "time"

func handleSelfLoopMCPToolCall(call mcpToolCall) mcpToolOutcome {
	switch call.Name {
	case "self_augment":
		result := planSelfAugmentation(SelfAugmentPlanRequest{
			Cycles:      intArg(call.Arguments, "cycles", 1),
			TargetScore: floatArg(call.Arguments, "target_score", defaultLoopTargetScoreExclusive),
		})
		if boolArg(call.Arguments, "save_state") {
			if err := saveSelfAugmentPlan(&result, stringArgWithDefault(call.Arguments, "state_key", "self-augment-latest")); err != nil {
				return mcpToolFailure(&rpcError{Code: -32000, Message: "Self-augmentation plan save failed", Data: result})
			}
		}
		return mcpToolPayload(result)
	case "self_augment_lesson":
		result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{
			CandidateID: stringArg(call.Arguments, "candidate_id"),
			Lesson:      stringArg(call.Arguments, "lesson"),
			NextAction:  stringArg(call.Arguments, "next_action"),
			Source:      stringArgWithDefault(call.Arguments, "source", "self-augment"),
			Severity:    stringArgWithDefault(call.Arguments, "severity", "info"),
			StateKey:    stringArg(call.Arguments, "state_key"),
		})
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "Self-augmentation lesson save failed", Data: result})
		}
		return mcpToolPayload(result)
	case "self_verify":
		runMode, modeErr := resolveSelfVerifyRunMode(boolArg(call.Arguments, "full"), argSet(call.Arguments, "iterations"), intArg(call.Arguments, "iterations", 10))
		if modeErr != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "Self-verification mode invalid", Data: modeErr.Error()})
		}
		seed := int64Arg(call.Arguments, "seed", time.Now().Unix())
		targetScore := floatArg(call.Arguments, "target_score", defaultLoopTargetScoreExclusive)
		result, err := selfVerify(runMode.Iterations, seed, targetScore, false)
		if boolArg(call.Arguments, "save_state") {
			saveErr := saveSelfVerificationSummary(&result, stringArgWithDefault(call.Arguments, "state_key", "self-verify-latest"))
			if err == nil && saveErr != nil {
				err = saveErr
			}
		}
		if err != nil && !isSelfVerificationGateError(err) {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "Self-verification failed", Data: result})
		}
		return mcpToolPayload(result)
	case "self_verify_candidates":
		result := exportSelfVerificationCandidates()
		if boolArg(call.Arguments, "save_state") {
			if err := saveSelfVerificationCandidateExport(&result, stringArgWithDefault(call.Arguments, "state_key", "self-verify-candidates-latest")); err != nil {
				return mcpToolFailure(&rpcError{Code: -32000, Message: "Self-verify candidate export save failed", Data: result})
			}
		}
		return mcpToolPayload(result)
	case "self_verify_history", "self_augment_history":
		result, err := selfAugmentHistory(
			stringArgWithDefault(call.Arguments, "prefix", "self-verify"),
			intArg(call.Arguments, "limit", 20),
			selfAugmentHistoryRetentionOptions{
				Limit:          intArg(call.Arguments, "retention_limit", 0),
				PruneRequested: boolArg(call.Arguments, "prune_retention"),
				Confirm:        boolArg(call.Arguments, "confirm"),
			},
		)
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "Self-verify history failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "self_verify_compare", "self_augment_compare":
		result, err := compareSelfAugmentSummaries(
			stringArg(call.Arguments, "baseline_key"),
			stringArg(call.Arguments, "candidate_key"),
			floatArg(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "Self-verify compare failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "self_verify_promote", "self_augment_promote":
		result, err := promoteSelfAugmentBaseline(
			stringArg(call.Arguments, "from_key"),
			stringArg(call.Arguments, "baseline_key"),
			boolArg(call.Arguments, "confirm"),
		)
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "Self-verify promote failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	default:
		return mcpToolOutcome{}
	}
}
