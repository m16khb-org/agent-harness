package nextaction

import (
	"strings"
	"unicode"
)

func choiceQualityBlockReason(message string, candidates []NextActionCandidate) string {
	if !hasChoiceQualityEvidence(message) {
		return "Stop hook이 선택지 품질 증거 누락으로 차단했습니다. `선택지 품질 증거` 섹션에 context 확인, 추천 근거, 사용자 승인 경계를 포함하세요."
	}
	if choicesIgnoreConversationLanguage(message, candidates) {
		return "Stop hook이 대화 언어와 맞지 않는 선택지 때문에 차단했습니다. 한국어 대화에서는 선택지 본문을 한국어 선택지로 작성하세요."
	}
	if hasDuplicateChoiceMeaning(candidates) {
		return "Stop hook이 서로 다른 선택지가 아니어서 차단했습니다. 반복된 no-op 또는 같은 의미의 선택지 대신 실질적으로 서로 다른 행동 3개를 제시하세요."
	}
	if recommendedChoiceIsDestructive(candidates) {
		return "Stop hook이 추천 선택지가 파괴적이거나 외부 상태를 바꾸는 작업으로 보여 차단했습니다. 명시적 사용자 선택 없이 push/delete/deploy/merge 같은 작업을 추천하지 마세요."
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
		if !containsHangul(choiceLanguageText(candidate.Text)) {
			nonKoreanChoices++
		}
	}
	return nonKoreanChoices == len(candidates)
}

func choiceLanguageText(text string) string {
	text = strings.ReplaceAll(text, "(추천)", "")
	text = strings.ReplaceAll(text, "(recommended)", "")
	text = strings.ReplaceAll(text, "(Recommended)", "")
	return strings.TrimSpace(text)
}

func containsHangul(text string) bool {
	for _, r := range text {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}
