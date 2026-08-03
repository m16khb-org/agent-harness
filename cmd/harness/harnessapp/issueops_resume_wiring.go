package harnessapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"

	leaseinbound "agent-harness/internal/adapter/inbound/issueopslease"
	"agent-harness/internal/adapter/orca"
	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/outbound/sqlstore"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func issueOpsResumeHandler(ctx context.Context, stateRoot string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
	orcaExecution := orca.NewExecution()
	service, err := newIssueOpsResumeService(stateRoot, orcaExecution, orcaExecution)
	if err != nil {
		return issueops.ExecutionResumeResult{ID: request.ID}, err
	}
	return leaseinbound.NewResumeHandler(service)(ctx, stateRoot, request)
}

func newIssueOpsResumeService(stateRoot string, provisioner port.ExecutionOrcaProvisioner, owner port.ExecutionOrcaOwnerInspector) (*leaseapp.ResumeService, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
	if err != nil {
		return nil, err
	}
	effects := &coreResumeEffects{stateRoot: stateRoot, provisioner: provisioner, owner: owner, now: time.Now}
	repository := leaseoutbound.NewResumeRepository(db, effects)
	return leaseapp.NewResumeService(
		fence,
		repository,
		leaseoutbound.NewResumeArtifacts(effects.readArtifacts),
		leaseoutbound.NewResumeOwnerInventory(effects.observeOwner),
		leaseoutbound.NewResumeStageExecutor(effects.inspectStage, effects.invokeStage),
		resumeOperationIDs{},
		leaseoutbound.InspectNativeProcess,
		leaseoutbound.FilesystemPathMatcher{},
	), nil
}

type resumeOperationIDs struct{}

func (resumeOperationIDs) New() (string, error) { return issueops.NewExecutionResumeOperationID() }

type coreResumeEffects struct {
	stateRoot   string
	provisioner port.ExecutionOrcaProvisioner
	owner       port.ExecutionOrcaOwnerInspector
	now         func() time.Time
}

func (e *coreResumeEffects) Begin(_ context.Context, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts, plan leasedomain.ResumePlan, operationID string) (leaseoutbound.ResumeEffectState, error) {
	coreRecord, err := resumeCoreRecord(record)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	state, err := issueops.BeginExecutionResumeIntent(e.stateRoot, coreRecord, raw, resumeCoreArtifacts(artifacts), plan.RuntimeID, plan.ReusedTerminalPTYID, operationID, e.now)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeEffectStateFromCore(state)
}

func (e *coreResumeEffects) Read(_ context.Context, id, operationID string) (leaseoutbound.ResumeEffectState, error) {
	state, err := issueops.ReadExecutionResumeIntent(e.stateRoot, id, operationID)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeEffectStateFromCore(state)
}

func (e *coreResumeEffects) MarkInvoking(_ context.Context, state leaseoutbound.ResumeEffectState) (leaseoutbound.ResumeEffectState, error) {
	coreState, err := resumeCoreIntentState(state)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	next, err := issueops.MarkExecutionResumeIntentInvoking(e.stateRoot, coreState)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeEffectStateFromCore(next)
}

func (e *coreResumeEffects) RecordFailure(_ context.Context, state leaseoutbound.ResumeEffectState, invocation string, cause error) error {
	coreState, err := resumeCoreIntentState(state)
	if err != nil {
		return err
	}
	return issueops.RecordExecutionResumeIntentFailure(e.stateRoot, coreState, invocation, cause, e.now)
}

func (e *coreResumeEffects) ApplyReceipt(ctx context.Context, state leaseoutbound.ResumeEffectState, receipt leasecontract.ResumeStageReceipt) (leaseoutbound.ResumeEffectState, error) {
	coreState, err := resumeCoreIntentState(state)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	next, err := issueops.ApplyExecutionResumeIntentReceipt(ctx, e.stateRoot, coreState, resumePortReceipt(state.Stage, receipt), e.now)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeEffectStateFromCore(next)
}

func (e *coreResumeEffects) readArtifacts(_ context.Context, record leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
	coreRecord, err := resumeCoreRecord(record)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	artifacts, err := issueops.ReadExecutionResumeArtifacts(coreRecord)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	return leasecontract.ResumeArtifacts{ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256}, nil
}

func (e *coreResumeEffects) observeOwner(ctx context.Context, record leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
	if e.owner == nil || record.Execution == nil || record.Execution.Orca == nil {
		return leasedomain.ResumeInventory{}, false, fmt.Errorf("resume owner inspector is required")
	}
	binding := record.Execution.Orca
	inventory, err := e.owner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID, AllowRuntimeRollover: true})
	if err != nil {
		return leasedomain.ResumeInventory{}, false, fmt.Errorf("inspect previous Orca owner: %w", err)
	}
	coreRecord, err := resumeCoreRecord(record)
	if err != nil {
		return leasedomain.ResumeInventory{}, false, err
	}
	if err := issueops.ValidateExecutionResumeOwner(coreRecord, inventory); err != nil {
		return leasedomain.ResumeInventory{}, false, err
	}
	return leasedomain.ResumeInventory{RuntimeID: inventory.RuntimeID, TerminalLive: inventory.TerminalLive, TaskLive: inventory.TaskLive, TerminalID: inventory.TerminalID}, true, nil
}

