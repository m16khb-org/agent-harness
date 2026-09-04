package issueopsapp

import (
	"context"

	completioninbound "issueops/internal/adapter/inbound/issueopscompletion"
	"issueops/internal/adapter/issueops"
	completionoutbound "issueops/internal/adapter/outbound/issueopscompletion"
	"issueops/internal/adapter/outbound/sqlstore"
	completionapp "issueops/internal/application/issueopscompletion"
	issueopscontract "issueops/internal/contract/issueops"
	completioncontract "issueops/internal/contract/issueopscompletion"
)

func issueOpsCompleteHandler(ctx context.Context, stateRoot string, request issueops.ExecutionCompleteRequest) (issueops.ExecutionResult, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	service := completionapp.NewService(
		completionoutbound.NewRepository(database), completionoutbound.NewEnvironment(), completionoutbound.UTCClock{},
		issueOpsCompletionProcessInspector,
	)
	return completioninbound.NewHandler(service)(ctx, stateRoot, request)
}

func issueOpsCompletionProcessInspector(_ context.Context, receipt completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error) {
	status, observed, err := issueops.InspectNativeProcessReceipt(issueopscontract.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	return status, completioncontract.ProcessReceipt{PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable}, err
}
