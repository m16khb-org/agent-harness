package nextaction

import (
	"strings"
	"unicode"
)

func choiceQualityBlockReason(message string, candidates []NextActionCandidate) string {
	if !hasChoiceQualityEvidence(message) {
		return "Stop hook blocked because the final response lacks choice quality evidence. Add a `선택지 품질 증거` section with context 확인, 추천 근거, and 사용자 승인 경계."
	}
	if choicesIgnoreConversationLanguage(message, candidates) {
		return "Stop hook blocked because the next-action choices do not match the conversation language. Use Korean choices in a Korean conversation."
	}
	if hasDuplicateChoiceMeaning(candidates) {
		return "Stop hook blocked because the next-action choices are not distinct. Present three materially different actions instead of repeated no-op or equivalent choices."
	}
	if recommendedChoiceIsDestructive(candidates) {
		return "Stop hook blocked because the recommended next action appears destructive or externally mutating. Do not recommend push/delete/deploy/merge/destructive actions without an explicit user choice."
	}
	return ""
}

func hasChoiceQualityEvidence(message string) bool {
	lower := strings.ToLower(message)
	hasHeader := strings.Contains(message, "선택지 품질 증거") || strings.Contains(lower, "choice quality evidence")
	if !hasHeader {
		return false
	}
	hasContext := strings.Contains(message, "context 확인") || strings.Contains(message, "context확인") || strings.Contains(lower, "context")
	hasRecommendation := strings.Contains(message, "추천 근거") || strings.Contains(lower, "recommendation rationale") || strings.Contains(lower, "safe/reversible/aligned")
	hasBoundary := strings.Contains(message, "사용자 승인 경계") || strings.Contains(lower, "approval boundary") || strings.Contains(lower, "user approval")
	return hasContext && hasRecommendation && hasBoundary
}

func hasDuplicateChoiceMeaning(candidates []NextActionCandidate) bool {
	seen := map[string]bool{}
	noOpCount := 0
	for _, candidate := range candidates {
		normalized := normalizeChoiceMeaning(candidate.Text)
		if normalized != "" {
			if seen[normalized] {
				return true
			}
			seen[normalized] = true
		}
		if choiceLooksLikeNoOp(candidate.Text) {
			noOpCount++
		}
	}
	return noOpCount > 1
}

func normalizeChoiceMeaning(text string) string {
	text = strings.ReplaceAll(text, "(추천)", "")
	text = strings.ReplaceAll(strings.ToLower(text), "(recommended)", "")
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func choiceLooksLikeNoOp(text string) bool {
	lower := strings.ToLower(text)
	needles := []string{
		"완료", "종료", "추가 작업 없음", "추가작업없", "그대로", "유지", "마침",
		"finish", "done", "no further", "do nothing", "as-is", "as is", "keep",
	}
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func recommendedChoiceIsDestructive(candidates []NextActionCandidate) bool {
	for _, candidate := range candidates {
		if candidate.Recommended {
			return nextActionIsDestructive(candidate.Text)
		}
	}
	return false
}

func choicesIgnoreConversationLanguage(message string, candidates []NextActionCandidate) bool {
	if !containsHangul(message) {
		return false
	}
	nonKoreanChoices := 0
	for _, candidate := range candidates {
		if !containsHangul(candidate.Text) {
			nonKoreanChoices++
		}
	}
	return nonKoreanChoices == len(candidates)
}

func containsHangul(text string) bool {
	for _, r := range text {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}
