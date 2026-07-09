package verifycmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/llmeval"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/cmd/harness/selfworkflow/progress"
	"agent-harness/internal/testsupport"
)

func TestRunCoversLLMEvalSaveStateAndJSON(t *testing.T) {
	var verifyCalled bool
	var evalCalled bool
	var saveCalled bool
	deps := Deps{
		LookupEnv: func(string) (string, bool) { return "", false },
		Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, reporter *progress.SelfVerifyProgressReporter, _ bool) (model.SelfAugmentResult, error) {
			verifyCalled = true
			if iterations != 1 || baseSeed != 42 || targetScore != 95 || verbose || reporter != nil {
				t.Fatalf("unexpected verify args: iterations=%d seed=%d target=%v verbose=%v progress=%v", iterations, baseSeed, targetScore, verbose, reporter)
			}
			return model.SelfAugmentResult{
				OK:                  true,
				LoopKind:            "self_verification",
				KoreanName:          model.SelfVerificationKoreanName,
				Iterations:          iterations,
				BaseSeed:            baseSeed,
				TargetScore:         targetScore,
				TerminationEligible: true,
				Summary:             model.SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true},
			}, nil
		},
		ApplyLLMEval: func(result model.SelfAugmentResult, opts llmeval.SelfVerifyLLMEvalOptions) (model.SelfAugmentResult, error) {
			evalCalled = true
			if !opts.Enabled || opts.Mode != "gate" || opts.TargetScore != 95 {
				t.Fatalf("unexpected LLM eval options: %+v", opts)
			}
			result.LLMEval = &model.SelfVerifyLLMEvalResult{OK: true, Mode: opts.Mode, Score: 99, Summary: "pass"}
			return result, nil
		},
		SaveSummary: func(result *model.SelfAugmentResult, key string) error {
			saveCalled = true
			if key != "verify-latest" || result.LLMEval == nil {
				t.Fatalf("unexpected saved result key=%q result=%+v", key, result)
			}
			result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: true, Key: key}
			return nil
		},
		PrintJSON: printJSONForTest,
	}

	out, err := captureStdoutAllowError(t, func() error {
		return Run([]string{
			"--llm-eval",
			"--llm-eval-mode", "gate",
			"--save-state",
			"--state-key", "verify-latest",
			"--seed", "42",
			"--json",
		}, deps)
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !verifyCalled || !evalCalled || !saveCalled {
		t.Fatalf("expected verify/eval/save calls, got verify=%v eval=%v save=%v", verifyCalled, evalCalled, saveCalled)
	}
	var result model.SelfAugmentResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode self-verify JSON: %v\n%s", err, out)
	}
	if result.LLMEval == nil || result.LLMEval.Mode != "gate" || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected self-verify JSON result: %+v", result)
	}
}

func TestRunReturnsSaveErrorAfterSuccessfulVerification(t *testing.T) {
	saveErr := errors.New("save failed")
	out, err := captureStdoutAllowError(t, func() error {
		return Run([]string{"--save-state", "--state-key", "bad-key", "--json"}, Deps{
			LookupEnv: func(string) (string, bool) { return "", false },
			Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, reporter *progress.SelfVerifyProgressReporter, _ bool) (model.SelfAugmentResult, error) {
				return model.SelfAugmentResult{OK: true, LoopKind: "self_verification", Summary: model.SelfAugmentSummary{MinimumGoalScore: 100}}, nil
			},
			SaveSummary: func(result *model.SelfAugmentResult, key string) error {
				result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: false, Key: key, Error: saveErr.Error()}
				return saveErr
			},
			PrintJSON: printJSONForTest,
		})
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
	if !strings.Contains(out, `"state_checkpoint"`) || !strings.Contains(out, saveErr.Error()) {
		t.Fatalf("expected JSON output to include failed checkpoint, got:\n%s", out)
	}
}

func captureStdoutAllowError(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStdoutAndError(t, fn)
}

func printJSONForTest(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
