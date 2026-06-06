package nextaction

import (
	"fmt"
	"strings"
)

func ParseCandidates(message string) []NextActionCandidate {
	candidates := parseNextActionCandidateFacts(message)
	for i := range candidates {
		candidates[i].Destructive = nextActionIsDestructive(candidates[i].Text)
		candidates[i].Score = scoreNextActionCandidate(candidates[i])
	}
	return candidates
}

func parseNextActionCandidateFacts(message string) []NextActionCandidate {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	candidates := []NextActionCandidate{}
	inChoices := false
	for _, line := range lines {
		trimmed := normalizeNextActionLine(line)
		if nextActionSectionHeader(trimmed) {
			inChoices = true
			continue
		}
		if !inChoices {
			continue
		}
		for i := 1; i <= 9; i++ {
			prefixDot := fmt.Sprintf("%d.", i)
			prefixParen := fmt.Sprintf("%d)", i)
			var rest string
			switch {
			case strings.HasPrefix(trimmed, prefixDot):
				rest = strings.TrimSpace(strings.TrimPrefix(trimmed, prefixDot))
			case strings.HasPrefix(trimmed, prefixParen):
				rest = strings.TrimSpace(strings.TrimPrefix(trimmed, prefixParen))
			default:
				continue
			}
			if rest == "" {
				continue
			}
			candidate := NextActionCandidate{
				Index:       i,
				Text:        rest,
				Recommended: nextActionIsRecommended(rest),
			}
			candidates = append(candidates, candidate)
			break
		}
	}
	return candidates
}

func hasNumberedNextActions(message string) bool {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	seen := map[int]bool{}
	inChoices := false
	for _, line := range lines {
		trimmed := normalizeNextActionLine(line)
		if nextActionSectionHeader(trimmed) {
			inChoices = true
			continue
		}
		if !inChoices {
			continue
		}
		if len(trimmed) < 2 {
			continue
		}
		for i := 1; i <= 3; i++ {
			prefix := fmt.Sprintf("%d.", i)
			if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(trimmed, fmt.Sprintf("%d)", i)) {
				seen[i] = true
			}
		}
	}
	return seen[1] && seen[2] && seen[3]
}

func hasExactlyOneRecommendedNextAction(message string) bool {
	count := 0
	for _, candidate := range parseNextActionCandidateFacts(message) {
		if candidate.Index < 1 || candidate.Index > 3 {
			continue
		}
		if candidate.Recommended {
			count++
		}
	}
	return count == 1
}

func normalizeNextActionLine(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "- ")
	trimmed = strings.TrimPrefix(trimmed, "* ")
	trimmed = strings.TrimPrefix(trimmed, "+ ")
	return strings.TrimSpace(trimmed)
}

func nextActionSectionHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "선택지:") ||
		strings.HasPrefix(lower, "options:") ||
		strings.HasPrefix(lower, "next actions:")
}
