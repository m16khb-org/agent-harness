package stateroundtrip

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
)

func TestValidateStateRoundtripWithDepsCoversSuccessAndSetupFailure(t *testing.T) {
	root := t.TempDir()
	tempState := t.TempDir()
	calls := []string{}
	writes := []string{}
	deps := stateRoundtripValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempState, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(path string, _ []byte, _ os.FileMode) error {
			writes = append(writes, path)
			return nil
		},
		stateRead: func(key string) (statecontract.StateResult, error) {
			if strings.HasSuffix(key, "-promoted-baseline") {
				return statecontract.StateResult{}, errors.New("missing")
			}
			return statecontract.StateResult{OK: true}, nil
		},
		writeSnapshot: func(string, string, SelfAugmentStateSnapshot) error { return nil },
		run: func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
			calls = append(calls, label+":"+strings.Join(command, " "))
			return stateRoundtripStep(t, label, command, validStateRoundtripPayload(t, label, 123))
		},
	}

	step := validateStateRoundtripWithDeps("bin/issueops", root, 123, deps)
	if !step.OK || step.Label != "state roundtrip" || len(calls) != 17 || !strings.Contains(step.Command, "state write") || !strings.Contains(step.Command, "self-verify history") {
		t.Fatalf("unexpected success step: %#v calls=%v", step, calls)
	}
	if !strings.Contains(step.Command, "self-verify promote --from-key self-verify-123-compare-candidate --baseline-key self-verify-123-promoted-baseline --allow-failed-source --confirm --json") {
		t.Fatalf("confirmed promote of failed fixture must pass explicit override, command=%q", step.Command)
	}
	if len(writes) != 0 {
		t.Fatalf("expected no raw state fixture writes, got %v", writes)
	}

	deps.mkdirTemp = func(_, _ string) (string, error) { return "", errors.New("no temp") }
	failed := validateStateRoundtripWithDeps("bin", root, 123, deps)
	if failed.OK || failed.Label != "state roundtrip" || !strings.Contains(failed.Error, "no temp") {
		t.Fatalf("expected setup failure, got %#v", failed)
	}
}

func TestValidateStateRoundtripWithDepsCoversCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	deps := stateRoundtripTestDeps(t, 456)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		if label == "state read" {
			return StepResult{Label: label, Command: strings.Join(command, " "), OK: false, Error: "read failed"}
		}
		return stateRoundtripStep(t, label, command, validStateRoundtripPayload(t, label, 456))
	}
	commandFailure := validateStateRoundtripWithDeps("bin", root, 456, deps)
	if commandFailure.OK || !strings.Contains(commandFailure.Error, "read failed") || !strings.Contains(commandFailure.Command, "state read") {
		t.Fatalf("expected command failure, got %#v", commandFailure)
	}

	deps = stateRoundtripTestDeps(t, 456)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		if label == "state list" {
			return StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: "{bad json"}
		}
		return stateRoundtripStep(t, label, command, validStateRoundtripPayload(t, label, 456))
	}
	parseFailure := validateStateRoundtripWithDeps("bin", root, 456, deps)
	if parseFailure.OK || !strings.Contains(parseFailure.Error, "invalid character") {
		t.Fatalf("expected parse failure, got %#v", parseFailure)
	}

	deps = stateRoundtripTestDeps(t, 456)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		payload := validStateRoundtripPayload(t, label, 456)
		if label == "state prune dry-run" {
			payload = statecontract.StatePruneResult{OK: true, DryRun: true, DeletedKeys: []string{}, KeptKeys: []string{"self-verify-456"}}
		}
		return stateRoundtripStep(t, label, command, payload)
	}
	contractFailure := validateStateRoundtripWithDeps("bin", root, 456, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "state prune dry-run did not classify old/fresh keys") {
		t.Fatalf("expected contract failure, got %#v", contractFailure)
	}

	deps = stateRoundtripTestDeps(t, 456)
	deps.stateRead = func(string) (statecontract.StateResult, error) { return statecontract.StateResult{OK: true}, nil }
	dryRunMutation := validateStateRoundtripWithDeps("bin", root, 456, deps)
	if dryRunMutation.OK || !strings.Contains(dryRunMutation.Error, "promote dry-run wrote baseline unexpectedly") {
		t.Fatalf("expected dry-run mutation failure, got %#v", dryRunMutation)
	}
}

func TestStateRoundtripSelfVerifySessionCombineFailedAggregatesContext(t *testing.T) {
	session := newStateRoundtripSelfVerifySession(validateStateRoundtripSelfVerifyInput{
		started:     time.Now(),
		stdoutParts: []string{"state write stdout", "self-verify compare stdout"},
		commands:    []string{"state write", "self-verify compare"},
	})
	child := StepResult{
		Label:           "self-verify compare",
		OK:              false,
		Stderr:          "compare stderr",
		StderrBytes:     14,
		StderrTruncated: true,
		Error:           "regression detected",
	}

	step := session.combineFailed(child)

	if step.OK || step.Label != "state roundtrip" {
		t.Fatalf("expected failed state roundtrip step, got %#v", step)
	}
	if step.Error != "self-verify compare: regression detected" {
		t.Fatalf("unexpected aggregate error: %q", step.Error)
	}
	if step.Command != "state write && self-verify compare" {
		t.Fatalf("unexpected aggregate command: %q", step.Command)
	}
	if !strings.Contains(step.Stdout, "state write stdout\nself-verify compare stdout") {
		t.Fatalf("unexpected aggregate stdout: %q", step.Stdout)
	}
	if step.Stderr != "compare stderr" || step.StderrBytes != 14 || !step.StderrTruncated {
		t.Fatalf("expected child stderr metadata, got %#v", step)
	}
}

