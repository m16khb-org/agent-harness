package llmeval

import (
	"strings"
	"time"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

func ApplySelfVerifyLLMEval(result model.SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (model.SelfAugmentResult, error) {
	if !opts.Enabled {
		return result, nil
	}
	mode := NormalizeSelfVerifyLLMEvalMode(opts.Mode)
	if err := ValidateSelfVerifyLLMEvalMode(mode); err != nil {
		return result, err
	}
	agyCommand := strings.TrimSpace(opts.AgyCommand)
	if agyCommand == "" {
		agyCommand = "agy"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
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

	llm, runErr := core.RunExternalLLMPrint(core.ExternalLLMPrintRequest{Provider: agyCommand, Prompt: evidencePacket, Timeout: timeout})
	eval := model.SelfVerifyLLMEvalResult{
		Mode:                mode,
		ExecutionClass:      "foreground_blocking",
		ReadOnly:            true,
		EvidencePacketBytes: evidenceBytes,
	}
	if runErr != nil {
		eval.Error = BoundedLLMEvalError("agy -p failed", runErr, string(llm.Output))
		result.LLMEval = &eval
		return ApplySelfVerifyLLMGate(result, opts.TargetScore)
	}
	if err := DecodeSelfVerifyLLMEval(llm.Output, &eval); err != nil {
		eval.OK = false
		eval.Error = BoundedLLMEvalError("parse agy JSON", err, string(llm.Output))
		result.LLMEval = &eval
		return ApplySelfVerifyLLMGate(result, opts.TargetScore)
	}
	eval.Mode = mode
	eval.EvidencePacketBytes = evidenceBytes
	result.LLMEval = &eval
	return ApplySelfVerifyLLMGate(result, opts.TargetScore)
}
