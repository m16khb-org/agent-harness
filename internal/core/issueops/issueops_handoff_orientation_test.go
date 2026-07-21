package issueops

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func TestOwnershipClaimEntersOrientationWithoutWriteLease(t *testing.T) {
	_, record, _ := ownershipOrientingRecord(t)
	if record.ExecutionHandoff.State != handoff.StateOwnerOrienting || record.ExecutionHandoff.OwnerSession == nil || record.Phase == IssueOpsPhaseImplement {
		t.Fatalf("ownership claim must enter orientation without implementation lease: %#v", record)
	}
}

func TestOwnershipClaimReturnsExecutableAcknowledgementCommand(t *testing.T) {
	stateRoot, record, claim := ownershipOrientingRecord(t)
	result, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	if result.NextCommand == "" || strings.Contains(result.NextCommand, "issueops resume") {
		t.Fatalf("claim next command must be the exact acknowledgement transition: %q", result.NextCommand)
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(result.NextCommand)
	if !ok || command.Path != "handoff acknowledge-context" {
		t.Fatalf("claim next command is not an exact acknowledgement command: path=%q ok=%v command=%q", command.Path, ok, result.NextCommand)
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatalf("acknowledgement command spec is missing for %q", command.Path)
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		t.Fatalf("claim next command flags are not exact: %q", result.NextCommand)
	}
	attempt, err := strconv.Atoi(flags["--attempt"][0])
	if err != nil {
		t.Fatal(err)
	}
	for flag, want := range map[string]string{
		"--id":              record.ID,
		"--ownership-epoch": claim.OwnershipEpoch,
		"--context-sha256":  claim.ContextSHA256,
		"--host":            claim.Host,
		"--session-id":      claim.SessionID,
		"--agent-id":        claim.AgentID,
		"--cwd":             claim.CWD,
		"--issue-url":       record.IssueURL,
	} {
		if got := flags[flag][0]; got != want {
			t.Fatalf("claim next command %s = %q, want %q", flag, got, want)
		}
	}
	if attempt != claim.Attempt || flags["--plan-sha256"][0] == "" || flags["--understanding"][0] == "" || flags["--scope-confirmation"][0] == "" {
		t.Fatalf("claim next command is incomplete: attempt=%d flags=%#v", attempt, flags)
	}

	activated, err := AcknowledgeIssueOpsHandoffContext(stateRoot, IssueOpsHandoffAcknowledgeRequest{
		ID: flags["--id"][0], Attempt: attempt, OwnershipEpoch: flags["--ownership-epoch"][0], ContextSHA256: flags["--context-sha256"][0],
		Host: flags["--host"][0], SessionID: flags["--session-id"][0], AgentID: flags["--agent-id"][0], CWD: flags["--cwd"][0],
		IssueURL: flags["--issue-url"][0], PlanSHA256: flags["--plan-sha256"][0], Understanding: flags["--understanding"][0], ScopeConfirmation: flags["--scope-confirmation"][0],
	})
	if err != nil {
		t.Fatalf("execute claim next command: %v", err)
	}
	if activated.ExecutionHandoff.State != handoff.StateOwnerActive {
		t.Fatalf("claim next command state = %q, want %q", activated.ExecutionHandoff.State, handoff.StateOwnerActive)
	}
}

func TestOwnershipOrientationRequiresExactIssuePlanAndContext(t *testing.T) {
	stateRoot, record, claim := ownershipOrientingRecord(t)
	packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	req := IssueOpsHandoffAcknowledgeRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, CWD: claim.CWD,
		IssueURL: record.IssueURL, PlanSHA256: packet.PlanSHA256, Understanding: "I will implement only the sealed cycle.", ScopeConfirmation: "worker root only",
	}
	wrongPlan := req
	wrongPlan.PlanSHA256 = strings.Repeat("0", 64)
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, wrongPlan); err == nil {
		t.Fatal("wrong plan SHA must be rejected")
	}
	wrongContext := req
	wrongContext.ContextSHA256 = strings.Repeat("0", 64)
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, wrongContext); err == nil {
		t.Fatal("wrong context fence must be rejected")
	}
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateOwnerActive || persisted.ExecutionHandoff.Orientation == nil {
		t.Fatalf("ownership acknowledgement did not grant active owner: %#v", persisted.ExecutionHandoff)
	}
	if repeated, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err != nil || repeated.ExecutionHandoff.Orientation.RecordedAt != persisted.ExecutionHandoff.Orientation.RecordedAt {
		t.Fatalf("identical acknowledgement must be idempotent: record=%#v err=%v", repeated.ExecutionHandoff, err)
	}

	req.Understanding = "conflicting acknowledgement"
	if _, err := AcknowledgeIssueOpsHandoffContext(stateRoot, req); err == nil {
		t.Fatal("conflicting acknowledgement must fail")
	}
}

func TestConcurrentHandoffPreflightRejectsSharedCoordinatorMailbox(t *testing.T) {
	stateRoot, active, _ := ownershipActiveRecorderRecord(t)
	active.ExecutionHandoff.CoordinatorMailboxHandle = "term_source"
	active, err := WriteIssueOps(stateRoot, active)
	if err != nil {
		t.Fatal(err)
	}
	second := IssueOpsRecord{ID: NewIssueOpsID(active.Repo, "17-second"), Repo: active.Repo}
	if _, err = resolveHandoffCoordinatorRecipient(context.Background(), stateRoot, second, active.ExecutionHandoff.CoordinatorMailboxHandle, model.IssueOpsHostSessionIdentity{}, nil); err == nil || !strings.Contains(err.Error(), "sealed by another active handoff") {
		t.Fatalf("shared coordinator mailbox must fail before dispatch mutation: %v", err)
	}
	got, err := resolveHandoffCoordinatorRecipient(context.Background(), stateRoot, second, "term_second", model.IssueOpsHostSessionIdentity{}, nil)
	if err != nil {
		t.Fatalf("distinct coordinator mailbox must pass preflight: %v", err)
	}
	if got != "term_second" {
		t.Fatalf("distinct coordinator mailbox = %q, want term_second", got)
	}
}

func ownershipOrientingRecord(t *testing.T) (string, IssueOpsRecord, IssueOpsHandoffClaimRequest) {
	t.Helper()
	stateRoot, dispatched, actor := ownershipActiveRecorderRecord(t)
	dispatched.ExecutionHandoff.State = handoff.StateOwnershipDispatched
	dispatched.ExecutionHandoff.OwnerSession = nil
	dispatched.ExecutionHandoff.Orientation = nil
	if _, err := WriteIssueOps(stateRoot, dispatched); err != nil {
		t.Fatal(err)
	}
	claim := IssueOpsHandoffClaimRequest{
		ID: dispatched.ID, Attempt: dispatched.ExecutionHandoff.Attempt, OwnershipEpoch: dispatched.ExecutionHandoff.OwnershipEpoch,
		ContextSHA256: dispatched.ExecutionHandoff.ContextSHA256, Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		CWD: dispatched.WorktreePath, OrcaWorktreeID: dispatched.ExecutionHandoff.Orca.WorktreeID,
	}
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, claimed.IssueOpsRecord, claim
}
