package selfworkflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunSelfVerifyWithDepsCoversLLMEvalSaveStateAndJSON(t *testing.T) {
	var verifyCalled bool
	var evalCalled bool
	var saveCalled bool
	deps := SelfVerifyRunDeps{
		LookupEnv: func(string) (string, bool) { return "", false },
		Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *SelfVerifyProgressReporter) (SelfAugmentResult, error) {
			verifyCalled = true
			if iterations != 1 || baseSeed != 42 || targetScore != 95 || verbose || progress != nil {
				t.Fatalf("unexpected verify args: iterations=%d seed=%d target=%v verbose=%v progress=%v", iterations, baseSeed, targetScore, verbose, progress)
			}
			return SelfAugmentResult{
				OK:                  true,
				LoopKind:            "self_verification",
				KoreanName:          SelfVerificationKoreanName,
				Iterations:          iterations,
				BaseSeed:            baseSeed,
				TargetScore:         targetScore,
				TerminationEligible: true,
				Summary:             SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true},
			}, nil
		},
		ApplyLLMEval: func(result SelfAugmentResult, opts SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
			evalCalled = true
			if !opts.Enabled || opts.Mode != "gate" || opts.AgyCommand != "fake-agy" || opts.TargetScore != 95 {
				t.Fatalf("unexpected LLM eval options: %+v", opts)
			}
			result.LLMEval = &SelfVerifyLLMEvalResult{OK: true, Mode: opts.Mode, Score: 99, Summary: "pass"}
			return result, nil
		},
		SaveSummary: func(result *SelfAugmentResult, key string) error {
			saveCalled = true
			if key != "verify-latest" || result.LLMEval == nil {
				t.Fatalf("unexpected saved result key=%q result=%+v", key, result)
			}
			result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: true, Key: key}
			return nil
		},
	}

	out, err := captureStdoutAllowErrorForSelfVerifyCLITest(t, func() error {
		return RunSelfVerifyWithDeps([]string{
			"--llm-eval",
			"--llm-eval-mode", "gate",
			"--agy-command", "fake-agy",
			"--save-state",
			"--state-key", "verify-latest",
			"--seed", "42",
			"--json",
		}, deps)
	})
	if err != nil {
		t.Fatalf("RunSelfVerifyWithDeps: %v", err)
	}
	if !verifyCalled || !evalCalled || !saveCalled {
		t.Fatalf("expected verify/eval/save calls, got verify=%v eval=%v save=%v", verifyCalled, evalCalled, saveCalled)
	}
	var result SelfAugmentResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode self-verify JSON: %v\n%s", err, out)
	}
	if result.LLMEval == nil || result.LLMEval.Mode != "gate" || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected self-verify JSON result: %+v", result)
	}
}

func TestRunSelfVerifyWithDepsReturnsSaveErrorAfterSuccessfulVerification(t *testing.T) {
	saveErr := errors.New("save failed")
	out, err := captureStdoutAllowErrorForSelfVerifyCLITest(t, func() error {
		return RunSelfVerifyWithDeps([]string{"--save-state", "--state-key", "bad-key", "--json"}, SelfVerifyRunDeps{
			LookupEnv: func(string) (string, bool) { return "", false },
			Verify: func(iterations int, baseSeed int64, targetScore float64, verbose bool, progress *SelfVerifyProgressReporter) (SelfAugmentResult, error) {
				return SelfAugmentResult{OK: true, LoopKind: "self_verification", Summary: SelfAugmentSummary{MinimumGoalScore: 100}}, nil
			},
			SaveSummary: func(result *SelfAugmentResult, key string) error {
				result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: saveErr.Error()}
				return saveErr
			},
		})
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
	if !strings.Contains(out, `"state_checkpoint"`) || !strings.Contains(out, saveErr.Error()) {
		t.Fatalf("expected JSON output to include failed checkpoint, got:\n%s", out)
	}
}

func captureStdoutAllowErrorForSelfVerifyCLITest(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer r.Close()
	os.Stdout = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return out.String(), callErr
}
