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
	candidates := parseNextActionCandidateFacts(message)
	if candidatesFormWellFormedChoiceSet(candidates) && countRecommendedCandidates(candidates) == 1 {
		if reason := choiceQualityBlockReason(message, candidates); reason != "" {
			result.Decision = "block"
			result.Reason = reason
			return result
		}
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
	return fmt.Sprintf("다음 행동 판단 지점에 도달했습니다. 훅이 관찰한 근거: 명시적 선택지 %d개, 추천 선택지 %s. 훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않습니다. 메인 에이전트가 현재 대화와 작업 맥락을 근거로 직접 판단하세요. 한 번에 하나의 판단만 하세요: 자동진행 또는 자동진행하지 않음 중 하나를 선택하고, 둘을 같은 답변에서 섞지 마세요. 자동진행한다면 왜 안전하고 가역적이며 사용자 의도에 맞는지 답변에 명시하고 지금 실행하세요. 자동진행 결과 보고에는 `선택지:` 3개와 정확히 하나의 `(추천)`을 포함하고, 선택지 3개와 `선택지 품질 증거`는 모두 한국어로 작성하세요. 자동진행하지 않는다면 `자동진행하지 않음`으로 시작하는 판단 줄에 왜 사용자 결정이 필요한지 또는 왜 후속 선택 지점인지 명시한 뒤 멈추세요. no-auto-proceed 판단을 남겼다면 같은 작업을 자동 goal continuation으로 재개하지 마세요. 자동진행하지 않음 판단에는 선택지 블록을 다시 붙이지 마세요.", trigger.ChoiceCount, recommended)
}

// IsNoAutoProceedJudgement reports whether the message declares a
// no-auto-proceed judgement. Only a line that STARTS with the marker counts:
// prose that merely quotes or explains the rule must not bypass the numbered
// next-actions gate.
func IsNoAutoProceedJudgement(message string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "**")
		if strings.HasPrefix(trimmed, "자동진행하지 않") {
			return true
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "no-auto-proceed") {
			return true
		}
	}
	return false
}

func missingNumberedNextActionsReason() string {
	return "Stop hook이 최종 응답에 올바른 numbered next actions가 없어 차단했습니다. Stop next-action relay에 대한 no-auto-proceed 응답이라면 `자동진행하지 않음`으로 시작하는 판단 줄만 쓰고 선택지 블록을 추가하지 마세요. 그 외에는 차단 원인을 짧게 설명한 뒤, 한국어로 작성한 `선택지:` 섹션을 제시하세요. 형식은 정확히 3개 번호 선택지, 정확히 하나의 `(추천)`, 그리고 `context 확인`, `추천 근거`, `사용자 승인 경계`를 포함한 `선택지 품질 증거` 섹션입니다."
}
