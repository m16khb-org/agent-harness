package lifecycle

import (
	"errors"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	statecontract "agent-harness/internal/contract/state"
)

func TestExecutionGuardRecordsUsesSingleBulkSnapshot(t *testing.T) {
	previous := issueOpsDeps
	defer func() {
		issueOpsDeps = previous
	}()
	scanCalls := 0
	issueOpsDeps.ScanIssueOps = func(string) ([]issueopscontract.IssueOpsRecord, error) {
		scanCalls++
		return []issueopscontract.IssueOpsRecord{{
			ID: "io-match",
			Execution: &issueopscontract.Execution{
				Workspace: issueopscontract.Workspace{Root: "/repo/worktree"},
			},
		}}, nil
	}
	issueOpsDeps.ListIssueOpsIDs = func(string) ([]string, error) {
		t.Fatal("bulk execution guard must not list IDs")
		return nil, nil
	}
	issueOpsDeps.ReadIssueOps = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		t.Fatal("bulk execution guard must not read individual records")
		return issueopscontract.IssueOpsRecord{}, nil
	}

	records, err := executionGuardRecords(
		lifecyclecontract.HookToolUseLifecycleRequest{},
		[]string{"/repo/worktree/file.go"},
	)

	if err != nil {
		t.Fatal(err)
	}
	if scanCalls != 1 || len(records) != 1 || records[0].ID != "io-match" {
		t.Fatalf("scan calls=%d records=%+v", scanCalls, records)
	}
}

func TestExecutionGuardRecordsFailsClosedOnInvalidBulkSnapshot(t *testing.T) {
	previous := issueOpsDeps
	defer func() {
		issueOpsDeps = previous
	}()
	issueOpsDeps.ScanIssueOps = func(string) ([]issueopscontract.IssueOpsRecord, error) {
		return nil, statecontract.ErrInvalidState
	}

	records, err := executionGuardRecords(
		lifecyclecontract.HookToolUseLifecycleRequest{},
		[]string{"/repo/worktree/file.go"},
	)

	if !errors.Is(err, statecontract.ErrInvalidState) || records != nil {
		t.Fatalf("records=%+v error=%v", records, err)
	}
}

func TestSealedIssueEditGuardUsesSingleReadableSnapshot(t *testing.T) {
	previousDeps := issueOpsDeps
	previousTargetParser := IssueEditTargetFromCommand
	defer func() {
		issueOpsDeps = previousDeps
		IssueEditTargetFromCommand = previousTargetParser
	}()
	scanCalls := 0
	IssueEditTargetFromCommand = func(string, string, string) (string, bool) {
		return "42", true
	}
	issueOpsDeps.ScanReadableIssueOps = func(string) ([]issueopscontract.IssueOpsRecord, error) {
		scanCalls++
		return []issueopscontract.IssueOpsRecord{}, nil
	}
	issueOpsDeps.ListIssueOpsIDs = func(string) ([]string, error) {
		t.Fatal("sealed issue guard must not list IDs")
		return nil, nil
	}
	issueOpsDeps.ReadIssueOps = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		t.Fatal("sealed issue guard must not read individual records")
		return issueopscontract.IssueOpsRecord{}, nil
	}

	got := sealedIssueEditBlockReason(lifecyclecontract.HookToolUseLifecycleRequest{})

	if got != "" || scanCalls != 1 {
		t.Fatalf("block reason=%q scan calls=%d", got, scanCalls)
	}
}

func TestDelegatedChildSmokeCoordinatorUsesSingleBulkSnapshot(t *testing.T) {
	previous := issueOpsDeps
	defer func() {
		issueOpsDeps = previous
	}()
	scanCalls := 0
	issueOpsDeps.ScanIssueOps = func(string) ([]issueopscontract.IssueOpsRecord, error) {
		scanCalls++
		return []issueopscontract.IssueOpsRecord{}, nil
	}
	issueOpsDeps.ListIssueOpsIDs = func(string) ([]string, error) {
		t.Fatal("delegated child coordinator must not list IDs")
		return nil, nil
	}
	issueOpsDeps.ReadIssueOps = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		t.Fatal("delegated child coordinator must not read individual records")
		return issueopscontract.IssueOpsRecord{}, nil
	}

	id, ok := delegatedChildSmokeCoordinator("/repo", "/repo/worktree", "42")

	if ok || id != "" || scanCalls != 1 {
		t.Fatalf("id=%q ok=%t scan calls=%d", id, ok, scanCalls)
	}
}
