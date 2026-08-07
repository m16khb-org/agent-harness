package stateio

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/adapter/core"
)

func TestNewSelfVerificationSummarySnapshotCopiesResultFields(t *testing.T) {
	generatedAt := time.Date(2026, 6, 6, 1, 2, 3, 4, time.UTC)
	result := selfVerificationSummaryResultForSaveTest()

	snapshot := NewSelfVerificationSummarySnapshot(result, generatedAt)

	if snapshot.SchemaVersion != 1 ||
		snapshot.Kind != model.SelfVerificationSummaryKind ||
		snapshot.LoopKind != result.LoopKind ||
		snapshot.KoreanName != result.KoreanName ||
		!snapshot.OK ||
		snapshot.Iterations != result.Iterations ||
		snapshot.BaseSeed != result.BaseSeed ||
		snapshot.TargetScore != result.TargetScore ||
		snapshot.ElapsedMS != result.ElapsedMS ||
		snapshot.HarnessRoot != result.HarnessRoot ||
		snapshot.GeneratedAt != generatedAt.Format(time.RFC3339Nano) ||
		snapshot.Summary.TotalSteps != result.Summary.TotalSteps {
		t.Fatalf("snapshot did not preserve result fields: %+v", snapshot)
	}
}

func TestSaveSelfVerificationSummaryWritesDefaultKeyAndRejectsInvalidKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result := selfVerificationSummaryResultForSaveTest()

	if err := SaveSelfVerificationSummary(&result, ""); err != nil {
		t.Fatalf("save default key: %v", err)
	}
	if result.StateCheckpoint == nil ||
		!result.StateCheckpoint.OK ||
		result.StateCheckpoint.Key != "self-verify-latest" ||
		result.StateCheckpoint.Path != filepath.Join(dir, "self-verify-latest.json") ||
		result.StateCheckpoint.Bytes == 0 {
		t.Fatalf("unexpected successful checkpoint: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-verify-latest")
	if err != nil {
		t.Fatalf("read saved summary: %v", err)
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("decode saved summary: %v\n%s", err, state.Record.Content)
	}
	if snapshot.Kind != model.SelfVerificationSummaryKind || snapshot.Summary.TotalSteps != result.Summary.TotalSteps {
		t.Fatalf("unexpected saved snapshot: %+v", snapshot)
	}

	failed := selfVerificationSummaryResultForSaveTest()
	err = SaveSelfVerificationSummary(&failed, "!bad-key")
	if err == nil || !strings.Contains(err.Error(), "invalid state key") {
		t.Fatalf("expected invalid key error, got %v", err)
	}
	if failed.StateCheckpoint == nil ||
		failed.StateCheckpoint.OK ||
		failed.StateCheckpoint.Key != "!bad-key" ||
		failed.StateCheckpoint.StateDir != dir ||
		failed.StateCheckpoint.Error == "" {
		t.Fatalf("unexpected failed checkpoint: %+v", failed.StateCheckpoint)
	}
}

func TestSaveSelfAugmentSummary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result := model.SelfAugmentResult{
		OK:          true,
		Iterations:  10,
		BaseSeed:    300,
		ElapsedMS:   1234,
		HarnessRoot: "/tmp/harness",
		Summary: model.SelfAugmentSummary{
			TotalRuns:   1,
			TotalSteps:  1,
			PassedSteps: 1,
			StepLabels:  []string{"go test"},
		},
	}
	if err := SaveSelfAugmentSummary(&result, "self-verify-test"); err != nil {
		t.Fatalf("SaveSelfAugmentSummary: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing state checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-verify-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-verify-test.json") {
		t.Fatalf("unexpected checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-verify-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved snapshot: %v", err)
	}
	if snapshot.Kind != model.SelfVerificationSummaryKind || !snapshot.OK || snapshot.Summary.TotalSteps != 1 || snapshot.Summary.PassedSteps != 1 {
		t.Fatalf("unexpected saved snapshot: %+v", snapshot)
	}
}

func selfVerificationSummaryResultForSaveTest() SelfAugmentResult {
	return SelfAugmentResult{
		OK:          true,
		LoopKind:    "self_verification",
		KoreanName:  model.SelfVerificationKoreanName,
		Iterations:  10,
		BaseSeed:    123,
		TargetScore: 95,
		ElapsedMS:   456,
		HarnessRoot: "/tmp/harness",
		Summary: model.SelfAugmentSummary{
			TotalRuns:   10,
			TotalSteps:  20,
			PassedSteps: 20,
			StepLabels:  []string{"go test"},
		},
	}
}
