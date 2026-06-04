package core

import (
	"fmt"
	"strings"
	"time"
)

// DEPRECATED / CURRENTLY UNUSED: as of 2026-06-04 the Stop hook no longer calls
// this external-LLM gate. A synchronous agy/Gemini call measured ~13-25s, which is
// unusable inside a Stop hook's latency budget, so the live decision path uses only
// the static heuristic (EvaluateNextActionAutoProceed) and the gate's intent is
// delivered to the main agent as prompting via the UserPromptSubmit hook. This code
// and its tests are intentionally preserved for possible future use behind a faster
// model; do not wire it back into a latency-bounded hook without re-checking the
// model's real-world latency against the hook timeout.
//
// NextActionAutoProceedLLMRequest configures the external-LLM auto-proceed gate.
// When used, the LLM is the primary decision; the static heuristic in
// EvaluateNextActionAutoProceed is the fallback the caller uses on any error.
type NextActionAutoProceedLLMRequest struct {
	Message    string
	AgyCommand string
	WorkDir    string
	Timeout    time.Duration
}

type nextActionAutoProceedLLMResponse struct {
	AutoProceed bool   `json:"auto_proceed"`
	Reason      string `json:"reason"`
}

// EvaluateNextActionAutoProceedLLM asks an external LLM whether the explicitly
// recommended next action is safe to auto-execute without user confirmation.
// The destructive static guard ALWAYS applies and short-circuits before any LLM
// call: the LLM can never upgrade a destructive/irreversible action. On any LLM
// transport or decode error this returns a zero result and a wrapped error so
// the caller can fall back to the static heuristic.
func EvaluateNextActionAutoProceedLLM(req NextActionAutoProceedLLMRequest, threshold float64) (NextActionAutoProceedResult, error) {
	if threshold <= 0 {
		threshold = defaultNextActionAutoProceedThreshold
	}
	result := NextActionAutoProceedResult{OK: true, Threshold: threshold, Candidates: []NextActionCandidate{}}

	candidates := parseNextActionCandidates(req.Message)
	if len(candidates) < 2 {
		result.Reason = "no numbered next-action choices to evaluate"
		return result, nil
	}
	result.Candidates = candidates

	recommended := selectRecommendedNextAction(candidates)
	if recommended == nil {
		result.Reason = "no explicitly recommended next action; user decision required"
		return result, nil
	}
	result.SelectedIndex = recommended.Index
	result.SelectedText = recommended.Text

	// SAFETY HARD-VETO (no LLM): a destructive/irreversible recommendation never
	// auto-proceeds regardless of what the LLM would say. Short-circuit so no
	// external call is made.
	if nextActionIsDestructive(recommended.Text) {
		result.BlockedByGuard = "destructive_action"
		result.Reason = "recommended action is destructive or irreversible; user decision required regardless of LLM judgement"
		return result, nil
	}

	command := strings.TrimSpace(req.AgyCommand)
	if command == "" {
		command = "agy"
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 25 * time.Second
	}

	prompt := buildNextActionAutoProceedLLMPrompt(*recommended, candidates)

	llm, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Command: command, WorkDir: req.WorkDir, Prompt: prompt, Timeout: timeout})
	if err != nil {
		return NextActionAutoProceedResult{}, fmt.Errorf("next-action auto-proceed LLM call failed: %w", err)
	}

	var response nextActionAutoProceedLLMResponse
	if err := DecodeExternalLLMStructuredJSONObject("agy next-action auto-proceed", llm.Output, &response); err != nil {
		return NextActionAutoProceedResult{}, fmt.Errorf("next-action auto-proceed LLM decode failed: %w", err)
	}

	result.AutoProceed = response.AutoProceed
	if response.AutoProceed {
		result.TopScore = 1.0
	} else {
		result.TopScore = 0.0
	}
	reason := strings.TrimSpace(response.Reason)
	if reason == "" {
		if response.AutoProceed {
			reason = "LLM judged the recommended action safe to auto-execute"
		} else {
			reason = "LLM judged the recommended action requires user confirmation"
		}
	}
	result.Reason = reason
	return result, nil
}

func buildNextActionAutoProceedLLMPrompt(recommended NextActionCandidate, candidates []NextActionCandidate) string {
	var choices strings.Builder
	for _, candidate := range candidates {
		marker := ""
		if candidate.Recommended {
			marker = " (recommended)"
		}
		fmt.Fprintf(&choices, "%d. %s%s\n", candidate.Index, candidate.Text, marker)
	}
	return BuildStructuredPrompt(StructuredPromptSpec{
		Identity:  "You are a cautious release-safety gate deciding whether an agent may auto-execute its recommended next action without asking the user.",
		Objective: "Given the explicitly recommended next action and the full list of choices, decide whether it is safe to AUTO-EXECUTE the recommended action without user confirmation.",
		Phases: []string{
			"Judge whether the recommended action is a confident, forward step.",
			"Judge whether it is reversible and free of external or irreversible side effects (no push, deploy, release, publish, merge, send, payment, infra apply, data drop).",
			"Set auto_proceed true only when the action is confident, forward, reversible, and side-effect-free; otherwise false.",
		},
		Inputs: []string{"The recommended next action.", "The full numbered choice list."},
		Rules: []string{
			"Auto-proceed only when there is no doubt the action is safe to run unattended.",
			"Any external, outbound, or irreversible side effect means auto_proceed must be false.",
			"Any ambiguity or uncertainty about intent means auto_proceed must be false.",
		},
		OutputContract: []string{
			"Return one JSON object matching the response schema.",
			"auto_proceed is true only when the recommended action is safe to auto-execute unattended.",
			"reason concisely justifies the decision.",
		},
		VerificationChecklist: []string{
			"auto_proceed is false whenever the action has any irreversible or external side effect.",
			"reason is grounded in the recommended action text.",
		},
		Data: []PromptDataSection{
			BuildExternalLLMJSONSchemaSection(nextActionAutoProceedLLMResponseSchemaExample(), []string{
				"auto_proceed: boolean, required, true only when safe to auto-execute unattended.",
				"reason: string, required, concise justification.",
			}),
			{Title: "Recommended Next Action", Content: recommended.Text},
			{Title: "All Next-Action Choices", Content: strings.TrimSpace(choices.String())},
		},
	})
}

func nextActionAutoProceedLLMResponseSchemaExample() string {
	return `{
  "auto_proceed": false,
  "reason": "Concise justification grounded in the recommended action."
}`
}
