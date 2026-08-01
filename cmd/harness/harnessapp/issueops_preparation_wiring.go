package harnessapp

import (
	"context"
	"time"

	"agent-harness/internal/adapter/gitworktree"
	preparationinbound "agent-harness/internal/adapter/inbound/issueopspreparation"
	"agent-harness/internal/adapter/orca"
	preparationoutbound "agent-harness/internal/adapter/outbound/issueopspreparation"
	"agent-harness/internal/adapter/provider"
	preparationapp "agent-harness/internal/application/issueopspreparation"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

type issueOpsPreparationCompositionDeps struct {
	Direct         port.ExecutionWorkspaceProvisioner
	Orca           port.ExecutionOrcaProvisioner
	ReadIssue      issueops.ExecutionIssueSnapshotReadFunc
	Now            func() time.Time
	NewOperationID func() (string, error)
}

type issueOpsExecutionCompositionDeps struct {
	Prepare   issueops.ExecutionPrepareHandler
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	ReadIssue issueops.ExecutionIssueSnapshotReadFunc
}

func productionIssueOpsExecutionDependencies() issueOpsExecutionCompositionDeps {
	orcaExecution := orca.NewExecution()
	readIssue := provider.ReadExecutionIssueSnapshot
	return issueOpsExecutionCompositionDeps{
		Prepare: newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
			Direct: gitworktree.New(), Orca: orcaExecution, ReadIssue: readIssue,
		}),
		Orca: orcaExecution, OrcaOwner: orcaExecution, ReadIssue: readIssue,
	}
}

func newIssueOpsPreparationHandler(deps issueOpsPreparationCompositionDeps) issueops.ExecutionPrepareHandler {
	return func(ctx context.Context, stateRoot string, request issueops.ExecutionPrepareRequest, invocation issueops.ExecutionPrepareInvocation) (issueops.ExecutionPrepareResult, error) {
		requestDeps := deps
		if invocation.ReadIssue != nil {
			requestDeps.ReadIssue = invocation.ReadIssue
		}
		service, err := newIssueOpsPreparationService(stateRoot, request.ID, requestDeps)
		if err != nil {
			return issueops.ExecutionPrepareResult{ID: request.ID}, err
		}
		return preparationinbound.NewHandler(service)(ctx, stateRoot, request, invocation)
	}
}

func newIssueOpsPreparationService(stateRoot, id string, deps issueOpsPreparationCompositionDeps) (*preparationapp.Service, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	repository := preparationoutbound.NewSQLiteRepositoryWithDiagnosticRedactor(database, func(context.Context) error {
		return issueops.RequireIssueOpsMutationAllowed(stateRoot)
	}, policy.RedactDiagnostic)
	direct := preparationoutbound.NewDirectWorkspace(deps.Direct)
	gateway := preparationoutbound.NewOrcaGateway(preparationoutbound.OrcaDependencies{
		Provisioner: deps.Orca,
		ValidateProbe: func(_ context.Context, request preparationcontract.ProbeRequest) (string, error) {
			return issueops.ValidateExecutionPreparationOrcaProbe(stateRoot, id, request)
		},
		HydrateLaunch: func(_ context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
			return issueops.HydrateExecutionPreparationLaunch(stateRoot, id, request)
		},
	})
	evidence := preparationoutbound.NewEvidence(preparationoutbound.EvidenceDependencies{
		Workspace: issueops.ResolveExecutionPreparationWorkspace,
		ReadOwner: func(ctx context.Context, snapshot preparationcontract.Snapshot, _ preparationcontract.Command) (preparationcontract.OwnerEvidence, error) {
			return issueops.ReadExecutionPreparationOwnerEvidence(ctx, snapshot, deps.ReadIssue)
		},
		MaterializeDirect: func(_ context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
			return issueops.MaterializeExecutionPreparationDirect(stateRoot, snapshot, receipt)
		},
		PrepareOwner: func(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
			return issueops.PrepareExecutionPreparationOwner(ctx, stateRoot, snapshot, command, intent, receipt, deps.ReadIssue)
		},
	})
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	operationID := deps.NewOperationID
	if operationID == nil {
		operationID = issueops.NewExecutionPreparationOperationID
	}
	return preparationapp.NewService(repository, preparationClock{now}, preparationOperationIDs{operationID}, direct, gateway, evidence), nil
}

type preparationClock struct{ now func() time.Time }

func (clock preparationClock) Now() time.Time { return clock.now() }

type preparationOperationIDs struct{ next func() (string, error) }

func (ids preparationOperationIDs) New() (string, error) { return ids.next() }
