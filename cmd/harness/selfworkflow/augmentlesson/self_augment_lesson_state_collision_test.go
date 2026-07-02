package augmentlesson

import (
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestSaveSelfAugmentLessonSameSecondKeysDoNotCollide(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	deps := Deps{HarnessRoot: func() string { return t.TempDir() }}

	req := model.SelfAugmentLessonRequest{
		CandidateID: "collision-cand",
		Lesson:      "attempt failed",
		NextAction:  "retry",
		Severity:    "error",
	}

	first, err := SaveSelfAugmentLesson(req, deps)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := SaveSelfAugmentLesson(req, deps)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	firstKey := first.StateCheckpoint.Key
	secondKey := second.StateCheckpoint.Key
	if firstKey == secondKey {
		t.Errorf("two lessons saved back-to-back share state key %q; the second overwrites the first", firstKey)
	}
}
