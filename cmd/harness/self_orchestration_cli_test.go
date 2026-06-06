package main

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunSelfAugmentSavesPlanStateAsJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	out := captureStatusVerifyStdout(t, func() error {
		return runSelfAugment([]string{"--cycles", "1", "--target-score", "99", "--save-state", "--state-key", "augment-plan", "--json"})
	})

	var result SelfAugmentPlanResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode self-augment JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Cycles != 1 || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected self-augment result: %#v", result)
	}
	assertStateRecordContains(t, "augment-plan", selfAugmentationPlanKind)
}

func TestRunSelfAugmentLessonSavesStateAndUsesTextDefaults(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	out := captureStatusVerifyStdout(t, func() error {
		return runSelfAugment([]string{
			"lesson",
			"--candidate", "candidate-one",
			"--lesson", "Keep coverage slices isolated",
			"--next-action", "Add the next characterization test",
			"--state-key", "lesson-one",
		})
	})

	if !strings.Contains(out, "self-augment lesson saved: candidate=candidate-one key=lesson-one") {
		t.Fatalf("unexpected lesson text output:\n%s", out)
	}
	assertStateRecordContains(t, "lesson-one", selfAugmentationLessonKind)
	assertStateRecordContains(t, "lesson-one", "Keep coverage slices isolated")
}

func TestRunSelfVerifyCandidatesSavesExportStateAsJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	out := captureStatusVerifyStdout(t, func() error {
		return runSelfVerify([]string{"candidates", "--save-state", "--state-key", "verify-candidates", "--json"})
	})

	var result SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode self-verify candidates JSON: %v\n%s", err, out)
	}
	if !result.OK || result.CandidateCount == 0 || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected self-verify candidates result: %#v", result)
	}
	assertStateRecordContains(t, "verify-candidates", selfVerificationCandidateExportKind)
}

func TestSelfOrchestrationStateSavesSurfaceInvalidKeys(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	plan := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 99})
	if err := saveSelfAugmentPlan(&plan, "!bad-key"); err == nil {
		t.Fatal("expected self-augment plan save to reject invalid state key")
	}
	if plan.StateCheckpoint == nil || plan.StateCheckpoint.OK || plan.StateCheckpoint.Error == "" {
		t.Fatalf("unexpected plan checkpoint after invalid save: %#v", plan.StateCheckpoint)
	}

	export := exportSelfVerificationCandidates()
	if err := saveSelfVerificationCandidateExport(&export, "!bad-key"); err == nil {
		t.Fatal("expected self-verify candidate export save to reject invalid state key")
	}
	if export.StateCheckpoint == nil || export.StateCheckpoint.OK || export.StateCheckpoint.Error == "" {
		t.Fatalf("unexpected export checkpoint after invalid save: %#v", export.StateCheckpoint)
	}
}

func TestSaveSelfAugmentLessonRejectsMissingRequiredFields(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	if result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{CandidateID: "candidate-one"}); err == nil || !strings.Contains(err.Error(), "lesson is required") || result.OK {
		t.Fatalf("expected missing lesson error, result=%#v err=%v", result, err)
	}
	if result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{CandidateID: "candidate-one", Lesson: "learned"}); err == nil || !strings.Contains(err.Error(), "next-action is required") || result.OK {
		t.Fatalf("expected missing next-action error, result=%#v err=%v", result, err)
	}
}

func TestSelfVerifyWithProgressRejectsZeroIterations(t *testing.T) {
	result, err := selfVerifyWithProgress(0, 100, 95, false, nil)
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 iteration") {
		t.Fatalf("expected zero-iteration error, got result=%#v err=%v", result, err)
	}
	if result.OK || result.Iterations != 0 || len(result.Summary.CoverageGaps) == 0 {
		t.Fatalf("unexpected zero-iteration result: %#v", result)
	}
}

func assertStateRecordContains(t *testing.T, key string, want string) {
	t.Helper()
	state, err := core.StateRead(key)
	if err != nil {
		t.Fatalf("read state %q: %v", key, err)
	}
	if !strings.Contains(state.Record.Content, want) {
		t.Fatalf("state %q content does not contain %q:\n%s", key, want, state.Record.Content)
	}
}
