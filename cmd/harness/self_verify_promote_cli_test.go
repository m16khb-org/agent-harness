package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRunSelfVerifyPromoteWithDepsPrintsDryRunAndConfirmedText(t *testing.T) {
	calls := 0
	deps := selfVerifyPromoteDeps{
		promote: func(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error) {
			calls++
			if fromKey != "candidate" || baselineKey != "baseline" {
				t.Fatalf("unexpected promote keys: from=%q baseline=%q", fromKey, baselineKey)
			}
			return SelfAugmentPromoteResult{
				OK:          true,
				FromKey:     fromKey,
				BaselineKey: baselineKey,
				Confirm:     confirm,
				DryRun:      !confirm,
				Promoted:    confirm,
			}, nil
		},
	}

	dryRun := captureStatusVerifyStdout(t, func() error {
		return runSelfVerifyPromoteWithDeps([]string{"--from-key", "candidate", "--baseline-key", "baseline"}, deps)
	})
	confirmed := captureStatusVerifyStdout(t, func() error {
		return runSelfVerifyPromoteWithDeps([]string{"--from-key", "candidate", "--baseline-key", "baseline", "--confirm"}, deps)
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

func TestRunSelfVerifyPromoteWithDepsPrintsJSONAndPropagatesErrors(t *testing.T) {
	promoteErr := errors.New("promote failed")
	jsonOut := captureStatusVerifyStdout(t, func() error {
		return runSelfVerifyPromoteWithDeps([]string{"--from-key", "candidate", "--baseline-key", "baseline", "--json"}, selfVerifyPromoteDeps{
			promote: func(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error) {
				return SelfAugmentPromoteResult{
					OK:          true,
					FromKey:     fromKey,
					BaselineKey: baselineKey,
					Confirm:     confirm,
					DryRun:      !confirm,
					Promoted:    confirm,
				}, nil
			},
		})
	})
	var result SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("decode promote JSON: %v\n%s", err, jsonOut)
	}
	if !result.OK || result.FromKey != "candidate" || result.BaselineKey != "baseline" || !result.DryRun || result.Promoted {
		t.Fatalf("unexpected promote JSON result: %+v", result)
	}

	err := runSelfVerifyPromoteWithDeps([]string{"--from-key", "candidate", "--baseline-key", "baseline"}, selfVerifyPromoteDeps{
		promote: func(string, string, bool) (SelfAugmentPromoteResult, error) {
			return SelfAugmentPromoteResult{}, promoteErr
		},
	})
	if !errors.Is(err, promoteErr) {
		t.Fatalf("expected promote error, got %v", err)
	}
}
