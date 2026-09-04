package issueopspreparation

import (
	"context"
	"fmt"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type EvidenceDependencies struct {
	Workspace         func(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	ReadOwner         func(context.Context, preparationcontract.Snapshot, preparationcontract.Command) (preparationcontract.OwnerEvidence, error)
	MaterializeDirect func(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
	PrepareOwner      func(context.Context, preparationcontract.Snapshot, preparationcontract.Command, preparationcontract.Intent, preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error)
}

type Evidence struct {
	dependencies EvidenceDependencies
}

func NewEvidence(dependencies EvidenceDependencies) *Evidence {
	return &Evidence{dependencies: dependencies}
}

func (evidence *Evidence) Workspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	if evidence.dependencies.Workspace == nil {
		return preparationcontract.WorkspaceRequest{}, fmt.Errorf("preparation workspace resolver is unavailable")
	}
	return evidence.dependencies.Workspace(snapshot.Clone(), confirm)
}

func (evidence *Evidence) ReadOwner(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command) (preparationcontract.OwnerEvidence, error) {
	if evidence.dependencies.ReadOwner == nil {
		return preparationcontract.OwnerEvidence{}, fmt.Errorf("issue snapshot reader is unavailable")
	}
	return evidence.dependencies.ReadOwner(ctx, snapshot.Clone(), command.Clone())
}

func (evidence *Evidence) MaterializeDirect(ctx context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	if evidence.dependencies.MaterializeDirect == nil {
		return fmt.Errorf("staged artifact materializer is unavailable")
	}
	return evidence.dependencies.MaterializeDirect(ctx, snapshot.Clone(), receipt)
}

func (evidence *Evidence) PrepareOwner(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
	if evidence.dependencies.PrepareOwner == nil {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("owner artifact preparer is unavailable")
	}
	return evidence.dependencies.PrepareOwner(ctx, snapshot.Clone(), command.Clone(), intent, receipt)
}
