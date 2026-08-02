package issueopspreparation

import (
	"context"
	"time"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

type Repository interface {
	Load(context.Context, string) (preparationcontract.Snapshot, error)
	EnsureRootUnclaimed(context.Context, string, string) error
	CommitDirect(context.Context, DirectCommit) (preparationcontract.Result, error)
	BeginIntent(context.Context, OrcaBegin) (IntentState, error)
	MarkInvoking(context.Context, IntentState) (IntentState, error)
	RecordFailure(context.Context, IntentState, string, error) error
	ApplyReceipt(context.Context, IntentState, preparationcontract.IntentReceipt) (IntentProgress, error)
}

type Clock interface{ Now() time.Time }

type OperationID interface{ New() (string, error) }

type DirectWorkspace interface {
	ProbeAccess(context.Context, preparationcontract.WorkspaceRequest, string) (preparationcontract.AccessResult, error)
	Prepare(context.Context, preparationcontract.WorkspaceRequest) (preparationcontract.WorkspaceReceipt, error)
}

type OrcaGateway interface {
	Probe(context.Context, preparationcontract.ProbeRequest) (preparationcontract.ProbeResult, error)
	Inspect(context.Context, preparationcontract.IntentRequest) (preparationcontract.IntentInventory, error)
	Invoke(context.Context, preparationcontract.IntentRequest) (preparationcontract.IntentReceipt, error)
}

type PreparationEvidence interface {
	Workspace(preparationcontract.Snapshot, bool) (preparationcontract.WorkspaceRequest, error)
	ReadOwner(context.Context, preparationcontract.Snapshot, preparationcontract.Command) (preparationcontract.OwnerEvidence, error)
	MaterializeDirect(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error
	PrepareOwner(context.Context, preparationcontract.Snapshot, preparationcontract.Command, preparationcontract.Intent, preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error)
}

type DirectCommit struct {
	Snapshot      preparationcontract.Snapshot
	Command       preparationcontract.Command
	Workspace     preparationcontract.WorkspaceReceipt
	RequestedMode string
	FallbackCode  string
	LinkedAt      string
	ClaimedAt     string
}

type OrcaBegin struct {
	Snapshot    preparationcontract.Snapshot
	Command     preparationcontract.Command
	Workspace   preparationcontract.WorkspaceRequest
	Probe       preparationcontract.ProbeRequest
	Owner       preparationcontract.OwnerEvidence
	OperationID string
	StartedAt   string
}

type IntentState struct {
	Snapshot       preparationcontract.Snapshot
	Command        preparationcontract.Command
	Intent         preparationcontract.Intent
	IntentRaw      []byte
	Owner          preparationcontract.OwnerEvidence
	OwnerArtifacts preparationcontract.OwnerArtifacts
	FailureAt      string
	Pending        bool
}

type IntentProgress struct {
	State   IntentState
	Result  preparationcontract.Result
	Pending bool
}
