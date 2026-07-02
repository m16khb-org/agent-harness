package augmentplan

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

const (
	selfAugmentLessonKeyPrefix = "self-augment-lesson-"
	// Reflexion lessons are advisory signals, not judgements: repeated severe
	// failures demote a candidate's curriculum rank so the planner rotates to
	// the next best option, but they never auto-fail or remove the candidate.
	lessonPenaltyThreshold = 2
	lessonPenaltyPerSevere = 15.0
)

func severeLessonSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "major", "critical", "blocker":
		return true
	}
	return false
}

func severeLessonCounts() (map[string]int, []string) {
	counts := map[string]int{}
	list, err := core.StateList()
	if err != nil {
		return counts, []string{"lesson scan: " + err.Error()}
	}
	for _, key := range list.Keys {
		if !strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			continue
		}
		record, err := core.StateRead(key)
		if err != nil {
			continue
		}
		var snapshot model.SelfAugmentLessonStateSnapshot
		if err := json.Unmarshal([]byte(record.Record.Content), &snapshot); err != nil {
			continue
		}
		if snapshot.Kind != model.SelfAugmentationLessonKind {
			continue
		}
		if snapshot.CandidateID == "" || !severeLessonSeverity(snapshot.Severity) {
			continue
		}
		counts[snapshot.CandidateID]++
	}
	return counts, nil
}

func applyLessonPenalties(candidates []model.SelfAugmentCandidate, severeCounts map[string]int) []string {
	warnings := []string{}
	for i := range candidates {
		if candidates[i].Status != augmentcatalog.SelfAugmentCandidateStatusOpen {
			continue
		}
		count := severeCounts[candidates[i].ID]
		if count < lessonPenaltyThreshold {
			continue
		}
		before := candidates[i].Score
		after := before - float64(count)*lessonPenaltyPerSevere
		if after < 0 {
			after = 0
		}
		candidates[i].Score = after
		warnings = append(warnings, fmt.Sprintf(
			"lesson penalty: candidate %q score %.1f -> %.1f after %d severe lessons (advisory demotion; candidate stays open)",
			candidates[i].ID, before, after, count))
	}
	return warnings
}
