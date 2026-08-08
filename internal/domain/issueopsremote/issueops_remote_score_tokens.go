package remote

import (
	"strings"
	"unicode"
)

func issueOpsRemoteTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_')
	}) {
		token = strings.Trim(token, "-_")
		if len(token) < 3 || issueOpsRemoteStopWords[token] {
			continue
		}
		tokens[token] = true
	}
	return tokens
}

var issueOpsRemoteStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true, "that": true, "from": true,
	"문제": true, "현재": true, "근거": true, "완료": true, "기준": true, "검증": true, "비목표": true,
}

func issueOpsRemoteOverlap(left, right map[string]bool) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersect := 0
	for token := range left {
		if right[token] {
			intersect++
		}
	}
	return float64(intersect) / float64(min(len(left), len(right)))
}

func issueOpsRemoteLabelHeuristic(issueTokens map[string]bool, candidate IssueOpsRemoteLabelCandidate) float64 {
	name := strings.ToLower(strings.TrimSpace(candidate.Name))
	switch name {
	case "enhancement":
		if issueTokens["feature"] || issueTokens["request"] || issueTokens["개선"] || issueTokens["추가"] || issueTokens["지원"] {
			return 1
		}
	case "bug":
		if issueTokens["bug"] || issueTokens["defect"] || issueTokens["failure"] || issueTokens["오류"] || issueTokens["결함"] {
			return 1
		}
	case "documentation":
		if issueTokens["docs"] || issueTokens["documentation"] || issueTokens["문서"] || issueTokens["skill"] || issueTokens["prompt"] {
			return 0.75
		}
	}
	return 0
}
