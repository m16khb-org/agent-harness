package nextaction

import (
	"regexp"
	"strings"
)

// nextActionCandidateLineRe requires whitespace after the "N." / "N)" marker so
// decimal numbers ("1.5배") and version strings never parse as candidates.
var nextActionCandidateLineRe = regexp.MustCompile(`^([1-9])[.)]\s+(\S.*)$`)

func ParseCandidates(message string) []NextActionCandidate {
	candidates := parseNextActionCandidateFacts(message)
	for i := range candidates {
		candidates[i].Destructive = nextActionIsDestructive(candidates[i].Text)
		candidates[i].Score = scoreNextActionCandidate(candidates[i])
	}
	return candidates
}

// parseNextActionCandidateFacts extracts the numbered options of the LAST
// choices section in the message. The section opens at a 선택지/options header,
// tolerates preamble prose before the first option, and closes at the first
// non-empty non-option line after an option, so unrelated numbered lists
// later in the message never pollute the candidates.
func parseNextActionCandidateFacts(message string) []NextActionCandidate {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	candidates := []NextActionCandidate{}
	seen := map[int]bool{}
	inChoices := false
	for _, line := range lines {
		trimmed := normalizeNextActionLine(line)
		if nextActionSectionHeader(trimmed) {
			// Last section wins: a fresh header replaces earlier candidates.
			inChoices = true
			candidates = candidates[:0]
			seen = map[int]bool{}
			continue
		}
		if !inChoices || trimmed == "" {
			continue
		}
		match := nextActionCandidateLineRe.FindStringSubmatch(trimmed)
		if match == nil {
			if len(candidates) > 0 {
				inChoices = false
			}
			continue
		}
		index := int(match[1][0] - '0')
		if seen[index] {
			continue
		}
		seen[index] = true
		text := strings.TrimSpace(match[2])
		candidates = append(candidates, NextActionCandidate{
			Index:       index,
			Text:        text,
			Recommended: nextActionIsRecommended(text),
		})
	}
	return candidates
}

// candidatesFormWellFormedChoiceSet reports whether the candidates are exactly
// the three options {1,2,3} the stop-gate reason text demands.
func candidatesFormWellFormedChoiceSet(candidates []NextActionCandidate) bool {
	if len(candidates) != 3 {
		return false
	}
	seen := map[int]bool{}
	for _, candidate := range candidates {
		seen[candidate.Index] = true
	}
	return seen[1] && seen[2] && seen[3]
}

func countRecommendedCandidates(candidates []NextActionCandidate) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Recommended {
			count++
		}
	}
	return count
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
	trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
	trimmed = strings.TrimPrefix(trimmed, "**")
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "선택지:") ||
		strings.HasPrefix(lower, "options:") ||
		strings.HasPrefix(lower, "next actions:")
}
