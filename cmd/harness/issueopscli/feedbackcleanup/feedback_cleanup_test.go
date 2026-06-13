package feedbackcleanup

import (
	"flag"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunFeedbackAddAndMarkIssueUpdated(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var printed []core.IssueOpsRecord
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintResult: func(record core.IssueOpsRecord, jsonOut bool, err error) error {
			if err != nil {
				return err
			}
			printed = append(printed, record)
			return nil
		},
	}
	if err := RunFeedback([]string{"add", "--id", record.ID, "--source", "review", "--body", "fix this", "--classification", "contract_change", "--json"}, deps); err != nil {
		t.Fatalf("RunFeedback add returned error: %v", err)
	}
	if err := RunFeedback([]string{"mark-issue-updated", "--id", record.ID}, deps); err != nil {
		t.Fatalf("RunFeedback mark returned error: %v", err)
	}
	if len(printed) != 2 {
		t.Fatalf("expected two printed records, got %d", len(printed))
	}
	if len(printed[0].Feedback) != 1 || printed[0].Feedback[0].Classification != "contract_change" {
		t.Fatalf("unexpected feedback record: %#v", printed[0].Feedback)
	}
	if printed[1].Feedback[0].IssueUpdatedAt == "" {
		t.Fatalf("expected issue updated timestamp: %#v", printed[1].Feedback[0])
	}
}

func TestRunCleanupStatusAndJSONError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var statuses []any
	var printedErrors []error
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			statuses = append(statuses, value)
			return nil
		},
		PrintError: func(err error) error {
			printedErrors = append(printedErrors, err)
			return nil
		},
		VerifyMerged: func(core.IssueOpsRemoteArtifactVerification) error {
			return nil
		},
	}
	if err := RunCleanup([]string{"status", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("RunCleanup status returned error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected one status output, got %d", len(statuses))
	}
	if err := RunCleanup([]string{"status", "--id", "missing", "--json"}, deps); err == nil {
		t.Fatal("expected missing status error")
	}
	if len(printedErrors) != 1 {
		t.Fatalf("expected JSON error to be printed, got %d", len(printedErrors))
	}
}

func TestCleanupMergedAndCommandBoundaries(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	deps := Deps{
		ParseFlags:   parseFeedbackCleanupFlags,
		PrintResult:  func(core.IssueOpsRecord, bool, error) error { return nil },
		VerifyMerged: func(core.IssueOpsRemoteArtifactVerification) error { return nil },
	}
	if CleanupMerged(record.ID, false, deps) {
		t.Fatal("unrequested merge confirmation should be false")
	}
	if CleanupMerged("missing", true, deps) {
		t.Fatal("missing record should not verify merged")
	}
	if err := RunFeedback(nil, deps); err != nil {
		t.Fatalf("help feedback returned error: %v", err)
	}
	if err := RunCleanup(nil, deps); err != nil {
		t.Fatalf("help cleanup returned error: %v", err)
	}
	if err := RunFeedback([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops feedback") {
		t.Fatalf("expected unknown feedback error, got %v", err)
	}
	if err := RunCleanup([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops cleanup") {
		t.Fatalf("expected unknown cleanup error, got %v", err)
	}
	if err := RunFeedback([]string{"add", "--bad"}, deps); err == nil {
		t.Fatal("expected parse flag error")
	}
}

func feedbackCleanupIssueOpsRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "1234-feedback-cleanup"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	return record
}

func parseFeedbackCleanupFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
