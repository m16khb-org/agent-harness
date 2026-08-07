package augmentplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
	statestore "agent-harness/internal/adapter/outbound/state"
)

const (
	selfAugmentLessonKeyPrefix = "self-augment-lesson-"
	// Reflexion lessons are advisory signals, not judgements: repeated severe
	// failures demote a candidate's curriculum rank so the planner rotates to
	// the next best option, but they never auto-fail or remove the candidate.
	lessonPenaltyThreshold = 2
	lessonPenaltyPerSevere = 15.0
	recentLessonWindow     = 30 * 24 * time.Hour
)

func severeLessonSeverity(severity string) bool {
	// "error" is the severe tier of the CLI convention (info|warning|error);
	// major/critical/blocker cover free-text MCP callers.
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error", "major", "critical", "blocker":
		return true
	}
	return false
}

func severeLessonCounts() (map[string]int, []string) {
	return severeLessonCountsAt(time.Now().UTC())
}

func severeLessonCountsAt(now time.Time) (map[string]int, []string) {
	counts := map[string]int{}
	list, err := statestore.StateList()
	if err != nil {
		return counts, []string{"lesson scan: " + err.Error()}
	}
	for _, key := range list.Keys {
		if !strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			continue
		}
		record, err := statestore.StateRead(key)
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
		if !recentLesson(snapshot.GeneratedAt, now) {
			continue
		}
		counts[snapshot.CandidateID]++
	}
	return counts, nil
}

func recentLesson(generatedAt string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(generatedAt))
	if err != nil {
		return false
	}
	if t.After(now) {
		return true
	}
	return now.Sub(t) <= recentLessonWindow
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
