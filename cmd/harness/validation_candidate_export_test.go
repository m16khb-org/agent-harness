package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
)

func TestValidateSelfVerifyCandidateExportWrapperUsesExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeCandidateExportFakeBinary(t, root, "self-verify-candidates-202")

	step := validateSelfVerifyCandidateExport(binary, root, 202)
	if !step.OK || !strings.Contains(step.Command, "self-verify candidates") || !strings.Contains(step.Command, "state read") {
		t.Fatalf("expected wrapper success, got %+v", step)
	}
}

func TestValidateCandidateExportWithDepsCoversSuccessAndCommandFailure(t *testing.T) {
	root := t.TempDir()
	tempState := t.TempDir()
	key := "self-verify-candidates-77"
	export := validCandidateExportResult(key)
	snapshot := validCandidateExportSnapshot(export)
	calls := []string{}
	deps := candidateExportValidationDeps{
		makeTempState: func(seed int64) (string, error) {
			if seed != 77 {
				t.Fatalf("unexpected seed: %d", seed)
			}
			return tempState, nil
		},
		removeAll: func(path string) error {
			if path != tempState {
				t.Fatalf("unexpected cleanup path: %s", path)
			}
			return nil
		},
		run: func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			calls = append(calls, label+":"+strings.Join(append([]string{name}, args...), " "))
			if dir != root || timeout != 30*time.Second || stdin != "" || len(env) != 1 || env[0] != "HARNESS_STATE_DIR="+tempState {
				t.Fatalf("unexpected command envelope: dir=%q label=%q timeout=%s stdin=%q env=%v", dir, label, timeout, stdin, env)
			}
			switch label {
			case "candidate export":
				return StepResult{Label: label, Command: calls[len(calls)-1], OK: true, Stdout: mustMarshalCandidateExportTest(t, export)}
			case "candidate export state read":
				return StepResult{Label: label, Command: calls[len(calls)-1], OK: true, Stdout: mustMarshalCandidateExportTest(t, core.StateResult{OK: true, Record: core.StateRecord{Content: mustMarshalCandidateExportTest(t, snapshot)}})}
			default:
				t.Fatalf("unexpected label: %s", label)
			}
			return StepResult{}
		},
	}
	step := validateSelfVerifyCandidateExportWithDeps("bin/agent-harness", root, 77, deps)
	if !step.OK || !strings.Contains(step.Command, "candidate export:bin/agent-harness self-verify candidates") || !strings.Contains(step.Command, "candidate export state read:bin/agent-harness state read") || len(calls) != 2 {
		t.Fatalf("unexpected candidate export success: step=%+v calls=%v", step, calls)
	}

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "candidate export", Command: "export", OK: false, Error: "boom"}
	}
	failed := validateSelfVerifyCandidateExportWithDeps("bin", root, 77, deps)
	if failed.OK || !strings.Contains(failed.Error, "candidate export: boom") {
		t.Fatalf("unexpected command failure: %+v", failed)
	}
}

func TestValidateCandidateExportWithDepsCoversParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	tempState := t.TempDir()
	key := "self-verify-candidates-5"
	export := validCandidateExportResult(key)
	deps := candidateExportValidationDeps{
		makeTempState: func(int64) (string, error) { return tempState, nil },
		removeAll:     func(string) error { return nil },
	}

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "candidate export", OK: true, Stdout: "{"}
	}
	badExportJSON := validateSelfVerifyCandidateExportWithDeps("bin", root, 5, deps)
	if badExportJSON.OK || badExportJSON.Error == "" {
		t.Fatalf("expected export JSON failure, got %+v", badExportJSON)
	}

	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, _ ...string) StepResult {
		if label == "candidate export" {
			return StepResult{Label: label, Command: "export", OK: true, Stdout: mustMarshalCandidateExportTest(t, export)}
		}
		return StepResult{Label: label, Command: "read", OK: true, Stdout: "{"}
	}
	badReadJSON := validateSelfVerifyCandidateExportWithDeps("bin", root, 5, deps)
	if badReadJSON.OK || badReadJSON.Error == "" {
		t.Fatalf("expected read JSON failure, got %+v", badReadJSON)
	}

	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, _ ...string) StepResult {
		if label == "candidate export" {
			return StepResult{Label: label, Command: "export", OK: true, Stdout: mustMarshalCandidateExportTest(t, export)}
		}
		return StepResult{Label: label, Command: "read", OK: true, Stdout: mustMarshalCandidateExportTest(t, core.StateResult{OK: true, Record: core.StateRecord{Content: "{"}})}
	}
	badSnapshot := validateSelfVerifyCandidateExportWithDeps("bin", root, 5, deps)
	if badSnapshot.OK || !strings.Contains(badSnapshot.Error, "candidate export state snapshot parse") {
		t.Fatalf("expected snapshot JSON failure, got %+v", badSnapshot)
	}

	invalidExport := export
	invalidExport.CandidateCount = 1
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, _ ...string) StepResult {
		if label == "candidate export" {
			return StepResult{Label: label, Command: "export", OK: true, Stdout: mustMarshalCandidateExportTest(t, invalidExport)}
		}
		return StepResult{Label: label, Command: "read", OK: true, Stdout: mustMarshalCandidateExportTest(t, core.StateResult{OK: true, Record: core.StateRecord{Content: mustMarshalCandidateExportTest(t, validCandidateExportSnapshot(export))}})}
	}
	contractFailure := validateSelfVerifyCandidateExportWithDeps("bin", root, 5, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "candidate export did not include the candidate curriculum") {
		t.Fatalf("expected contract failure, got %+v", contractFailure)
	}

	deps.makeTempState = func(int64) (string, error) { return "", errors.New("temp fail") }
	tempFailure := validateSelfVerifyCandidateExportWithDeps("bin", root, 5, deps)
	if tempFailure.OK || tempFailure.Error != "temp fail" {
		t.Fatalf("unexpected temp failure: %+v", tempFailure)
	}
}

func validCandidateExportResult(key string) SelfVerificationCandidateExportResult {
	candidates := make([]SelfVerificationCandidate, 10)
	for i := range candidates {
		candidates[i] = SelfVerificationCandidate{ID: string(rune('a' + i)), Status: selfAugmentCandidateStatusSatisfied}
	}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		CandidateCount:        len(candidates),
		SatisfiedCandidateIDs: []string{"completion-evidence-audit", "self-verify-candidate-export", "self-verify-step-budget-baseline", "self-verify-install-dry-run-smoke"},
		Candidates:            candidates,
		StateCheckpoint:       &SelfAugmentStateCheckpoint{OK: true, Key: key},
	}
}

func validCandidateExportSnapshot(result SelfVerificationCandidateExportResult) SelfVerificationCandidateExportStateSnapshot {
	return SelfVerificationCandidateExportStateSnapshot{
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              result.LoopKind,
		OK:                    true,
		CandidateCount:        result.CandidateCount,
		SatisfiedCandidateIDs: result.SatisfiedCandidateIDs,
		Candidates:            result.Candidates,
	}
}

func mustMarshalCandidateExportTest(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeCandidateExportFakeBinary(t *testing.T, dir, key string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	export := validCandidateExportResult(key)
	snapshot := validCandidateExportSnapshot(export)
	state := core.StateResult{OK: true, Record: core.StateRecord{Content: mustMarshalCandidateExportTest(t, snapshot)}}
	body := "#!/bin/sh\nset -eu\ncase \"$*\" in\n" +
		"  \"self-verify candidates --save-state --state-key " + key + " --json\") printf '%s\\n' '" + mustMarshalCandidateExportTest(t, export) + "' ;;\n" +
		"  \"state read --key " + key + " --json\") printf '%s\\n' '" + mustMarshalCandidateExportTest(t, state) + "' ;;\n" +
		"  *) echo \"unexpected fake harness args: $*\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