func stateRoundtripTestDeps(t *testing.T, seed int64) stateRoundtripValidationDeps {
	t.Helper()
	tempState := t.TempDir()
	return stateRoundtripValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempState, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
		stateRead: func(string) (statecontract.StateResult, error) {
			return statecontract.StateResult{}, errors.New("missing")
		},
		writeSnapshot: func(string, string, SelfAugmentStateSnapshot) error { return nil },
		run: func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
			return stateRoundtripStep(t, label, command, validStateRoundtripPayload(t, label, seed))
		},
	}
}

func stateRoundtripStep(t *testing.T, label string, command []string, payload any) StepResult {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: string(b)}
}

func validStateRoundtripPayload(t *testing.T, label string, seed int64) any {
	t.Helper()
	const updatedAt = "2026-08-02T00:00:00Z"
	key := fmt.Sprintf("self-verify-%d", seed)
	content := fmt.Sprintf("seed=%d\nLore: state roundtrip\n", seed)
	oldKey := key + "-old"
	baseKey := key + "-compare-base"
	candidateKey := key + "-compare-candidate"
	promotedKey := key + "-promoted-baseline"
	switch label {
	case "state write":
		return statecontract.StateResult{OK: true, Path: "/tmp/" + key + ".json", Record: statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: key, Content: content, UpdatedAt: updatedAt, Bytes: len([]byte(content))}}
	case "state read":
		return statecontract.StateResult{OK: true, Record: statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: key, Content: content, UpdatedAt: updatedAt, Bytes: len([]byte(content))}}
	case "state list":
		return statecontract.StateListResult{OK: true, Keys: []string{key, oldKey}}
	case "state old write":
		return statecontract.StateResult{OK: true, Path: "/tmp/" + oldKey + ".json", Record: statecontract.RecordEnvelope{SchemaVersion: statecontract.SchemaVersion, Key: oldKey, Content: "old state", UpdatedAt: updatedAt, Bytes: len([]byte("old state"))}}
	case "state prune dry-run":
		return statecontract.StatePruneResult{OK: true, DryRun: true, DeletedKeys: []string{oldKey}, KeptKeys: []string{key}}
	case "state prune confirm":
		return statecontract.StatePruneResult{OK: true, Confirm: true, DeletedKeys: []string{oldKey}}
	case "state list after prune":
		return statecontract.StateListResult{OK: true, Keys: []string{key}}
	case "self verify compare ok":
		return SelfAugmentCompareResult{OK: true, ElapsedDeltaMS: 100}
	case "self verify compare regression":
		return SelfAugmentCompareResult{OK: true, Regressed: true, Regressions: []string{"elapsed_ms_increased_by_10.00_pct"}}
	case "self verify promote dry-run":
		return SelfAugmentPromoteResult{OK: true, DryRun: true}
	case "self verify promote confirm":
		return SelfAugmentPromoteResult{OK: true, Promoted: true}
	case "self verify compare promoted":
		return SelfAugmentCompareResult{OK: true, ElapsedDeltaMS: 0}
	case "self verify history":
		return SelfAugmentHistoryResult{OK: true, TotalMatches: 3, Entries: []SelfAugmentHistoryEntry{{Key: baseKey}, {Key: candidateKey}, {Key: promotedKey}}}
	case "self verify history retention dry-run":
		return SelfAugmentHistoryResult{OK: true, Retention: &SelfAugmentHistoryRetention{DryRun: true, Limit: 1, CandidateKeys: []string{baseKey}}}
	case "self verify history retention confirm":
		return SelfAugmentHistoryResult{OK: true, Retention: &SelfAugmentHistoryRetention{Confirm: true, DeletedKeys: []string{baseKey}}}
	case "self verify history after retention":
		return SelfAugmentHistoryResult{OK: true, TotalMatches: 1}
	case "state doctor":
		return statecontract.StateDoctorResult{OK: true, Healthy: false, ValidKeys: []string{key}, Issues: []statecontract.StateDoctorIssue{{Code: "invalid_state"}}}
	default:
		t.Fatalf("unexpected state roundtrip label %q", label)
		return nil
	}
}

// prune 계약의 나머지 실패 분기를 잠근다: confirm 응답이 삭제 대상을
// 누락하는 경우와 list-after-prune에서 삭제된 키가 살아남는 경우.
func TestValidateStateRoundtripPruneConfirmAndResidueFailures(t *testing.T) {
	root := t.TempDir()
	deps := stateRoundtripTestDeps(t, 789)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		payload := validStateRoundtripPayload(t, label, 789)
		if label == "state prune confirm" {
			// DryRun이 그대로 true로 오면 confirm 계약 위반이다.
			payload = statecontract.StatePruneResult{OK: true, Confirm: true, DeletedKeys: []string{}}
		}
		return stateRoundtripStep(t, label, command, payload)
	}
	confirmFailure := validateStateRoundtripWithDeps("bin", root, 789, deps)
	if confirmFailure.OK || !strings.Contains(confirmFailure.Error, "state prune confirm did not delete old key") {
		t.Fatalf("expected confirm contract failure, got %#v", confirmFailure)
	}

	deps = stateRoundtripTestDeps(t, 789)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		payload := validStateRoundtripPayload(t, label, 789)
		if label == "state list after prune" {
			// 삭제된 키가 여전히 목록에 남는다.
			payload = statecontract.StateListResult{OK: true, Keys: []string{"self-verify-789", "self-verify-789-old"}}
		}
		return stateRoundtripStep(t, label, command, payload)
	}
	residue := validateStateRoundtripWithDeps("bin", root, 789, deps)
	if residue.OK || !strings.Contains(residue.Error, "state prune did not preserve fresh key and remove old key") {
		t.Fatalf("expected prune residue failure, got %#v", residue)
	}
}
