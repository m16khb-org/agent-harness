package handoff

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestCurrentHandoffEnvelopeHasNoProtocolVersion(t *testing.T) {
	encoded, err := json.Marshal(validOwnershipEnvelopeRecord(t).ExecutionHandoff)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "protocol_version") {
		t.Fatalf("current handoff envelope still exposes protocol version: %s", encoded)
	}
}

func TestOwnershipEnvelopeStateMatrix(t *testing.T) {
	for _, state := range []string{StateOwnershipDispatching, StateOwnershipDispatched, StateOwnerOrienting, StateOwnerActive, StateCleanupPendingHumanDecision, StateCleanupExecuting, StateClosed, StateRecoveryRequired} {
		record := validOwnershipEnvelopeRecord(t)
		record.ExecutionHandoff.State = state
		switch state {
		case StateOwnershipDispatching, StateOwnershipDispatched:
			record.ExecutionHandoff.OwnerSession = nil
			record.ExecutionHandoff.Orientation = nil
		case StateOwnerOrienting:
			record.ExecutionHandoff.Orientation = nil
		case StateCleanupPendingHumanDecision, StateCleanupExecuting, StateClosed, StateRecoveryRequired:
			record.ExecutionHandoff.Completion = &model.IssueOpsOwnershipCompletion{FinalHead: strings.Repeat("a", 40), CompletedAt: "2026-07-20T00:00:00Z"}
		}
		if state == StateCleanupExecuting {
			record.ExecutionHandoff.Cleanup = &model.IssueOpsExecutionHandoffCleanup{ApprovedBySession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "source"}, InventoryFingerprint: strings.Repeat("b", 64)}
		}
		if err := ValidateEnvelope(record); err != nil {
			t.Fatalf("state %s rejected: %v", state, err)
		}
	}
}

func TestOwnershipEnvelopeAllowsCancellationRecoveryWithoutCompletion(t *testing.T) {
	record := validOwnershipEnvelopeRecord(t)
	record.ExecutionHandoff.State = StateRecoveryRequired
	record.ExecutionHandoff.Cancellation = &model.IssueOpsExecutionHandoffCancellation{
		RequestedAt: "2026-07-20T00:00:00Z",
		Reason:      "stranded owner cleanup requested",
	}
	record.ExecutionHandoff.Failure = &model.IssueOpsExecutionHandoffFailure{
		Code:    "cancellation_requested",
		Message: "stranded owner cleanup requested",
		At:      "2026-07-20T00:00:00Z",
	}
	if err := ValidateEnvelope(record); err != nil {
		t.Fatalf("cancellation recovery without completion rejected: %v", err)
	}
}

func TestOwnershipEnvelopeRejectsInvalidOrientation(t *testing.T) {
	for _, mutate := range []func(*model.IssueOpsOwnershipOrientation){
		func(o *model.IssueOpsOwnershipOrientation) { o.IssueURL = "https://example.test/issues/other" },
		func(o *model.IssueOpsOwnershipOrientation) { o.PlanSHA256 = "not-a-sha" },
		func(o *model.IssueOpsOwnershipOrientation) { o.Understanding = "\n" },
		func(o *model.IssueOpsOwnershipOrientation) { o.ScopeConfirmation = strings.Repeat("x", 4097) },
		func(o *model.IssueOpsOwnershipOrientation) { o.RecordedAt = "not-a-timestamp" },
	} {
		record := validOwnershipEnvelopeRecord(t)
		mutate(record.ExecutionHandoff.Orientation)
		if err := ValidateEnvelope(record); err == nil {
			t.Fatalf("invalid ownership orientation was accepted: %#v", record.ExecutionHandoff.Orientation)
		}
	}
}

func validOwnershipEnvelopeRecord(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	worker := filepath.Join(repo+".worktrees", "demo")
	preparation := &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "prep"}
	return model.IssueOpsRecord{
		SchemaVersion: model.IssueOpsCurrentSchemaVersion, ID: "io-ownership", Repo: repo, Branch: "demo", IssueURL: "https://example.test/issues/1",
		ExecutionWorkspace: &model.IssueOpsExecutionWorkspace{State: "ready", WorkspaceEpoch: "workspace-1", Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worker, PreparationSession: preparation, BaseHead: strings.Repeat("b", 40)},
		ExecutionHandoff:   &model.IssueOpsExecutionHandoff{State: StateOwnerActive, WorkspaceEpoch: "workspace-1", WorkspaceSHA256: strings.Repeat("c", 64), OwnerSession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "owner"}, Orientation: &model.IssueOpsOwnershipOrientation{IssueURL: "https://example.test/issues/1", PlanSHA256: strings.Repeat("d", 64), Understanding: "understood", ScopeConfirmation: "scoped", RecordedAt: "2026-07-20T00:00:00Z"}},
	}
}
