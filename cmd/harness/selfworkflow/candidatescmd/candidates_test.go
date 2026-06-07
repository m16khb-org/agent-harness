package candidatescmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/candidateexport"
	"agent-harness/cmd/harness/selfworkflow/model"
)

func TestRunPrintsTextOutput(t *testing.T) {
	out := captureStdout(t, func() error {
		return Run([]string{}, Deps{
			Export: func() candidateexport.SelfVerificationCandidateExportResult {
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

func TestRunSavesAndPrintsJSON(t *testing.T) {
	var savedKey string
	out := captureStdout(t, func() error {
		return Run([]string{"--save-state", "--state-key", "candidate-key", "--json"}, Deps{
			Export: func() candidateexport.SelfVerificationCandidateExportResult {
				return selfVerifyCandidatesCLIResultForTest()
			},
			Save: func(result *candidateexport.SelfVerificationCandidateExportResult, key string) error {
				savedKey = key
				result.StateCheckpoint = &model.SelfAugmentStateCheckpoint{OK: true, Key: key}
				return nil
			},
			PrintJSON: printJSONForTest,
		})
	})
	if savedKey != "candidate-key" {
		t.Fatalf("expected save key candidate-key, got %q", savedKey)
	}
	var result candidateexport.SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode candidates JSON: %v\n%s", err, out)
	}
	if !result.OK || result.StateCheckpoint == nil || result.StateCheckpoint.Key != "candidate-key" {
		t.Fatalf("unexpected candidates JSON result: %+v", result)
	}
}

func TestRunPropagatesSaveError(t *testing.T) {
	saveErr := errors.New("save candidates failed")
	err := Run([]string{"--save-state"}, Deps{
		Export: func() candidateexport.SelfVerificationCandidateExportResult {
			return selfVerifyCandidatesCLIResultForTest()
		},
		Save: func(*candidateexport.SelfVerificationCandidateExportResult, string) error {
			return saveErr
		},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func selfVerifyCandidatesCLIResultForTest() candidateexport.SelfVerificationCandidateExportResult {
	selected := candidateexport.SelfVerificationCandidate{Priority: 1, ID: "open-candidate", Category: "coverage", Status: "open", Score: 90}
	satisfied := candidateexport.SelfVerificationCandidate{Priority: 2, ID: "satisfied-candidate", Category: "reliability", Status: "already_satisfied", Score: 0}
	return candidateexport.SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  candidateexport.SelfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            model.SelfVerificationKoreanName,
		CandidateCount:        2,
		OpenCandidateIDs:      []string{selected.ID},
		SatisfiedCandidateIDs: []string{satisfied.ID},
		SelectedCandidate:     &selected,
		Candidates:            []candidateexport.SelfVerificationCandidate{selected, satisfied},
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
