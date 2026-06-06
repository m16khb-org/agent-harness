package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateStepBudgetBaselineWithDepsCoversSuccessAndWriteFailure(t *testing.T) {
	root := t.TempDir()
	tempState := t.TempDir()
	writes := []string{}
	deps := stepBudgetValidationDeps{
		makeTempState: func(seed int64) (string, error) {
			if seed != 42 {
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
		writeSnapshot: func(dir, key string, snapshot SelfAugmentStateSnapshot) error {
			writes = append(writes, key)
			if dir != tempState || snapshot.Kind != selfVerificationSummaryKind || snapshot.LoopKind != "self_verification" || snapshot.HarnessRoot != root {
				t.Fatalf("unexpected snapshot write: dir=%q key=%q snapshot=%+v", dir, key, snapshot)
			}
			return nil
		},
		run: func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			if dir != root || label != "step budget baseline" || timeout != 30*time.Second || stdin != "" || len(env) != 1 || env[0] != "HARNESS_STATE_DIR="+tempState {
				t.Fatalf("unexpected compare envelope: dir=%q label=%q timeout=%s stdin=%q env=%v", dir, label, timeout, stdin, env)
			}
			command := strings.Join(append([]string{name}, args...), " ")
			if !strings.Contains(command, "self-verify compare") || !strings.Contains(command, "--baseline-key self-verify-budget-baseline-42") || !strings.Contains(command, "--candidate-key self-verify-budget-candidate-42") {
				t.Fatalf("unexpected compare command: %s", command)
			}
			return StepResult{Label: label, Command: command, OK: true, Stdout: mustMarshalStepBudgetTest(t, validStepBudgetCompareResult())}
		},
	}
	step := validateStepBudgetBaselineWithDeps("bin/agent-harness", root, 42, deps)
	if !step.OK || len(writes) != 2 || writes[0] != "self-verify-budget-baseline-42" || writes[1] != "self-verify-budget-candidate-42" {
		t.Fatalf("unexpected step budget success: step=%+v writes=%v", step, writes)
	}

	deps.writeSnapshot = func(string, string, SelfAugmentStateSnapshot) error { return errors.New("write fail") }
	failed := validateStepBudgetBaselineWithDeps("bin", root, 42, deps)
	if failed.OK || failed.Error != "write fail" {
		t.Fatalf("unexpected write failure: %+v", failed)
	}
}

func TestValidateStepBudgetBaselineWithDepsCoversCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	tempState := t.TempDir()
	deps := stepBudgetValidationDeps{
		makeTempState: func(int64) (string, error) { return tempState, nil },
		removeAll:     func(string) error { return nil },
		writeSnapshot: func(string, string, SelfAugmentStateSnapshot) error { return nil },
	}

	deps.makeTempState = func(int64) (string, error) { return "", errors.New("temp fail") }
	tempFailure := validateStepBudgetBaselineWithDeps("bin", root, 7, deps)
	if tempFailure.OK || tempFailure.Error != "temp fail" {
		t.Fatalf("unexpected temp failure: %+v", tempFailure)
	}
	deps.makeTempState = func(int64) (string, error) { return tempState, nil }

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "step budget baseline", Command: "compare", OK: false, Error: "boom"}
	}
	commandFailure := validateStepBudgetBaselineWithDeps("bin", root, 7, deps)
	if commandFailure.OK || !strings.Contains(commandFailure.Error, "step budget baseline: boom") {
		t.Fatalf("unexpected command failure: %+v", commandFailure)
	}

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "step budget baseline", Command: "compare", OK: true, Stdout: "{"}
	}
	parseFailure := validateStepBudgetBaselineWithDeps("bin", root, 7, deps)
	if parseFailure.OK || parseFailure.Error == "" {
		t.Fatalf("expected compare JSON failure, got %+v", parseFailure)
	}

	invalidResult := validStepBudgetCompareResult()
	invalidResult.StepBudgetRegressions = nil
	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "step budget baseline", Command: "compare", OK: true, Stdout: mustMarshalStepBudgetTest(t, invalidResult)}
	}
	contractFailure := validateStepBudgetBaselineWithDeps("bin", root, 7, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "step budget compare did not report exactly one budget regression") {
		t.Fatalf("unexpected contract failure: %+v", contractFailure)
	}
}

func validStepBudgetCompareResult() SelfAugmentCompareResult {
	return SelfAugmentCompareResult{
		OK:        true,
		Regressed: true,
		Regressions: []string{
			"step_budget:docs index smoke_p95_increased_by_30.00_pct",
		},
		SlowStepRegressions: []SelfAugmentSlowStepRegression{},
		StepBudgetRegressions: []SelfAugmentStepBudgetRegression{
			{Label: "docs index smoke", Metric: "p95_duration_ms", DeltaMS: 30, DeltaPct: 30},
		},
	}
}

func mustMarshalStepBudgetTest(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
