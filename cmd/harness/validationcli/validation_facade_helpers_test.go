package validationcli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statecontract "agent-harness/internal/contract/state"
)

func TestValidationDependencyHelpers(t *testing.T) {
	if step := runCommandStep(".", "true", time.Second, "", "true"); !step.OK {
		t.Fatalf("runCommandStep true failed: %#v", step)
	}
	if step := runCommandStepEnv(".", "true env", time.Second, "", []string{"A=B"}, "true"); !step.OK {
		t.Fatalf("runCommandStepEnv true failed: %#v", step)
	}
	if step := runCommandStepEnvWithBudget(".", "true env budget", time.Second, "", nil, 10, "true"); !step.OK {
		t.Fatalf("runCommandStepEnvWithBudget true failed: %#v", step)
	}
	started := time.Now()
	failed := StepResult{Label: "child", OK: false, Error: "boom"}
	if combined := combineFailedStep("parent", started, failed, []string{"stdout"}, []string{"cmd"}); combined.OK || !strings.Contains(combined.Error, "child") {
		t.Fatalf("combineFailedStep = %#v", combined)
	}
	if assertion := assertionStep("assert", started, []string{"bad"}); assertion.OK || assertion.Error == "" {
		t.Fatalf("assertionStep = %#v", assertion)
	}
	if assertion := assertionStepWithOutput("assert", started, []string{"bad"}, []string{"out"}, []string{"cmd"}); assertion.OK || assertion.Stdout == "" {
		t.Fatalf("assertionStepWithOutput = %#v", assertion)
	}
	if failed := failedStep("failed", errors.New("boom")); failed.OK || failed.Error != "boom" {
		t.Fatalf("failedStep = %#v", failed)
	}
	dir := t.TempDir()
	snapshot := SelfAugmentStateSnapshot{Kind: selfVerificationSummaryKind, OK: true}
	if err := writeSelfAugmentSnapshotRecord(dir, "snapshot", snapshot); err != nil {
		t.Fatalf("writeSelfAugmentSnapshotRecord: %v", err)
	}
	// The snapshot lands in the directory's state database; the exists helper
	// itself is exercised against a real file.
	if !exists(filepath.Join(dir, "harness.db")) || exists(filepath.Join(dir, "missing.json")) {
		t.Fatal("exists helper mismatch")
	}
	if out, truncated, bytes := tailWithBudget("abcdef", 3); !truncated || bytes != 6 || out == "" {
		t.Fatalf("tailWithBudget = out=%q truncated=%v bytes=%d", out, truncated, bytes)
	}
	if !containsString([]string{"a", "b"}, "b") || containsString([]string{"a"}, "z") {
		t.Fatal("containsString mismatch")
	}
	if lines := splitLines("a\nb\n"); len(lines) != 2 || lines[1] != "b" {
		t.Fatalf("splitLines = %#v", lines)
	}
	if splitLines(" \n ") != nil {
		t.Fatal("blank splitLines should return nil")
	}
}

func TestValidationCandidateExportFacadeWithDeps(t *testing.T) {
	key := "self-verify-candidates-123"
	exportResult := validCandidateExportResult(key)
	snapshot := SelfVerificationCandidateExportStateSnapshot{
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		OK:                    true,
		CandidateCount:        exportResult.CandidateCount,
		SatisfiedCandidateIDs: exportResult.SatisfiedCandidateIDs,
		Candidates:            exportResult.Candidates,
	}
	if errs := CandidateExportValidationErrors(key, exportResult, snapshot); len(errs) != 0 {
		t.Fatalf("valid export errors = %#v", errs)
	}
	if errs := CandidateExportValidationErrors(key, SelfVerificationCandidateExportResult{}, SelfVerificationCandidateExportStateSnapshot{}); len(errs) == 0 {
		t.Fatal("invalid export should report errors")
	}
	exportJSON, err := json.Marshal(exportResult)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	stateJSON, err := json.Marshal(statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: key, Content: string(snapshotJSON), Bytes: len(snapshotJSON)}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	step := ValidateSelfVerifyCandidateExportWithDeps("agent-harness", ".", 123, CandidateExportValidationDeps{
		MakeTempState: func(seed int64) (string, error) {
			return t.TempDir(), nil
		},
		RemoveAll: func(path string) error {
			return nil
		},
		Run: func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			calls++
			if label == "candidate export" {
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(exportJSON)}
			}
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(stateJSON)}
		},
	})
	if !step.OK || calls != 2 {
		t.Fatalf("ValidateSelfVerifyCandidateExportWithDeps = %#v calls=%d", step, calls)
	}
}

func TestValidationPureFacadeWrappers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(DetectClaudeMCPDuplicateWarnings(ClaudeMCPDuplicateWarningFixture())) == 0 {
		t.Fatal("duplicate warning fixture should be detected")
	}
	if ClaudeMCPDuplicateWarningFixture() == "" {
		t.Fatal("duplicate warning fixture should be non-empty")
	}
	if errs := LintMermaidBlocks("README.md", "```mermaid\ngraph TD\nA-->B\n```\n"); len(errs) != 0 {
		t.Fatalf("valid mermaid block errors = %#v", errs)
	}
	if hits := FindUnredactedSecretLike("token sk-1234567890abcdefghijklmnop"); len(hits) == 0 {
		t.Fatal("secret-like text should be detected")
	}
	if ContainsForbiddenLegacyOutsideRuntimePaths("ordinary text", root) {
		t.Fatal("unrelated path should not hit forbidden legacy names")
	}
	if len(ForbiddenNameHits(root)) != 0 {
		t.Fatal("empty temp root should not contain forbidden names")
	}
	if ValidateHarnessInvariants(root).OK {
		t.Fatal("empty temp root should not satisfy harness invariants")
	}
	if ValidateContractCheckSmoke("bin", root).OK {
		t.Fatal("missing binary should not satisfy contract check smoke")
	}
	if ValidateWorkerLifecycleSmoke("bin", root, 1).OK {
		t.Fatal("missing binary should not satisfy worker lifecycle smoke")
	}
}

func validCandidateExportResult(key string) SelfVerificationCandidateExportResult {
	candidates := make([]SelfVerificationCandidate, 10)
	for i := range candidates {
		candidates[i] = SelfVerificationCandidate{ID: "candidate-" + string(rune('a'+i)), Status: selfAugmentCandidateStatusSatisfied}
	}
	satisfied := []string{
		"completion-evidence-audit",
		"self-verify-candidate-export",
		"self-verify-step-budget-baseline",
		"self-verify-install-dry-run-smoke",
	}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		CandidateCount:        len(candidates),
		SatisfiedCandidateIDs: satisfied,
		Candidates:            candidates,
		StateCheckpoint:       &SelfAugmentStateCheckpoint{OK: true, Key: key},
	}
}
