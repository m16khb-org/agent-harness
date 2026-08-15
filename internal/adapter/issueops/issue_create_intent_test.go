package issueops

import (
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestIssueCreateIntentPersistsBeforeMutationAndBlocksConcurrentBegin(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "42-durable-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "11111111111111111111111111111111",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Durable issue",
		BodySHA256:       strings.Repeat("a", 64),
		Labels:           []string{"quality"},
		Assignees:        []string{"owner"},
		StartedAt:        "2026-08-14T00:00:00Z",
	}

	updated, err := BeginIssueCreateIntent(stateRoot, record.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if updated.IssueCreateIntent == nil || updated.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("intent = %+v", updated.IssueCreateIntent)
	}
	if !strings.Contains(updated.IssueCreateIntent.Marker, request.OperationID) {
		t.Fatalf("marker = %q", updated.IssueCreateIntent.Marker)
	}
	if _, err := BeginIssueCreateIntent(stateRoot, record.ID, request); err == nil {
		t.Fatal("concurrent begin must fail while outcome is pending")
	}
}

func TestIssueCreateIntentRetriesOnlyProvenNonInvocation(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "43-retry-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "22222222222222222222222222222222",
		Provider:         "gitlab",
		ProjectAuthority: "gitlab.example.com/acme/repo",
		Title:            "Retryable issue",
		BodySHA256:       strings.Repeat("b", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := BeginIssueCreateIntent(stateRoot, record.ID, request); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueCreateOutcome(stateRoot, record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentNotInvoked,
		Failure:    "process start failed",
		ObservedAt: "2026-08-14T00:01:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	retried, err := BeginIssueCreateIntent(stateRoot, record.ID, request)
	if err != nil {
		t.Fatalf("proven non-invocation must permit retry: %v", err)
	}
	if retried.IssueCreateIntent.Attempt != 2 || retried.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("retried intent = %+v", retried.IssueCreateIntent)
	}
	if _, err := RecordIssueCreateOutcome(stateRoot, record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentInvokedUnknown,
		Failure:    "timeout after process start",
		ObservedAt: "2026-08-14T00:02:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueCreateOutcome(stateRoot, record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
		Status:     issueopscontract.IssueCreateIntentNotInvoked,
		Failure:    "invalid downgrade",
		ObservedAt: "2026-08-14T00:03:00Z",
	}); err == nil {
		t.Fatal("invoked_unknown must never transition back to not_invoked")
	}
	if _, err := BeginIssueCreateIntent(stateRoot, record.ID, request); err == nil {
		t.Fatal("ambiguous invocation must block retry")
	}
}

func TestCompleteIssueCreateIntentLinksCanonicalURLAtomically(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "44-complete-issue"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "33333333333333333333333333333333",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Complete issue",
		BodySHA256:       strings.Repeat("c", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := BeginIssueCreateIntent(stateRoot, record.ID, request); err != nil {
		t.Fatal(err)
	}
	const issueURL = "https://github.com/acme/repo/issues/44"
	completed, err := CompleteIssueCreateIntent(stateRoot, record.ID, issueURL, "2026-08-14T00:03:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if completed.IssueURL != issueURL || completed.IssueCreateIntent == nil {
		t.Fatalf("completed record = %+v", completed)
	}
	if completed.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted ||
		completed.IssueCreateIntent.CanonicalURL != issueURL {
		t.Fatalf("completed intent = %+v", completed.IssueCreateIntent)
	}
	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.IssueURL != issueURL || reloaded.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		t.Fatalf("reloaded record = %+v", reloaded)
	}
}

func TestCompleteIssueCreateIntentRejectsDifferentProjectAuthority(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "45-authority-mismatch"})
	if err != nil {
		t.Fatal(err)
	}
	request := issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      "0123456789abcdef0123456789abcdef",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Authority-bound issue",
		BodySHA256:       strings.Repeat("d", 64),
		StartedAt:        "2026-08-14T00:00:00Z",
	}
	if _, err := BeginIssueCreateIntent(stateRoot, record.ID, request); err != nil {
		t.Fatal(err)
	}

	_, err = CompleteIssueCreateIntent(stateRoot, record.ID, "https://github.com/other/repo/issues/45", "2026-08-14T00:01:00Z")

	if err == nil || !strings.Contains(err.Error(), "project authority") {
		t.Fatalf("error = %v, want project authority mismatch", err)
	}
	reloaded, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if reloaded.IssueURL != "" || reloaded.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentPending {
		t.Fatalf("mismatched authority changed durable state: %+v", reloaded)
	}
}
