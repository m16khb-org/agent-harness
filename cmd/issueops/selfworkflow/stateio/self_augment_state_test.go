package stateio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"issueops/cmd/issueops/selfworkflow/augmentplan"
	"issueops/cmd/issueops/selfworkflow/model"
	statestore "issueops/internal/adapter/outbound/state"
)

func TestSaveSelfAugmentPlan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", "..", "..", ".."))
	result := augmentplan.Plan(model.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}, root, "test")
	if err := SaveSelfAugmentPlan(&result, "self-augment-plan-test"); err != nil {
		t.Fatalf("SaveSelfAugmentPlan: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing plan checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-augment-plan-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-augment-plan-test.json") {
		t.Fatalf("unexpected plan checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := statestore.StateRead("self-augment-plan-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot model.SelfAugmentPlanStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved plan snapshot: %v", err)
	}
	if snapshot.Kind != model.SelfAugmentationPlanKind || snapshot.LoopKind != "self_augmentation" {
		t.Fatalf("unexpected saved plan snapshot: %+v", snapshot)
	}
	if snapshot.CandidateCount < 10 || len(snapshot.SatisfiedCandidateIDs) == 0 {
		t.Fatalf("saved plan did not preserve candidate memory: %+v", snapshot)
	}
}

func TestSaveSelfAugmentPlanRejectsInvalidStateKey(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	result := augmentplan.Plan(model.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 99}, ".", "test")
	if err := SaveSelfAugmentPlan(&result, "!bad-key"); err == nil {
		t.Fatal("expected self-augment plan save to reject invalid state key")
	}
	if result.StateCheckpoint == nil || result.StateCheckpoint.OK || result.StateCheckpoint.Error == "" {
		t.Fatalf("unexpected plan checkpoint after invalid save: %#v", result.StateCheckpoint)
	}
}
