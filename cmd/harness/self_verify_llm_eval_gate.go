package main

import (
	"fmt"
	"strings"
)

func applySelfVerifyLLMGate(result SelfAugmentResult, targetScore float64) (SelfAugmentResult, error) {
	if result.LLMEval == nil || result.LLMEval.Mode != "gate" {
		return result, nil
	}
	reasons := []string{}
	if !result.LLMEval.OK {
		reasons = append(reasons, "llm_eval_not_ok")
	}
	if result.LLMEval.Score < targetScore {
		reasons = append(reasons, fmt.Sprintf("score %.2f below target %.2f", result.LLMEval.Score, targetScore))
	}
	if len(result.LLMEval.Blockers) > 0 {
		reasons = append(reasons, "blockers: "+strings.Join(result.LLMEval.Blockers, "; "))
	}
	if len(reasons) == 0 {
		return result, nil
	}
	result.OK = false
	result.TerminationEligible = false
	result.Summary.TerminationEligible = false
	return result, fmt.Errorf("LLM evaluation gate failed: %s", strings.Join(reasons, "; "))
}
