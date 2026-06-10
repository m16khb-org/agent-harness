package nextaction

import (
	"fmt"
	"strings"
)

type NumberedNextActionsDecisionResult struct {
	OK       bool   `json:"ok"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Source   string `json:"source"`
}

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

func BuildJudgementRelayReason(trigger NextActionJudgementTriggerResult) string {
	recommended := "없음"
	if trigger.RecommendedCount == 1 {
		recommended = fmt.Sprintf("%d번 %q", trigger.RecommendedIndex, trigger.RecommendedText)
	} else if trigger.RecommendedCount > 1 {
		recommended = fmt.Sprintf("%d개", trigger.RecommendedCount)
	}
	return fmt.Sprintf("다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거: 명시적 선택지 %d개, 추천 선택지 %s. 훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않습니다. 메인 에이전트가 현재 대화와 작업 맥락을 근거로 직접 판단하세요. 한 번에 하나의 판단만 하세요: 자동진행 또는 자동진행하지 않음 중 하나를 선택하고, 둘을 같은 답변에서 섞지 마세요. 자동진행한다면 왜 안전하고 가역적이며 사용자 의도에 맞는지 답변에 명시하고 지금 실행하세요. 자동진행하지 않는다면 왜 사용자 결정이 필요한지 또는 왜 후속 선택 지점인지 답변에 명시한 뒤 멈추세요. no-auto-proceed 판단을 남겼다면 같은 작업을 자동 goal continuation으로 재개하지 마세요. 자동진행 결과 보고에도 `선택지:` 3개와 정확히 하나의 `(추천)`을 포함하세요.", trigger.ChoiceCount, recommended)
}

func IsNoAutoProceedJudgement(message string) bool {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(trimmed, "자동진행하지") ||
		strings.Contains(lower, "no-auto-proceed")
}

func missingNumberedNextActionsReason() string {
	return "Stop hook blocked because the final response lacks well-formed numbered next actions. If this is a no-auto-proceed response to a Stop next-action relay, state the no-auto-proceed rationale and stop without adding another choices block. Otherwise, continue by briefly explaining that missing or malformed next-action choices caused the block, then present a context-specific `선택지:` section with exactly three numbered options and exactly one `(추천)` option."
}
