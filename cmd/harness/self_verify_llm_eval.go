package main

import (
	"strings"
	"time"

	"agent-harness/internal/core"
)

type SelfVerifyLLMEvalOptions struct {
	Enabled     bool
	Mode        string
	AgyCommand  string
	TargetScore float64
	Timeout     time.Duration
}

type SelfVerifyLLMEvalResult struct {
	OK                     bool     `json:"ok"`
	Mode                   string   `json:"mode"`
	ExecutionClass         string   `json:"execution_class"`
	ReadOnly               bool     `json:"read_only"`
	Score                  float64  `json:"score"`
	Summary                string   `json:"summary,omitempty"`
	Blockers               []string `json:"blockers,omitempty"`
	Risks                  []string `json:"risks,omitempty"`
	RecommendedNextActions []string `json:"recommended_next_actions,omitempty"`
	EvidencePacketBytes    int      `json:"evidence_packet_bytes"`
	Error                  string   `json:"error,omitempty"`
}

func applySelfVerifyLLMEval(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
	if !opts.Enabled {
		return result, nil
	}
	mode := normalizeSelfVerifyLLMEvalMode(opts.Mode)
	if err := validateSelfVerifyLLMEvalMode(mode); err != nil {
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
	evidencePacket, evidenceBytes, err := buildSelfVerifyLLMEvalPrompt(result)
	if err != nil {
		result.LLMEval = &SelfVerifyLLMEvalResult{
			OK:                  false,
			Mode:                mode,
			ExecutionClass:      "foreground_blocking",
			ReadOnly:            true,
			EvidencePacketBytes: evidenceBytes,
			Error:               boundedLLMEvalError("build LLM evidence packet", err, ""),
		}
		return applySelfVerifyLLMGate(result, opts.TargetScore)
	}

	llm, runErr := core.RunExternalLLMPrint(core.ExternalLLMPrintRequest{Command: agyCommand, Prompt: evidencePacket, Timeout: timeout})
	eval := SelfVerifyLLMEvalResult{
		Mode:                mode,
		ExecutionClass:      "foreground_blocking",
		ReadOnly:            true,
		EvidencePacketBytes: evidenceBytes,
	}
	if runErr != nil {
		eval.Error = boundedLLMEvalError("agy -p failed", runErr, string(llm.Output))
		result.LLMEval = &eval
		return applySelfVerifyLLMGate(result, opts.TargetScore)
	}
	if err := decodeSelfVerifyLLMEval(llm.Output, &eval); err != nil {
		eval.OK = false
		eval.Error = boundedLLMEvalError("parse agy JSON", err, string(llm.Output))
		result.LLMEval = &eval
		return applySelfVerifyLLMGate(result, opts.TargetScore)
	}
	eval.Mode = mode
	eval.EvidencePacketBytes = evidenceBytes
	result.LLMEval = &eval
	return applySelfVerifyLLMGate(result, opts.TargetScore)
}
