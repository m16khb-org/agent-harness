package issueopspreparation

import (
	"context"
	"strings"
	"testing"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

func TestEvidenceCallbacksAndFailClosedDependencies(t *testing.T) {
	snapshot := preparationcontract.Snapshot{}
	receipt := preparationcontract.WorkspaceReceipt{Root: "/worktree"}
	adapter := NewEvidence(EvidenceDependencies{
		Workspace: func(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error) {
			return preparationcontract.WorkspaceRequest{Root: "/worktree"}, nil
		},
		MaterializeDirect: func(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error {
			return nil
		},
	})
	if workspace, err := adapter.Workspace(snapshot, true); err != nil || workspace.Root != receipt.Root {
		t.Fatalf("workspace=%+v err=%v", workspace, err)
	}
	if err := adapter.MaterializeDirect(context.Background(), snapshot, receipt); err != nil {
		t.Fatal(err)
	}

	empty := NewEvidence(EvidenceDependencies{})
	if _, err := empty.Workspace(snapshot, false); err == nil || !strings.Contains(err.Error(), "workspace resolver") {
		t.Fatalf("workspace err=%v", err)
	}
	if err := empty.MaterializeDirect(context.Background(), snapshot, receipt); err == nil || !strings.Contains(err.Error(), "artifact materializer") {
		t.Fatalf("materialize err=%v", err)
	}
	if _, err := empty.ReadOwner(context.Background(), snapshot, preparationcontract.Command{}); err == nil || !strings.Contains(err.Error(), "issue snapshot reader") {
		t.Fatalf("read owner err=%v", err)
	}
	if _, err := empty.PrepareOwner(context.Background(), snapshot, preparationcontract.Command{}, preparationcontract.Intent{}, preparationcontract.IntentReceipt{}); err == nil || !strings.Contains(err.Error(), "owner artifact preparer") {
		t.Fatalf("prepare owner err=%v", err)
	}
}
