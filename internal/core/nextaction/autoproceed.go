package nextaction

import (
	"fmt"
	"regexp"
	"strings"
)

const defaultNextActionAutoProceedThreshold = 0.80

type NextActionAutoProceedResult struct {
	OK                     bool                  `json:"ok"`
	AutoProceed            bool                  `json:"auto_proceed"`
	AgentJudgementRequired bool                  `json:"agent_judgement_required"`
	Threshold              float64               `json:"threshold"`
	TopScore               float64               `json:"top_score"`
	SelectedIndex          int                   `json:"selected_index,omitempty"`
	SelectedText           string                `json:"selected_text,omitempty"`
	Reason                 string                `json:"reason"`
	BlockedByGuard         string                `json:"blocked_by_guard,omitempty"`
	Candidates             []NextActionCandidate `json:"candidates"`
}

func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	if threshold <= 0 {
		threshold = defaultNextActionAutoProceedThreshold
	}
	result := NextActionAutoProceedResult{OK: true, Threshold: threshold, Candidates: []NextActionCandidate{}}
	candidates := ParseCandidates(message)
	if len(candidates) < 2 {
		result.Reason = "no numbered next-action choices to evaluate"
		return result
	}
	result.Candidates = candidates

	recommended := SelectRecommendedCandidate(candidates)
	if recommended == nil {
		result.Reason = "no explicitly recommended next action; user decision required"
		return result
	}
	result.SelectedIndex = recommended.Index
	result.SelectedText = recommended.Text
	result.TopScore = recommended.Score
	if recommended.Destructive {
		result.BlockedByGuard = "destructive_action"
		result.Reason = "recommended action is destructive or irreversible; user decision required"
		return result
	}
	if recommended.Score >= threshold {
		result.AgentJudgementRequired = true
		result.Reason = fmt.Sprintf("recommended action scored %.2f >= threshold %.2f and is reversible; agent judgement required", recommended.Score, threshold)
		return result
	}
	result.Reason = fmt.Sprintf("recommended action scored %.2f below threshold %.2f; user decision required", recommended.Score, threshold)
	return result
}

func SelectRecommendedCandidate(candidates []NextActionCandidate) *NextActionCandidate {
	for i := range candidates {
		if candidates[i].Recommended {
			return &candidates[i]
		}
	}
	return nil
}

func scoreNextActionCandidate(candidate NextActionCandidate) float64 {
	if candidate.Destructive {
		return 0
	}
	score := 0.0
	if candidate.Recommended {
		score += 0.55
	}
	if nextActionHasForwardVerb(candidate.Text) {
		score += 0.30
	}
	score += 0.15
	if nextActionIsAmbiguous(candidate.Text) {
		score -= 0.45
	}
	return clampScore(score)
}

func clampScore(score float64) float64 {
	switch {
	case score < 0:
		return 0
	case score > 1:
		return 1
	default:
		return score
	}
}

var nextActionForwardVerbs = []string{
	"진행", "계속", "구현", "추가", "작성", "검증", "테스트", "빌드", "린트", "확인", "점검", "실행", "수정", "반영", "적용",
	"proceed", "continue", "implement", "add", "write", "verify", "test", "lint", "build", "check", "inspect", "dry-run", "dry run", "run", "apply", "fix", "update",
}

var nextActionAmbiguityNeedles = []string{
	"아마도", "아마", "확실치", "확실하지", "추정", "미확인", "검토 필요", "검토필요",
	"maybe", "perhaps", "might", "not sure", "unsure", "tbd", "???",
}

var nextActionDestructiveWordNeedles = []string{
	"delete", "remove", "drop", "truncate", "reset", "revert", "rollback", "rollout", "overwrite", "force", "discard", "purge", "close", "merge", "rebase",
	"push", "deploy", "release", "publish", "ship", "send", "email", "notify", "payment", "charge", "refund", "terraform", "kubectl", "prod", "production",
}

var nextActionDestructiveRawNeedles = []string{
	"삭제", "제거", "지우", "되돌리", "덮어", "초기화", "닫기", "강제", "병합", "머지",
	"푸시", "배포", "릴리즈", "게시", "전송", "결제", "환불", "운영", "프로덕션", "롤백", "롤아웃",
	"rm ", "--force", "-f ", "reset --hard", "push --force", "force-push", "terraform apply", "kubectl apply", "kubectl delete",
}

var nextActionDestructiveWordRe = regexp.MustCompile(`(?i)\b(?:` + strings.Join(nextActionDestructiveWordNeedles, "|") + `)\b`)

func nextActionIsRecommended(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(text, "추천") || strings.Contains(lower, "(recommended)") || strings.Contains(lower, "recommended")
}

func nextActionIsDestructive(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range nextActionDestructiveRawNeedles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return nextActionDestructiveWordRe.MatchString(text)
}

func nextActionIsAmbiguous(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range nextActionAmbiguityNeedles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func nextActionHasForwardVerb(text string) bool {
	lower := strings.ToLower(text)
	for _, verb := range nextActionForwardVerbs {
		if strings.Contains(lower, strings.ToLower(verb)) {
			return true
		}
	}
	return false
}
