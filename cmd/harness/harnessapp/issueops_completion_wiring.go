package harnessapp

import (
	"context"

	completioninbound "agent-harness/internal/adapter/inbound/issueopscompletion"
	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/orca"
	completionoutbound "agent-harness/internal/adapter/outbound/issueopscompletion"
	"agent-harness/internal/adapter/outbound/sqlstore"
	completionapp "agent-harness/internal/application/issueopscompletion"
	issueopscontract "agent-harness/internal/contract/issueops"
	completioncontract "agent-harness/internal/contract/issueopscompletion"
)

func issueOpsCompleteHandler(ctx context.Context, stateRoot string, request issueops.ExecutionCompleteRequest) (issueops.ExecutionResult, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	service := completionapp.NewService(
		completionoutbound.NewRepository(database), completionoutbound.NewEnvironment(), completionoutbound.UTCClock{},
		issueOpsCompletionProcessInspector, completionoutbound.NewTaskSettler(orca.New().SettleTask),
	)
	return completioninbound.NewHandler(service)(ctx, stateRoot, request)
}

func issueOpsCompletionProcessInspector(_ context.Context, receipt completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error) {
	status, observed, err := issueops.InspectNativeProcessReceipt(issueopscontract.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	return status, completioncontract.ProcessReceipt{PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable}, err
}
