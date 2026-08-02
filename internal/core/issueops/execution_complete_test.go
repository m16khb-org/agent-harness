package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
)

func TestExecutionCompletePersistsReceiptAndReleasesLease(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-complete")
	prepareExecutionCompletionFixture(t, stateRoot, &fixture)
	actor := executionActor("codex", "complete-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(fixture.worktree, ".agent-harness", "turing", "issue69-report.json")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	head := preflight.GitOut(fixture.worktree, "rev-parse", "HEAD")
	request := ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree,
		FinalHead: head, TuringReportPath: report,
		Verification:      []string{"go test ./... -count=1", "go test -race ./... -count=1"},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}
	completed, err := CompleteExecution(stateRoot, request, ExecutionCompleteDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Execution.Completion == nil || completed.Execution.Completion.FinalHead != head {
		t.Fatalf("completion receipt missing: %#v", completed.Execution)
	}
	if completed.Execution.Lease.Status != issueops.LeaseStatusReleased || completed.Execution.Lease.Holder != nil {
		t.Fatalf("completion did not release the lease: %#v", completed.Execution.Lease)
	}
	persisted, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != IssueOpsPhaseDone {
		t.Fatalf("completion did not atomically enter done: %s", persisted.Phase)
	}
	if entry := persisted.PhaseLedger[IssueOpsPhasePR]; entry.CompletedAt == "" {
		t.Fatalf("completion did not stamp the pr ledger entry: %#v", persisted.PhaseLedger)
	}
	if entry := persisted.PhaseLedger[IssueOpsPhaseDone]; entry.EnteredAt == "" {
		t.Fatalf("completion did not stamp the done ledger entry: %#v", persisted.PhaseLedger)
	}
	if _, err := CompleteExecution(stateRoot, request, ExecutionCompleteDeps{}); err != nil {
		t.Fatalf("byte-identical completion retry must be idempotent: %v", err)
	}
}

func TestExecutionCompleteRequiresVerifiedDurableRemoteArtifact(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*claimableExecutionFixture)
		requestURL string
	}{
		{
			name: "pr phase",
			mutate: func(fixture *claimableExecutionFixture) {
				fixture.record.Phase = IssueOpsPhaseFeedback
			},
		},
		{
			name: "durable artifact",
			mutate: func(fixture *claimableExecutionFixture) {
				fixture.record.RemoteArtifact = nil
			},
		},
		{
			name: "verification timestamp",
			mutate: func(fixture *claimableExecutionFixture) {
				fixture.record.RemoteArtifact.VerifiedAt = ""
			},
		},
		{
			name: "valid artifact projection",
			mutate: func(fixture *claimableExecutionFixture) {
				fixture.record.RemoteArtifact.Provider = "bitbucket"
			},
		},
		{
			name: "target branch match",
			mutate: func(fixture *claimableExecutionFixture) {
				fixture.record.RemoteArtifact.TargetBranch = "release"
			},
		},
		{
			name:       "request artifact url match",
			requestURL: "https://github.com/example/agent-harness/pull/70",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			fixture := newClaimableExecutionFixture(t, stateRoot, "69-complete-"+strings.ReplaceAll(test.name, " ", "-"))
			prepareExecutionCompletionFixture(t, stateRoot, &fixture)
			if test.mutate != nil {
				test.mutate(&fixture)
				if _, err := writeIssueOps(stateRoot, fixture.record); err != nil {
					t.Fatal(err)
				}
			}
			holder := executionActor("codex", "complete-holder-"+test.name)
			if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
				ID: fixture.record.ID, Generation: 1, Actor: holder,
				CWD: fixture.worktree, TokenFile: fixture.tokenPath,
			}); err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(fixture.worktree, "turing.json")
			if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			requestURL := test.requestURL
			if requestURL == "" {
				requestURL = "https://github.com/example/agent-harness/pull/69"
			}
			before, err := ReadIssueOps(stateRoot, fixture.record.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = CompleteExecution(stateRoot, ExecutionCompleteRequest{
				ID: fixture.record.ID, Generation: 1, Actor: holder, CWD: fixture.worktree,
				FinalHead: preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"), TuringReportPath: report,
				Verification: []string{"go test ./..."}, RemoteArtifactURL: requestURL, Confirm: true,
			}, ExecutionCompleteDeps{})
			if err == nil {
				t.Fatalf("completion without %s was accepted", test.name)
			}
			after, readErr := ReadIssueOps(stateRoot, fixture.record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if after.Phase != before.Phase || after.Execution.Completion != nil ||
				after.Execution.Lease.Status != issueops.LeaseStatusActive ||
				!sameNativeActor(after.Execution.Lease.Holder, &holder) {
				t.Fatalf("rejected completion mutated state:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func TestExecutionCompleteRejectsNonHolderAndMismatchedHead(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-complete-deny")
	prepareExecutionCompletionFixture(t, stateRoot, &fixture)
	holder := executionActor("codex", "complete-holder")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: holder,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(fixture.worktree, "turing.json")
	if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1, Actor: executionActor("claude", "not-holder"),
		CWD: fixture.worktree, FinalHead: "0000000000000000000000000000000000000000",
		TuringReportPath: report, Verification: []string{"go test ./..."},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}
	if _, err := CompleteExecution(stateRoot, base, ExecutionCompleteDeps{}); err == nil {
		t.Fatal("non-holder completion was accepted")
	}
	base.Actor = holder
	if _, err := CompleteExecution(stateRoot, base, ExecutionCompleteDeps{}); err == nil {
		t.Fatal("completion with a mismatched final HEAD was accepted")
	}
}

func prepareExecutionCompletionFixture(t *testing.T, stateRoot string, fixture *claimableExecutionFixture) {
	t.Helper()
	fixture.record.Phase = IssueOpsPhasePR
	fixture.record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	fixture.record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/agent-harness/pull/69",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"},
		VerifiedAt: "2026-07-22T00:00:00Z", TargetBranch: "main",
	}
	if _, err := writeIssueOps(stateRoot, fixture.record); err != nil {
		t.Fatal(err)
	}
}
