package selfworkflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

func TestSaveSelfAugmentLesson(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{
		CandidateID: "reflexion-state-memory",
		Lesson:      "실패 교훈은 다음 cycle에서 재사용 가능해야 한다.",
		NextAction:  "다음 자가 증강 후보 선택 전에 저장된 lesson을 확인한다.",
		Source:      "unit-test",
		Severity:    "warning",
		StateKey:    "self-augment-lesson-test",
	})
	if err != nil {
		t.Fatalf("saveSelfAugmentLesson: %v", err)
	}
	if !result.OK || result.Kind != selfAugmentationLessonKind || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected lesson result: %+v", result)
	}
	state, err := core.StateRead("self-augment-lesson-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentLessonStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved lesson snapshot: %v", err)
	}
	if snapshot.Kind != selfAugmentationLessonKind || snapshot.CandidateID != "reflexion-state-memory" || snapshot.NextAction == "" {
		t.Fatalf("unexpected lesson snapshot: %+v", snapshot)
	}
}

func TestSaveSelfAugmentPlan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	previousHarnessRoot := HarnessRoot
	HarnessRoot = func() string {
		return filepath.Clean(filepath.Join(cwd, "..", "..", ".."))
	}
	defer func() {
		HarnessRoot = previousHarnessRoot
	}()
	result := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	if err := saveSelfAugmentPlan(&result, "self-augment-plan-test"); err != nil {
		t.Fatalf("saveSelfAugmentPlan: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing plan checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-augment-plan-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-augment-plan-test.json") {
		t.Fatalf("unexpected plan checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-augment-plan-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentPlanStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved plan snapshot: %v", err)
	}
	if snapshot.Kind != selfAugmentationPlanKind || snapshot.LoopKind != "self_augmentation" {
		t.Fatalf("unexpected saved plan snapshot: %+v", snapshot)
	}
	if snapshot.CandidateCount < 10 || len(snapshot.SatisfiedCandidateIDs) == 0 {
		t.Fatalf("saved plan did not preserve candidate memory: %+v", snapshot)
	}
}
