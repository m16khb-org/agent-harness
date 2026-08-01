package harnessapp

import (
	"context"

	completioninbound "agent-harness/internal/adapter/inbound/issueopscompletion"
	"agent-harness/internal/adapter/orca"
	completionoutbound "agent-harness/internal/adapter/outbound/issueopscompletion"
	completionapp "agent-harness/internal/application/issueopscompletion"
	completioncontract "agent-harness/internal/contract/issueopscompletion"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func issueOpsCompleteHandler(ctx context.Context, stateRoot string, request issueops.ExecutionCompleteRequest) (issueops.ExecutionResult, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionResult{ID: request.ID}, err
	}
	service := completionapp.NewService(
		completionoutbound.NewRepository(database), completionoutbound.NewEnvironment(issueOpsCompletionArtifactVerifier), completionoutbound.UTCClock{},
		issueOpsCompletionProcessInspector, completionoutbound.NewTaskSettler(orca.New().SettleTask),
	)
	return completioninbound.NewHandler(service)(ctx, stateRoot, request)
}

func issueOpsCompletionProcessInspector(_ context.Context, receipt completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error) {
	status, observed, err := issueops.InspectNativeProcessReceipt(model.NativeProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	return status, completioncontract.ProcessReceipt{PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable}, err
}

func issueOpsCompletionArtifactVerifier(record completioncontract.RecordSnapshot, requestedURL string) error {
	coreRecord := issueops.IssueOpsRecord{
		Phase: issueops.IssueOpsPhase(record.Phase), IssueURL: record.IssueURL,
		BranchPrepare: &issueops.IssueOpsBranchPrepare{BaseBranch: record.BaseBranch},
	}
	if record.Artifact != nil {
		coreRecord.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
			Provider: record.Artifact.Provider, Kind: record.Artifact.Kind, URL: record.Artifact.URL,
			Labels: append([]string(nil), record.Artifact.Labels...), Assignees: append([]string(nil), record.Artifact.Assignees...),
			VerifiedAt: record.Artifact.VerifiedAt, TargetBranch: record.Artifact.TargetBranch,
		}
	}
	return issueops.ValidateExecutionCompletionArtifact(coreRecord, requestedURL)
}
