package augmentlesson

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

func TestSaveSelfAugmentLesson(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result, err := SaveSelfAugmentLesson(model.SelfAugmentLessonRequest{
		CandidateID: "reflexion-state-memory",
		Lesson:      "실패 교훈은 다음 cycle에서 재사용 가능해야 한다.",
		NextAction:  "다음 자가 증강 후보 선택 전에 저장된 lesson을 확인한다.",
		Source:      "unit-test",
		Severity:    "warning",
		StateKey:    "self-augment-lesson-test",
	}, Deps{})
	if err != nil {
		t.Fatalf("SaveSelfAugmentLesson: %v", err)
	}
	if !result.OK || result.Kind != model.SelfAugmentationLessonKind || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected lesson result: %+v", result)
	}
	state, err := core.StateRead("self-augment-lesson-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot model.SelfAugmentLessonStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved lesson snapshot: %v", err)
	}
	if snapshot.Kind != model.SelfAugmentationLessonKind || snapshot.CandidateID != "reflexion-state-memory" || snapshot.NextAction == "" {
		t.Fatalf("unexpected lesson snapshot: %+v", snapshot)
	}
}

func TestSaveSelfAugmentLessonPrunesOldLessonRecords(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := core.StateWrite("self-augment-lesson-old", `{"kind":"self_augmentation_lesson"}`); err != nil {
		t.Fatalf("write old lesson: %v", err)
	}
	old, err := core.StateRead("self-augment-lesson-old")
	if err != nil {
		t.Fatalf("read old lesson: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	if _, err := core.WriteStateRecord(core.StateDir(), "self-augment-lesson-old", old.Record); err != nil {
		t.Fatalf("rewrite old lesson: %v", err)
	}

	if _, err := SaveSelfAugmentLesson(model.SelfAugmentLessonRequest{
		CandidateID: "candidate-one",
		Lesson:      "old lessons should not grow forever",
		NextAction:  "keep only recent lesson state",
		Severity:    "error",
	}, Deps{}); err != nil {
		t.Fatalf("SaveSelfAugmentLesson: %v", err)
	}

	if _, err := core.StateRead("self-augment-lesson-old"); err == nil {
		t.Fatalf("old lesson record should be pruned")
	}
}

func TestSaveSelfAugmentLessonRejectsMissingRequiredFields(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	if result, err := SaveSelfAugmentLesson(model.SelfAugmentLessonRequest{CandidateID: "candidate-one"}, Deps{}); err == nil || !strings.Contains(err.Error(), "lesson is required") || result.OK {
		t.Fatalf("expected missing lesson error, result=%#v err=%v", result, err)
	}
	if result, err := SaveSelfAugmentLesson(model.SelfAugmentLessonRequest{CandidateID: "candidate-one", Lesson: "learned"}, Deps{}); err == nil || !strings.Contains(err.Error(), "next-action is required") || result.OK {
		t.Fatalf("expected missing next-action error, result=%#v err=%v", result, err)
	}
}

func TestStateKeySlugNormalizesUnsafeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims and lowercases", in: "  Ship This Lesson  ", want: "ship-this-lesson"},
		{name: "collapses punctuation", in: "A/B:C___D", want: "a-b-c-d"},
		{name: "fallback for empty slug", in: "!!!", want: "lesson"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateKeySlug(tt.in)
			if got != tt.want {
				t.Fatalf("StateKeySlug(%q)=%q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, "_") {
				t.Fatalf("StateKeySlug(%q) kept underscore in %q", tt.in, got)
			}
		})
	}
}
