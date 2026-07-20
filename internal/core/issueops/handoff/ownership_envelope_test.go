package handoff

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestOwnershipEnvelopeRejectsV1FieldsInProtocolV2(t *testing.T) {
	record := validOwnershipEnvelopeRecord(t)
	if err := ValidateEnvelope(record); err != nil {
		t.Fatalf("valid ownership envelope rejected: %v", err)
	}
	record.ExecutionHandoff.WorkerSession = &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "legacy-worker"}
	if err := ValidateEnvelope(record); err == nil || !strings.Contains(err.Error(), "protocol-v1") {
		t.Fatalf("protocol-v2 envelope accepted worker_session: %v", err)
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
		if err := ValidateEnvelope(record); err != nil {
			t.Fatalf("state %s rejected: %v", state, err)
		}
	}
}

func validOwnershipEnvelopeRecord(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	worker := filepath.Join(repo+".worktrees", "demo")
	preparation := &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "prep"}
	return model.IssueOpsRecord{
		SchemaVersion: model.IssueOpsCurrentSchemaVersion, ID: "io-ownership", Repo: repo, Branch: "demo",
		ExecutionWorkspace: &model.IssueOpsExecutionWorkspace{State: "ready", WorkspaceEpoch: "workspace-1", Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worker, PreparationSession: preparation, BaseHead: strings.Repeat("b", 40)},
		ExecutionHandoff:   &model.IssueOpsExecutionHandoff{ProtocolVersion: OwnershipTransferProtocolVersion, State: StateOwnerActive, WorkspaceEpoch: "workspace-1", WorkspaceSHA256: strings.Repeat("c", 64), OwnerSession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "owner"}, Orientation: &model.IssueOpsOwnershipOrientation{IssueURL: "https://example.test/issues/1", PlanSHA256: strings.Repeat("d", 64), Understanding: "understood", ScopeConfirmation: "scoped", RecordedAt: "2026-07-20T00:00:00Z"}},
	}
}
