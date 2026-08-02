package harnessapp

import (
	"context"
	"encoding/json"
	"fmt"

	leaseinbound "agent-harness/internal/adapter/inbound/issueopslease"
	"agent-harness/internal/adapter/orca"
	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/provider"
	leaseapp "agent-harness/internal/application/issueopslease"
	model "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func issueOpsReseedHandler(ctx context.Context, stateRoot string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
	return issueOpsReseedHandlerWithOwner(ctx, stateRoot, request, orca.NewExecution())
}

func issueOpsReseedHandlerWithOwner(ctx context.Context, stateRoot string, request issueops.ExecutionReseedRequest, owner port.ExecutionOrcaOwnerInspector) (issueops.ExecutionReplaceResult, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.ExecutionReplaceResult{ID: request.ID, Action: issueops.ExecutionReplaceReseed}, err
	}
	fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		return issueops.ExecutionReplaceResult{ID: request.ID, Action: issueops.ExecutionReplaceReseed}, err
	}
	inventory := leaseoutbound.NewReseedInventory(owner, leaseoutbound.InspectNativeProcess)
	readIssue := request.ReadIssue
	if readIssue == nil {
		readIssue = provider.ReadExecutionIssueSnapshot
	}
	artifacts := leaseoutbound.NewReseedArtifacts(func(ctx context.Context, record leasecontract.Record) (leasecontract.ReseedReceipt, error) {
		execution, err := issueOpsReseedExecution(record)
		if err != nil {
			return leasecontract.ReseedReceipt{}, err
		}
		prepared, err := issueops.PrepareExecutionReseedOwnerArtifacts(ctx, stateRoot, record.ID, execution, readIssue)
		if err != nil {
			return leasecontract.ReseedReceipt{}, err
		}
		return leasecontract.ReseedReceipt{IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketPath: prepared.ContextPacketPath, ContextPacketSHA256: prepared.ContextPacketSHA256, OwnerPromptPath: prepared.OwnerPromptPath, OwnerPromptSHA256: prepared.OwnerPromptSHA256}, nil
	})
	service := leaseapp.NewReseedService(fence, leaseoutbound.NewReseedRepository(db), inventory, artifacts, leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, leaseoutbound.FilesystemPathMatcher{})
	return leaseinbound.NewReseedHandler(service)(ctx, stateRoot, request)
}

func issueOpsReseedExecution(record leasecontract.Record) (model.Execution, error) {
	if record.Execution == nil {
		return model.Execution{}, fmt.Errorf("reseed owner artifacts require an execution")
	}
	data, err := json.Marshal(record.Execution)
	if err != nil {
		return model.Execution{}, err
	}
	var execution model.Execution
	if err := json.Unmarshal(data, &execution); err != nil {
		return model.Execution{}, err
	}
	return execution, nil
}
