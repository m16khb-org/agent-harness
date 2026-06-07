package augmentcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestRunSavesPlanStateAsJSON(t *testing.T) {
	var savedKey string
	out := captureStdout(t, func() error {
		return Run([]string{"--cycles", "1", "--target-score", "99", "--save-state", "--state-key", "augment-plan", "--json"}, Deps{
			Plan: func(req model.SelfAugmentPlanRequest) model.SelfAugmentPlanResult {
				return model.SelfAugmentPlanResult{
					OK:          true,
					LoopKind:    "self_augmentation",
					KoreanName:  model.SelfAugmentationKoreanName,
					Cycles:      req.Cycles,
					TargetScore: req.TargetScore,
				}
			},
			SavePlan: func(result *model.SelfAugmentPlanResult, key string) error {
				savedKey = key
				result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: true, Key: key}
				return nil
			},
			PrintJSON: printJSONForTest,
		})
	})
	if savedKey != "augment-plan" {
		t.Fatalf("expected save key augment-plan, got %q", savedKey)
	}
	var result model.SelfAugmentPlanResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode self-augment JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Cycles != 1 || result.TargetScore != 99 || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected self-augment result: %#v", result)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	err = fn()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stdout writer: %v", closeErr)
	}
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err != nil {
		t.Fatalf("function returned error: %v", err)
	}
	return string(out)
}

func printJSONForTest(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
