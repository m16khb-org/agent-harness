package llmeval

import "issueops/cmd/issueops/selfworkflow/model"

func ApplySelfVerifyLLMEval(result model.SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (model.SelfAugmentResult, error) {
	if !opts.Enabled {
		return result, nil
	}
	mode := NormalizeSelfVerifyLLMEvalMode(opts.Mode)
	if err := ValidateSelfVerifyLLMEvalMode(mode); err != nil {
		return result, err
	}
	evidencePacket, evidenceBytes, err := BuildSelfVerifyLLMEvalPrompt(result)
	if err != nil {
		result.LLMEval = &model.SelfVerifyLLMEvalResult{
			OK:                  false,
			Mode:                mode,
			ExecutionClass:      "foreground_blocking",
			ReadOnly:            true,
			EvidencePacketBytes: evidenceBytes,
			Error:               BoundedLLMEvalError("build LLM evidence packet", err, ""),
		}
		return ApplySelfVerifyLLMGate(result, opts.TargetScore)
	}

	eval := model.SelfVerifyLLMEvalResult{
		Mode:                mode,
		ExecutionClass:      "foreground_blocking",
		ReadOnly:            true,
		EvidencePacketBytes: evidenceBytes,
		Prompt:              evidencePacket,
		Error:               "self-verify external LLM evaluation was removed; run the rendered prompt with the host agent and record the result file",
	}
	result.LLMEval = &eval
	return ApplySelfVerifyLLMGate(result, opts.TargetScore)
}
