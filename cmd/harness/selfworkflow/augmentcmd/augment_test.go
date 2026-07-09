package augmentcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/testsupport"
)

func TestRunDelegatesSubcommands(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantKind  string
		wantArgs  []string
		returnErr error
	}{
		{name: "lesson", args: []string{"lesson", "--topic", "qa"}, wantKind: "lesson", wantArgs: []string{"--topic", "qa"}},
		{name: "verify", args: []string{"verify", "--seed", "100"}, wantKind: "verify", wantArgs: []string{"--seed", "100"}},
		{name: "history", args: []string{"history", "--json"}, wantKind: "verify", wantArgs: []string{"history", "--json"}},
		{name: "compare", args: []string{"compare", "a", "b"}, wantKind: "verify", wantArgs: []string{"compare", "a", "b"}},
		{name: "promote", args: []string{"promote", "--id", "candidate"}, wantKind: "verify", wantArgs: []string{"promote", "--id", "candidate"}},
		{name: "delegated error", args: []string{"lesson"}, wantKind: "lesson", returnErr: errors.New("lesson failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotKind string
			var gotArgs []string
			err := Run(tc.args, Deps{
				RunLesson: func(args []string) error {
					gotKind = "lesson"
					gotArgs = append([]string(nil), args...)
					return tc.returnErr
				},
				RunVerify: func(args []string) error {
					gotKind = "verify"
					gotArgs = append([]string(nil), args...)
					return tc.returnErr
				},
			})
			if !errors.Is(err, tc.returnErr) {
				t.Fatalf("error=%v, want %v", err, tc.returnErr)
			}
			if gotKind != tc.wantKind {
				t.Fatalf("delegated to %q, want %q", gotKind, tc.wantKind)
			}
			if strings.Join(gotArgs, "\x00") != strings.Join(tc.wantArgs, "\x00") {
				t.Fatalf("args=%v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestRunValidatesFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "invalid flag", args: []string{"--unknown"}, wantErr: "flag provided but not defined"},
		{name: "cycles must be positive", args: []string{"--cycles", "0"}, wantErr: "cycles must be positive"},
		{name: "target score lower bound", args: []string{"--target-score", "-1"}, wantErr: "target-score must be >= 0 and < 100"},
		{name: "target score upper bound", args: []string{"--target-score", "100"}, wantErr: "target-score must be >= 0 and < 100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args, Deps{Output: io.Discard})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestRunPrintsTextPlanAndSelectedCandidate(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"--cycles", "2", "--target-score", "95"}, Deps{
		Output: &out,
		Plan: func(req model.SelfAugmentPlanRequest) model.SelfAugmentPlanResult {
			candidate := &model.SelfAugmentCandidate{ID: "quality-next", Title: "Improve the next quality signal"}
			return model.SelfAugmentPlanResult{
				OK:                  true,
				KoreanName:          model.SelfAugmentationKoreanName,
				Cycles:              req.Cycles,
				TargetScore:         req.TargetScore,
				SelectedCandidate:   candidate,
				Candidates:          []model.SelfAugmentCandidate{*candidate},
				TerminationEligible: true,
				Goals: []model.SelfAugmentGoal{{
					Name:        "quality",
					KoreanName:  "품질",
					Score:       100,
					TargetScore: req.TargetScore,
					Passed:      true,
				}},
			}
		},
		SelectedCandidateID: func(candidate *model.SelfAugmentCandidate) string {
			if candidate == nil {
				return ""
			}
			return "selected:" + candidate.ID
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		model.SelfAugmentationKoreanName + " plan: 1 candidate(s), selected=selected:quality-next, termination_eligible=true",
		"- 품질 score=100.0 target>95.0 passed=true",
		"selected: quality-next",
		"Improve the next quality signal",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output does not contain %q:\n%s", want, got)
		}
	}
}

func TestRunReportsDefaultDependencyErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "lesson default", args: []string{"lesson"}, wantErr: "lesson runner dependency is required"},
		{name: "verify default", args: []string{"verify"}, wantErr: "self-verify runner dependency is required"},
		{name: "save default", args: []string{"--save-state"}, wantErr: "state dependency is required"},
		{name: "json default", args: []string{"--json"}, wantErr: "JSON printer dependency is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run(tc.args, Deps{Output: io.Discard})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

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
	return testsupport.CaptureStdout(t, fn)
}

func printJSONForTest(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
