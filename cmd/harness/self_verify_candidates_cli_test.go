package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRunSelfVerifyCandidatesWithDepsPrintsTextOutput(t *testing.T) {
	out := captureStatusVerifyStdout(t, func() error {
		return runSelfVerifyCandidatesWithDeps([]string{}, selfVerifyCandidatesDeps{
			export: func() SelfVerificationCandidateExportResult {
				return selfVerifyCandidatesCLIResultForTest()
			},
		})
	})

	if !strings.Contains(out, "자기 검증 루프 candidates: 2 candidate(s), selected=open-candidate") ||
		!strings.Contains(out, "- open-candidate coverage score=90.0 status=open") ||
		!strings.Contains(out, "- satisfied-candidate reliability score=0.0 status=already_satisfied") {
		t.Fatalf("unexpected candidates text output:\n%s", out)
	}
}

func TestRunSelfVerifyCandidatesWithDepsSavesAndPrintsJSON(t *testing.T) {
	var savedKey string
	out := captureStatusVerifyStdout(t, func() error {
		return runSelfVerifyCandidatesWithDeps([]string{"--save-state", "--state-key", "candidate-key", "--json"}, selfVerifyCandidatesDeps{
			export: func() SelfVerificationCandidateExportResult {
				return selfVerifyCandidatesCLIResultForTest()
			},
			save: func(result *SelfVerificationCandidateExportResult, key string) error {
				savedKey = key
				result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: true, Key: key}
				return nil
			},
		})
	})
	if savedKey != "candidate-key" {
		t.Fatalf("expected save key candidate-key, got %q", savedKey)
	}
	var result SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode candidates JSON: %v\n%s", err, out)
	}
	if !result.OK || result.StateCheckpoint == nil || result.StateCheckpoint.Key != "candidate-key" {
		t.Fatalf("unexpected candidates JSON result: %+v", result)
	}
}

func TestRunSelfVerifyCandidatesWithDepsPropagatesSaveError(t *testing.T) {
	saveErr := errors.New("save candidates failed")
	err := runSelfVerifyCandidatesWithDeps([]string{"--save-state"}, selfVerifyCandidatesDeps{
		export: func() SelfVerificationCandidateExportResult {
			return selfVerifyCandidatesCLIResultForTest()
		},
		save: func(*SelfVerificationCandidateExportResult, string) error {
			return saveErr
		},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func selfVerifyCandidatesCLIResultForTest() SelfVerificationCandidateExportResult {
	selected := SelfVerificationCandidate{Priority: 1, ID: "open-candidate", Category: "coverage", Status: selfAugmentCandidateStatusOpen, Score: 90}
	satisfied := SelfVerificationCandidate{Priority: 2, ID: "satisfied-candidate", Category: "reliability", Status: selfAugmentCandidateStatusSatisfied, Score: 0}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		CandidateCount:        2,
		OpenCandidateIDs:      []string{selected.ID},
		SatisfiedCandidateIDs: []string{satisfied.ID},
		SelectedCandidate:     &selected,
		Candidates:            []SelfVerificationCandidate{selected, satisfied},
	}
}
