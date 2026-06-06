package core

import (
	"fmt"
	"strings"
)

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	result := NumberedNextActionsDecisionResult{
		OK:       true,
		Decision: "allow",
		Source:   strings.TrimSpace(source),
	}
	if !enforce {
		return result
	}
	message = strings.TrimSpace(message)
	if message == "" {
		result.Decision = "allow"
		result.Reason = "no assistant message available to inspect"
		return result
	}
	if hasNumberedNextActions(message) && hasExactlyOneRecommendedNextAction(message) {
		return result
	}
	result.Decision = "block"
	result.Reason = missingNumberedNextActionsReason()
	return result
}

type NextActionCandidate struct {
	Index       int     `json:"index"`
	Text        string  `json:"text"`
	Recommended bool    `json:"recommended"`
	Destructive bool    `json:"destructive"`
	Score       float64 `json:"score"`
}

type NextActionJudgementTriggerResult struct {
	OK                 bool                  `json:"ok"`
	ShouldReenterAgent bool                  `json:"should_reenter_agent"`
	ChoicesFound       bool                  `json:"choices_found"`
	ChoiceCount        int                   `json:"choice_count"`
	RecommendedCount   int                   `json:"recommended_count"`
	RecommendedIndex   int                   `json:"recommended_index,omitempty"`
	RecommendedText    string                `json:"recommended_text,omitempty"`
	Reason             string                `json:"reason"`
	Evidence           []string              `json:"evidence"`
	Candidates         []NextActionCandidate `json:"candidates"`
}

func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	result := NextActionJudgementTriggerResult{OK: true, Candidates: []NextActionCandidate{}}
	candidates := parseNextActionCandidateFacts(message)
	if len(candidates) == 0 {
		result.Reason = "no explicit next-action choices found"
		return result
	}
	result.ChoicesFound = true
	result.ShouldReenterAgent = true
	result.ChoiceCount = len(candidates)
	result.Candidates = candidates
	result.Evidence = append(result.Evidence, fmt.Sprintf("explicit next-action choices found: %d", len(candidates)))
	for _, candidate := range candidates {
		if !candidate.Recommended {
			continue
		}
		result.RecommendedCount++
		if result.RecommendedIndex == 0 {
			result.RecommendedIndex = candidate.Index
			result.RecommendedText = candidate.Text
		}
	}
	result.Evidence = append(result.Evidence, fmt.Sprintf("recommended marker count: %d", result.RecommendedCount))
	switch result.RecommendedCount {
	case 0:
		result.Reason = "next-action choices found without an explicit recommendation"
	case 1:
		result.Reason = "next-action choices found with exactly one explicit recommendation"
	default:
		result.Reason = "next-action choices found with multiple explicit recommendations"
	}
	return result
}

func missingNumberedNextActionsReason() string {
	return "Stop hook blocked because the final response lacks well-formed numbered next actions. Continue by briefly explaining that missing or malformed next-action choices caused the block, then present a context-specific `선택지:` section with exactly three numbered options and exactly one `(추천)` option."
}