func (e *coreResumeEffects) inspectStage(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	if e.provisioner == nil {
		return leasecontract.ResumeStageInventory{}, fmt.Errorf("resume Orca provisioner is required")
	}
	coreState, err := resumeCoreIntentState(resumeEffectState(intent))
	if err != nil {
		return leasecontract.ResumeStageInventory{}, err
	}
	request, err := issueops.ExecutionResumeIntentRequest(coreState)
	if err != nil {
		return leasecontract.ResumeStageInventory{}, err
	}
	inventory, err := e.provisioner.InspectIntent(ctx, request)
	if err != nil {
		return leasecontract.ResumeStageInventory{}, err
	}
	result := leasecontract.ResumeStageInventory{AuthoritativeZero: inventory.AuthoritativeZero}
	for _, candidate := range inventory.Candidates {
		result.Candidates = append(result.Candidates, leasecontract.ResumeStageReceipt{TerminalPTYID: candidate.TerminalPTYID, RunID: candidate.RunID, RunBound: candidate.RunBound, TaskID: candidate.TaskID, DispatchID: candidate.DispatchID})
	}
	return result, nil
}

func (e *coreResumeEffects) invokeStage(ctx context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
	if e.provisioner == nil {
		return leasecontract.ResumeStageReceipt{}, fmt.Errorf("resume Orca provisioner is required")
	}
	coreState, err := resumeCoreIntentState(resumeEffectState(intent))
	if err != nil {
		return leasecontract.ResumeStageReceipt{}, err
	}
	request, err := issueops.ExecutionResumeIntentRequest(coreState)
	if err != nil {
		return leasecontract.ResumeStageReceipt{}, err
	}
	receipt, err := e.provisioner.InvokeIntent(ctx, request)
	if err != nil {
		return leasecontract.ResumeStageReceipt{}, err
	}
	return leasecontract.ResumeStageReceipt{TerminalPTYID: receipt.TerminalPTYID, RunID: receipt.RunID, RunBound: receipt.RunBound, TaskID: receipt.TaskID, DispatchID: receipt.DispatchID}, nil
}

func resumeCoreRecord(record leasecontract.Record) (issueopscontract.IssueOpsRecord, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	var result issueopscontract.IssueOpsRecord
	if err := json.Unmarshal(data, &result); err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	return result, nil
}

func resumeCoreArtifacts(artifacts leasecontract.ResumeArtifacts) issueops.ExecutionResumeArtifactsReceipt {
	return issueops.ExecutionResumeArtifactsReceipt{ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256}
}

func resumeEffectStateFromCore(state issueops.ExecutionResumeIntentState) (leaseoutbound.ResumeEffectState, error) {
	data, err := json.Marshal(state.Record)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	record, err := leasecontract.Decode(state.Record.ID, data)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return leaseoutbound.ResumeEffectState{Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...), OperationID: state.OperationID, Stage: string(state.Stage), InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts, Pending: state.Pending}, nil
}

func resumeCoreIntentState(state leaseoutbound.ResumeEffectState) (issueops.ExecutionResumeIntentState, error) {
	record, err := resumeCoreRecord(state.Record)
	if err != nil {
		return issueops.ExecutionResumeIntentState{}, err
	}
	return issueops.ExecutionResumeIntentState{Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...), OperationID: state.OperationID, Stage: port.ExecutionOrcaIntentStage(state.Stage), InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts, Pending: state.Pending}, nil
}

func resumeEffectState(intent leaseapp.ResumeIntentState) leaseoutbound.ResumeEffectState {
	return leaseoutbound.ResumeEffectState{Record: intent.Progress.Record.Stable, RecordRaw: append([]byte(nil), intent.RecordRaw...), IntentRaw: append([]byte(nil), intent.IntentRaw...), OperationID: intent.OperationID, Stage: intent.Stage, InvocationState: intent.InvocationState, InvocationAttempts: intent.InvocationAttempts, Pending: intent.Progress.Pending}
}

func resumePortReceipt(stage string, receipt leasecontract.ResumeStageReceipt) port.ExecutionOrcaIntentReceipt {
	result := port.ExecutionOrcaIntentReceipt{}
	switch port.ExecutionOrcaIntentStage(stage) {
	case port.ExecutionOrcaIntentTerminal:
		result.TerminalPTYID = receipt.TerminalPTYID
	case port.ExecutionOrcaIntentRun:
		result.RunID = receipt.RunID
	case port.ExecutionOrcaIntentRunBind:
		result.RunID, result.RunBound = receipt.RunID, receipt.RunBound
	case port.ExecutionOrcaIntentTask:
		result.TaskID = receipt.TaskID
	case port.ExecutionOrcaIntentDispatch:
		result.TaskID, result.DispatchID = receipt.TaskID, receipt.DispatchID
	}
	return result
}
