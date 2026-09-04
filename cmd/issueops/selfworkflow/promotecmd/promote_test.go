package promotecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"issueops/cmd/issueops/selfworkflow/model"
	"issueops/internal/testsupport"
)

func TestRunPrintsDryRunAndConfirmedText(t *testing.T) {
	calls := 0
	deps := Deps{
		Promote: func(fromKey, baselineKey string, confirm, allowFailedSource bool) (model.SelfAugmentPromoteResult, error) {
			calls++
			if fromKey != "candidate" || baselineKey != "baseline" {
				t.Fatalf("unexpected promote keys: from=%q baseline=%q", fromKey, baselineKey)
			}
			return model.SelfAugmentPromoteResult{
				OK:          true,
				FromKey:     fromKey,
				BaselineKey: baselineKey,
				Confirm:     confirm,
				DryRun:      !confirm,
				Promoted:    confirm,
			}, nil
		},
	}

	dryRun := captureStdout(t, func() error {
		return Run([]string{"--from-key", "candidate", "--baseline-key", "baseline"}, deps)
	})
	confirmed := captureStdout(t, func() error {
		return Run([]string{"--from-key", "candidate", "--baseline-key", "baseline", "--confirm"}, deps)
	})

	if calls != 2 {
		t.Fatalf("expected two promote calls, got %d", calls)
	}
	if !strings.Contains(dryRun, `would promote self-verification summary "candidate" to baseline "baseline"`) {
		t.Fatalf("unexpected dry-run text:\n%s", dryRun)
	}
	if !strings.Contains(confirmed, `promoted self-verification summary "candidate" to baseline "baseline"`) {
		t.Fatalf("unexpected confirmed text:\n%s", confirmed)
	}
}

func TestRunPrintsJSONAndPropagatesErrors(t *testing.T) {
	promoteErr := errors.New("promote failed")
	jsonOut := captureStdout(t, func() error {
		return Run([]string{"--from-key", "candidate", "--baseline-key", "baseline", "--json"}, Deps{
			Promote: func(fromKey, baselineKey string, confirm, allowFailedSource bool) (model.SelfAugmentPromoteResult, error) {
				return model.SelfAugmentPromoteResult{
					OK:          true,
					FromKey:     fromKey,
					BaselineKey: baselineKey,
					Confirm:     confirm,
					DryRun:      !confirm,
					Promoted:    confirm,
				}, nil
			},
			PrintJSON: printJSONForTest,
		})
	})
	var result model.SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("decode promote JSON: %v\n%s", err, jsonOut)
	}
	if !result.OK || result.FromKey != "candidate" || result.BaselineKey != "baseline" || !result.DryRun || result.Promoted {
		t.Fatalf("unexpected promote JSON result: %+v", result)
	}

	err := Run([]string{"--from-key", "candidate", "--baseline-key", "baseline"}, Deps{
		Promote: func(string, string, bool, bool) (model.SelfAugmentPromoteResult, error) {
			return model.SelfAugmentPromoteResult{}, promoteErr
		},
	})
	if !errors.Is(err, promoteErr) {
		t.Fatalf("expected promote error, got %v", err)
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
