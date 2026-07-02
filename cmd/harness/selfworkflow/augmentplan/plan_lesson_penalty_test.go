package augmentplan

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/augmentcatalog"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

func TestApplyLessonPenaltiesDemotesRepeatedSevereLessons(t *testing.T) {
	candidates := []model.SelfAugmentCandidate{
		{ID: "repeat-fail", Status: augmentcatalog.SelfAugmentCandidateStatusOpen, Score: 80},
		{ID: "single-fail", Status: augmentcatalog.SelfAugmentCandidateStatusOpen, Score: 70},
		{ID: "already-done", Status: augmentcatalog.SelfAugmentCandidateStatusSatisfied, Score: 0},
		{ID: "floor-case", Status: augmentcatalog.SelfAugmentCandidateStatusOpen, Score: 20},
	}
	counts := map[string]int{
		"repeat-fail":  2,
		"single-fail":  1,
		"already-done": 3,
		"floor-case":   3,
	}

	warnings := applyLessonPenalties(candidates, counts)

	if got := candidates[0].Score; got != 50 {
		t.Errorf("repeat-fail score = %v, want 50 (80 - 2*15)", got)
	}
	if got := candidates[1].Score; got != 70 {
		t.Errorf("single-fail score = %v, want unchanged 70 (below threshold)", got)
	}
	if got := candidates[2].Score; got != 0 {
		t.Errorf("already-done score = %v, want unchanged 0 (non-open skipped)", got)
	}
	if got := candidates[3].Score; got != 0 {
		t.Errorf("floor-case score = %v, want floored at 0", got)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want exactly 2 (repeat-fail, floor-case)", warnings)
	}
	for _, w := range warnings {
		if !strings.Contains(w, "lesson penalty: candidate") {
			t.Errorf("warning %q missing lesson penalty prefix", w)
		}
	}
}

func TestSevereLessonCountsCountsOnlySevereLessonSnapshots(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	writeLesson := func(t *testing.T, key, candidateID, severity string) {
		t.Helper()
		snapshot := model.SelfAugmentLessonStateSnapshot{
			SchemaVersion: 1,
			Kind:          model.SelfAugmentationLessonKind,
			LoopKind:      "self_augmentation",
			OK:            true,
			CandidateID:   candidateID,
			Lesson:        "lesson for " + candidateID,
			NextAction:    "retry",
			Severity:      severity,
		}
		b, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("marshal lesson: %v", err)
		}
		if _, err := core.StateWrite(key, string(b)); err != nil {
			t.Fatalf("write lesson state %s: %v", key, err)
		}
	}

	writeLesson(t, "self-augment-lesson-cand-a-1", "cand-a", "error")
	writeLesson(t, "self-augment-lesson-cand-a-2", "cand-a", "MAJOR")
	writeLesson(t, "self-augment-lesson-cand-a-3", "cand-a", "info")
	writeLesson(t, "self-augment-lesson-cand-a-4", "cand-a", "warning")
	writeLesson(t, "self-augment-lesson-cand-b-1", "cand-b", "critical")
	if _, err := core.StateWrite("self-augment-lesson-broken-1", "{not json"); err != nil {
		t.Fatalf("write malformed lesson: %v", err)
	}
	if _, err := core.StateWrite("unrelated-key", `{"kind":"other"}`); err != nil {
		t.Fatalf("write unrelated state: %v", err)
	}

	counts, warnings := severeLessonCounts()

	if got := counts["cand-a"]; got != 2 {
		t.Errorf("cand-a severe count = %d, want 2 (info/warning lessons excluded, CLI-convention error included)", got)
	}
	if got := counts["cand-b"]; got != 1 {
		t.Errorf("cand-b severe count = %d, want 1", got)
	}
	if len(counts) != 2 {
		t.Errorf("counts = %v, want only cand-a and cand-b", counts)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for a readable state dir", warnings)
	}
}

func TestPlanAppliesLessonPenaltyAndRotatesSelection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	req := model.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}

	baseline := Plan(req, root, "test")
	if baseline.SelectedCandidate == nil {
		t.Fatal("baseline plan selected no candidate")
	}
	topID := baseline.SelectedCandidate.ID

	for i := range 4 {
		snapshot := model.SelfAugmentLessonStateSnapshot{
			SchemaVersion: 1,
			Kind:          model.SelfAugmentationLessonKind,
			LoopKind:      "self_augmentation",
			OK:            true,
			CandidateID:   topID,
			Lesson:        "implementation attempt failed",
			NextAction:    "redesign",
			Severity:      "major",
		}
		b, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("marshal lesson: %v", err)
		}
		key := fmt.Sprintf("self-augment-lesson-top-%d", i)
		if _, err := core.StateWrite(key, string(b)); err != nil {
			t.Fatalf("write lesson state: %v", err)
		}
	}

	demoted := Plan(req, root, "test")
	if demoted.SelectedCandidate == nil {
		t.Fatal("demoted plan selected no candidate")
	}
	if demoted.SelectedCandidate.ID == topID {
		t.Errorf("selected candidate still %q after 4 severe lessons; want curriculum rotation", topID)
	}
	found := false
	for _, w := range demoted.Warnings {
		if strings.Contains(w, "lesson penalty: candidate") && strings.Contains(w, topID) {
			found = true
		}
	}
	if !found {
		t.Errorf("plan warnings %v missing lesson penalty entry for %q", demoted.Warnings, topID)
	}
}
