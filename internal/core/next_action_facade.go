package core

import "agent-harness/internal/core/nextaction"

type NumberedNextActionsDecisionResult = nextaction.NumberedNextActionsDecisionResult
type NextActionCandidate = nextaction.NextActionCandidate
type NextActionJudgementTriggerResult = nextaction.NextActionJudgementTriggerResult
type NextActionAutoProceedResult = nextaction.NextActionAutoProceedResult
type NextActionAutoProceedLLMRequest = nextaction.NextActionAutoProceedLLMRequest

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	return nextaction.BuildNumberedNextActionsDecision(message, enforce, source)
}

func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	return nextaction.BuildNextActionJudgementTrigger(message)
}

func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	return nextaction.EvaluateNextActionAutoProceed(message, threshold)
}

func EvaluateNextActionAutoProceedLLM(req NextActionAutoProceedLLMRequest, threshold float64) (NextActionAutoProceedResult, error) {
	return nextaction.EvaluateNextActionAutoProceedLLM(req, threshold)
}

func parseNextActionCandidates(message string) []NextActionCandidate {
	return nextaction.ParseCandidates(message)
}

func selectRecommendedNextAction(candidates []NextActionCandidate) *NextActionCandidate {
	return nextaction.SelectRecommendedCandidate(candidates)
}

func buildNextActionAutoProceedLLMPrompt(recommended NextActionCandidate, candidates []NextActionCandidate) string {
	return nextaction.BuildLLMPrompt(recommended, candidates)
}
