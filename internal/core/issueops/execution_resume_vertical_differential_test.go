package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

type resumeDifferentialObservation struct {
	ResultJSON       []byte
	PublicErrorCode  string
	PublicError      string
	RecordBytes      []byte
	IntentBytes      []byte
	Checkpoints      []resumeDifferentialCheckpoint
	Stages           []port.ExecutionOrcaIntentStage
	MutationAttempts int
}

type resumeDifferentialCheckpoint struct {
	Stage       port.ExecutionOrcaIntentStage
	RecordBytes []byte
	IntentBytes []byte
}

type resumeDifferentialCase struct {
	name         string
	owner        port.ExecutionOrcaOwnerInventory
	request      func(ExecutionResumeRequest) ExecutionResumeRequest
	mutateRecord func(*IssueOpsRecord)
	mutateFiles  func(t *testing.T, record IssueOpsRecord)
	fixture      func(t *testing.T, mutate func(*IssueOpsRecord)) (string, IssueOpsRecord)
	inspectErr   error
	inventory    *port.ExecutionOrcaIntentInventory
	failureStage port.ExecutionOrcaIntentStage
	invokeErr    error
}

func TestResumeVerticalDifferential(t *testing.T) {
	cases := []resumeDifferentialCase{
		{name: "same_generation_live_task", mutateRecord: func(record *IssueOpsRecord) {
			record.Execution.Orca.LeaseGeneration = record.Execution.Lease.Generation
		}, owner: port.ExecutionOrcaOwnerInventory{TerminalLive: true, TaskLive: true}},
		{name: "reuse_live_terminal", owner: port.ExecutionOrcaOwnerInventory{TerminalLive: true}},
		{name: "fresh_terminal_run_task_dispatch"},
		{name: "task_without_terminal", owner: port.ExecutionOrcaOwnerInventory{TaskLive: true}},
		{name: "other_generation_live_task", owner: port.ExecutionOrcaOwnerInventory{TerminalLive: true, TaskLive: true}},
		{name: "runtime_rollover_rejected", owner: port.ExecutionOrcaOwnerInventory{RuntimeID: "other-runtime"}},
		{name: "terminal_identity_drift", owner: port.ExecutionOrcaOwnerInventory{TerminalLive: true, TerminalID: "pty-other"}},
		{name: "confirm_actor_cwd_generation_lease_denials/confirm", request: func(request ExecutionResumeRequest) ExecutionResumeRequest { request.Confirm = false; return request }},
		{name: "confirm_actor_cwd_generation_lease_denials/confirm_before_invalid_actor", request: func(request ExecutionResumeRequest) ExecutionResumeRequest {
			request.Confirm = false
			request.Actor = model.NativeActor{}
			return request
		}},
		{name: "confirm_actor_cwd_generation_lease_denials/cwd", request: func(request ExecutionResumeRequest) ExecutionResumeRequest {
			request.CWD = "/not-canonical"
			return request
		}},
		{name: "confirm_actor_cwd_generation_lease_denials/generation", request: func(request ExecutionResumeRequest) ExecutionResumeRequest {
			request.ExpectedGeneration++
			return request
		}},
		{name: "confirm_actor_cwd_generation_lease_denials/lease", mutateRecord: func(record *IssueOpsRecord) {
			record.Execution.Lease.Status = model.LeaseStatusReleased
			record.Execution.Lease.ClaimTokenSHA256 = ""
		}},
		{name: "token_packet_prompt_manifest_tamper/token", mutateFiles: resumeDifferentialTamperToken},
		{name: "token_packet_prompt_manifest_tamper/packet", mutateFiles: resumeDifferentialTamperPacket},
		{name: "token_packet_prompt_manifest_tamper/prompt", mutateFiles: resumeDifferentialTamperPrompt},
		{name: "token_packet_prompt_manifest_tamper/manifest", fixture: resumeDifferentialManifestFixture, mutateFiles: resumeDifferentialTamperManifest},
		{name: "inspect_error", inspectErr: errors.New("inventory unavailable")},
		{name: "non_authoritative_zero", inventory: &port.ExecutionOrcaIntentInventory{}},
		{name: "terminal_invoked_unknown_ambiguous", failureStage: port.ExecutionOrcaIntentTerminal},
		{name: "run_invoked_unknown_ambiguous", failureStage: port.ExecutionOrcaIntentRun},
		{name: "run_bind_invoked_unknown_ambiguous", failureStage: port.ExecutionOrcaIntentRunBind},
		{name: "task_invoked_unknown_ambiguous", failureStage: port.ExecutionOrcaIntentTask},
		{name: "dispatch_invoked_unknown_ambiguous", failureStage: port.ExecutionOrcaIntentDispatch},
		{name: "terminal_not_invoked_error", failureStage: port.ExecutionOrcaIntentTerminal, invokeErr: &port.OrcaError{Code: "transport", Invoked: false}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := resumeDifferentialFixture
			if tt.fixture != nil {
				fixture = tt.fixture
			}
			legacyRoot, record := fixture(t, tt.mutateRecord)
			if tt.mutateFiles != nil {
				tt.mutateFiles(t, record)
			}
			verticalRoot := resumeDifferentialClone(t, legacyRoot, record.ID)
			request := resumeRequest(record)
			if tt.request != nil {
				request = tt.request(request)
			}
			if tt.owner.TerminalLive && tt.owner.TerminalID == "" {
				tt.owner.TerminalID = record.Execution.Orca.TerminalPTYID
			}
			legacy := runLegacyResumeDifferential(t, legacyRoot, record, request, tt.owner, tt)
			vertical := runVerticalResumeDifferential(t, verticalRoot, record.ID, request, tt.owner, tt)
			assertResumeDifferentialEqual(t, legacy, vertical)
			if tt.failureStage != "" {
				assertResumeDifferentialSingleAttempt(t, legacy, tt.failureStage)
				assertResumeDifferentialSingleAttempt(t, vertical, tt.failureStage)
			}
			if tt.inspectErr != nil || tt.inventory != nil && !tt.inventory.AuthoritativeZero {
				if legacy.MutationAttempts != 0 || vertical.MutationAttempts != 0 {
					t.Fatalf("inventory failure invoked a mutation: legacy=%s vertical=%s", resumeDifferentialSummary(legacy), resumeDifferentialSummary(vertical))
				}
			}
			if tt.name == "fresh_terminal_run_task_dispatch" {
				assertResumeDifferentialStageCheckpoints(t, legacy)
				assertResumeDifferentialStageCheckpoints(t, vertical)
			}
		})
	}
}

func TestResumeVerticalDifferentialLegacyReconcileCompletesVerticalPending(t *testing.T) {
	stateRoot, record := resumeDifferentialFixture(t, nil)
	vertical := runVerticalResumeDifferential(t, stateRoot, record.ID, resumeRequest(record), port.ExecutionOrcaOwnerInventory{}, resumeDifferentialCase{failureStage: port.ExecutionOrcaIntentDispatch})
	if !strings.Contains(vertical.PublicError, "requires execution reconcile") {
		t.Fatalf("vertical dispatch ambiguity=%#v", vertical)
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution.Pending == nil || pending.Execution.Pending.Kind != "dispatch" {
		t.Fatalf("vertical pending=%#v", pending.Execution)
	}
	fake := &executionOrcaFake{inspect: func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TaskID: request.TaskID, DispatchID: "dispatch-resume"}}}, nil
	}}
	reconciled, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{ID: record.ID, Confirm: true, Actor: executionActor("codex", "vertical-reconciler"), CWD: record.Execution.Workspace.Root}, ExecutionReconcileDependencies{Orca: fake})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Execution.Pending != nil || reconciled.Execution.Lease != record.Execution.Lease || reconciled.Execution.Orca.LeaseGeneration != record.Execution.Lease.Generation {
		t.Fatalf("reconciled vertical pending=%#v", reconciled.Execution)
	}
}

func TestResumeVerticalDifferentialReceiptCASDrift(t *testing.T) {
	stateRoot, record := resumeDifferentialFixture(t, nil)
	artifacts, err := ReadExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatal(err)
	}
	state, err := BeginExecutionResumeIntent(stateRoot, record, rawIssueOpsRow(t, stateRoot, record.ID), artifacts, record.Execution.Orca.RuntimeID, "", strings.Repeat("e", 32), resumeDifferentialClock)
	if err != nil {
		t.Fatal(err)
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.UpdatedAt += "-drift"
	if _, err := writeIssueOps(stateRoot, current); err != nil {
		t.Fatal(err)
	}
	beforeRecord, beforeIntent := rawIssueOpsRow(t, stateRoot, record.ID), rawExternalIntentRow(t, stateRoot, state.OperationID)
	_, err = ApplyExecutionResumeIntentReceipt(context.Background(), stateRoot, state, port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-resume"}, resumeDifferentialClock)
	if err == nil || !strings.Contains(err.Error(), "stale raw record snapshot") {
		t.Fatalf("receipt drift error=%v", err)
	}
	if got := rawIssueOpsRow(t, stateRoot, record.ID); !bytes.Equal(got, beforeRecord) {
		t.Fatal("receipt drift changed record bytes")
	}
	if got := rawExternalIntentRow(t, stateRoot, state.OperationID); !bytes.Equal(got, beforeIntent) {
		t.Fatal("receipt drift changed external intent bytes")
	}
}

func resumeDifferentialFixture(t *testing.T, mutate func(*IssueOpsRecord)) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record, _ := reseededOrcaCycle(t)
	if mutate != nil {
		mutate(&record)
		if _, err := writeIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
	}
	return stateRoot, record
}

func resumeDifferentialManifestFixture(t *testing.T, mutate func(*IssueOpsRecord)) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record, _ := reseededOrcaCycleWithArtifacts(t, map[string]string{"plan": "# sealed plan\n"})
	if mutate != nil {
		mutate(&record)
		if _, err := writeIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
	}
	return stateRoot, record
}

func resumeDifferentialClone(t *testing.T, sourceRoot, id string) string {
	t.Helper()
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []sqlstore.Mutation{{Bucket: issueOpsBucket, ID: id, Data: rawIssueOpsRow(t, sourceRoot, id)}}); err != nil {
		t.Fatal(err)
	}
	return stateRoot
}

func runLegacyResumeDifferential(t *testing.T, stateRoot string, record IssueOpsRecord, request ExecutionResumeRequest, owner port.ExecutionOrcaOwnerInventory, testCase resumeDifferentialCase) resumeDifferentialObservation {
	t.Helper()
	var stages []port.ExecutionOrcaIntentStage
	var checkpoints []resumeDifferentialCheckpoint
	fake := resumeOrcaFake(t, &stages)
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		checkpoints = append(checkpoints, resumeDifferentialCheckpointAt(t, stateRoot, record.ID, strings.Repeat("a", 32), request.Stage))
		if testCase.inspectErr != nil {
			return port.ExecutionOrcaIntentInventory{}, testCase.inspectErr
		}
		if testCase.inventory != nil {
			return *testCase.inventory, nil
		}
		return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
	}
	if testCase.failureStage != "" {
		invoke := fake.invoke
		fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
			if request.Stage == testCase.failureStage {
				stages = append(stages, request.Stage)
				if testCase.invokeErr != nil {
					return port.ExecutionOrcaIntentReceipt{}, testCase.invokeErr
				}
				return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
			}
			return invoke(request)
		}
	}
	result, err := ResumeExecutionWithDependencies(context.Background(), stateRoot, request, ExecutionResumeDependencies{Orca: fake, OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: owner}, Now: resumeDifferentialClock, OperationID: strings.Repeat("a", 32)})
	return resumeDifferentialObserve(t, stateRoot, record.ID, strings.Repeat("a", 32), result, err, stages, checkpoints)
}

func runVerticalResumeDifferential(t *testing.T, stateRoot, id string, request ExecutionResumeRequest, owner port.ExecutionOrcaOwnerInventory, testCase resumeDifferentialCase) resumeDifferentialObservation {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	checkpoints := []resumeDifferentialCheckpoint{}
	effects := resumeDifferentialEffects{stateRoot: stateRoot, checkpoints: &checkpoints, t: t}
	stages := []port.ExecutionOrcaIntentStage{}
	service := leaseapp.NewResumeService(
		resumeDifferentialFence{},
		leaseoutbound.NewResumeRepository(db, effects),
		leaseoutbound.NewResumeArtifacts(resumeDifferentialArtifacts),
		leaseoutbound.NewResumeOwnerInventory(func(_ context.Context, record leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
			coreRecord, err := resumeDifferentialCoreRecord(record)
			if err != nil {
				return leasedomain.ResumeInventory{}, false, err
			}
			if err := ValidateExecutionResumeOwner(coreRecord, owner); err != nil {
				return leasedomain.ResumeInventory{}, false, err
			}
			return leasedomain.ResumeInventory{RuntimeID: owner.RuntimeID, TerminalLive: owner.TerminalLive, TaskLive: owner.TaskLive, TerminalID: owner.TerminalID}, true, nil
		}),
		leaseoutbound.NewResumeStageExecutor(
			func(context.Context, leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
				if testCase.inspectErr != nil {
					return leasecontract.ResumeStageInventory{}, testCase.inspectErr
				}
				if testCase.inventory != nil {
					return leasecontract.ResumeStageInventory{AuthoritativeZero: testCase.inventory.AuthoritativeZero}, nil
				}
				return leasecontract.ResumeStageInventory{AuthoritativeZero: true}, nil
			},
			func(_ context.Context, intent leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
				stage := port.ExecutionOrcaIntentStage(intent.Stage)
				stages = append(stages, stage)
				if stage == testCase.failureStage {
					if testCase.invokeErr != nil {
						return leasecontract.ResumeStageReceipt{}, testCase.invokeErr
					}
					return leasecontract.ResumeStageReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
				}
				switch stage {
				case port.ExecutionOrcaIntentTerminal:
					return leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-resume"}, nil
				case port.ExecutionOrcaIntentRun:
					return leasecontract.ResumeStageReceipt{RunID: "run-resume"}, nil
				case port.ExecutionOrcaIntentRunBind:
					return leasecontract.ResumeStageReceipt{RunID: "run-resume", RunBound: true}, nil
				case port.ExecutionOrcaIntentTask:
					return leasecontract.ResumeStageReceipt{TaskID: "task-resume"}, nil
				case port.ExecutionOrcaIntentDispatch:
					return leasecontract.ResumeStageReceipt{TaskID: "task-resume", DispatchID: "dispatch-resume"}, nil
				default:
					return leasecontract.ResumeStageReceipt{}, fmt.Errorf("unexpected resume stage %s", stage)
				}
			},
		),
		resumeDifferentialOperationIDs{},
		func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", receipt, nil
		},
		resumeDifferentialPathMatcher{},
	)
	process := leasedomain.ProcessReceipt{PID: 7, StartedAt: "2026-07-31T02:20:00Z", Executable: "codex"}
	result, err := service.Resume(context.Background(), leaseapp.ResumeRequest{ID: id, ExpectedGeneration: request.ExpectedGeneration, Actor: leasedomain.Actor{Host: request.Actor.Host, SessionID: request.Actor.SessionID, Process: &process}, Ancestry: []leasedomain.ProcessReceipt{process}, CWD: request.CWD, Confirm: request.Confirm})
	if err != nil {
		return resumeDifferentialObserve(t, stateRoot, id, strings.Repeat("a", 32), ExecutionResumeResult{}, resumeDifferentialPublicError(err), stages, checkpoints)
	}
	data, marshalErr := json.Marshal(result.Receipt.Execution)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var execution model.Execution
	if unmarshalErr := json.Unmarshal(data, &execution); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	artifacts := result.Receipt.Artifacts
	coreResult := ExecutionResumeResult{OK: result.OK, ID: result.ID, Execution: execution, ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256, NextCommand: ExecutionResumeNextCommand(result.ID, execution.Lease.Generation, artifacts.ClaimTokenPath, artifacts.IssueBodySHA256, artifacts.ContextPacketSHA256)}
	return resumeDifferentialObserve(t, stateRoot, id, strings.Repeat("a", 32), coreResult, nil, stages, checkpoints)
}

func resumeDifferentialPublicError(err error) error {
	if leasedomain.DenyCodeOf(err) == "" {
		return err
	}
	if cause := errors.Unwrap(err); cause != nil {
		return cause
	}
	return err
}

func resumeDifferentialObserve(t *testing.T, stateRoot, id, operationID string, result ExecutionResumeResult, err error, stages []port.ExecutionOrcaIntentStage, checkpoints []resumeDifferentialCheckpoint) resumeDifferentialObservation {
	t.Helper()
	observation := resumeDifferentialObservation{Stages: append([]port.ExecutionOrcaIntentStage(nil), stages...), Checkpoints: append([]resumeDifferentialCheckpoint(nil), checkpoints...), MutationAttempts: len(stages)}
	if err != nil {
		observation.PublicError = err.Error()
		observation.PublicErrorCode = string(leasedomain.DenyCodeOf(err))
	} else {
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		observation.ResultJSON = data
	}
	observation.RecordBytes = rawIssueOpsRow(t, stateRoot, id)
	db, openErr := sqlstore.Open(stateRoot)
	if openErr != nil {
		t.Fatal(openErr)
	}
	intent, ok, getErr := db.Get(externalIntentBucket, operationID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if ok {
		observation.IntentBytes = append([]byte(nil), intent...)
	}
	return observation
}

func assertResumeDifferentialEqual(t *testing.T, legacy, vertical resumeDifferentialObservation) {
	t.Helper()
	if legacy.PublicError != vertical.PublicError || legacy.PublicErrorCode != vertical.PublicErrorCode || !bytes.Equal(legacy.ResultJSON, vertical.ResultJSON) || !bytes.Equal(legacy.RecordBytes, vertical.RecordBytes) || !bytes.Equal(legacy.IntentBytes, vertical.IntentBytes) || !resumeDifferentialCheckpointsEqual(legacy.Checkpoints, vertical.Checkpoints) || !slices.Equal(legacy.Stages, vertical.Stages) || legacy.MutationAttempts != vertical.MutationAttempts {
		t.Fatalf("resume differential drift\nlegacy=%s\nvertical=%s", resumeDifferentialSummary(legacy), resumeDifferentialSummary(vertical))
	}
}

func resumeDifferentialSummary(observation resumeDifferentialObservation) string {
	return fmt.Sprintf("result=%x code=%q error=%q record_bytes=%d intent_bytes=%d checkpoints=%d stages=%v mutation_attempts=%d", observation.ResultJSON, observation.PublicErrorCode, observation.PublicError, len(observation.RecordBytes), len(observation.IntentBytes), len(observation.Checkpoints), observation.Stages, observation.MutationAttempts)
}

func resumeDifferentialCheckpointAt(t *testing.T, stateRoot, id, operationID string, stage port.ExecutionOrcaIntentStage) resumeDifferentialCheckpoint {
	t.Helper()
	return resumeDifferentialCheckpoint{Stage: stage, RecordBytes: rawIssueOpsRow(t, stateRoot, id), IntentBytes: rawExternalIntentRow(t, stateRoot, operationID)}
}

func resumeDifferentialCheckpointsEqual(left, right []resumeDifferentialCheckpoint) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Stage != right[index].Stage || !bytes.Equal(left[index].RecordBytes, right[index].RecordBytes) || !bytes.Equal(left[index].IntentBytes, right[index].IntentBytes) {
			return false
		}
	}
	return true
}

func assertResumeDifferentialStageCheckpoints(t *testing.T, observation resumeDifferentialObservation) {
	t.Helper()
	want := []port.ExecutionOrcaIntentStage{
		port.ExecutionOrcaIntentTerminal,
		port.ExecutionOrcaIntentRun,
		port.ExecutionOrcaIntentRunBind,
		port.ExecutionOrcaIntentTask,
		port.ExecutionOrcaIntentDispatch,
	}
	if len(observation.Checkpoints) != len(want) {
		t.Fatalf("stage checkpoints=%d observation=%s", len(observation.Checkpoints), resumeDifferentialSummary(observation))
	}
	for index, stage := range want {
		checkpoint := observation.Checkpoints[index]
		if checkpoint.Stage != stage || len(checkpoint.RecordBytes) == 0 || len(checkpoint.IntentBytes) == 0 {
			t.Fatalf("checkpoint %d=%#v", index, checkpoint)
		}
	}
}

func assertResumeDifferentialSingleAttempt(t *testing.T, observation resumeDifferentialObservation, failedStage port.ExecutionOrcaIntentStage) {
	t.Helper()
	attempts := 0
	for _, stage := range observation.Stages {
		if stage == failedStage {
			attempts++
		}
	}
	if attempts != 1 {
		t.Fatalf("ambiguous %s stage attempts=%d observation=%s", failedStage, attempts, resumeDifferentialSummary(observation))
	}
}

func resumeDifferentialArtifacts(_ context.Context, record leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
	coreRecord, err := resumeDifferentialCoreRecord(record)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	artifacts, err := ReadExecutionResumeArtifacts(coreRecord)
	if err != nil {
		return leasecontract.ResumeArtifacts{}, err
	}
	return leasecontract.ResumeArtifacts{ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256}, nil
}

type resumeDifferentialEffects struct {
	stateRoot   string
	checkpoints *[]resumeDifferentialCheckpoint
	t           *testing.T
}

func (e resumeDifferentialEffects) Begin(_ context.Context, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts, plan leasedomain.ResumePlan, operationID string) (leaseoutbound.ResumeEffectState, error) {
	coreRecord, err := resumeDifferentialCoreRecord(record)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	state, err := BeginExecutionResumeIntent(e.stateRoot, coreRecord, raw, ExecutionResumeArtifactsReceipt{ClaimTokenPath: artifacts.ClaimTokenPath, IssueBodySHA256: artifacts.IssueBodySHA256, ContextPacketPath: artifacts.ContextPacketPath, ContextPacketSHA256: artifacts.ContextPacketSHA256, OwnerPromptPath: artifacts.OwnerPromptPath, OwnerPromptSHA256: artifacts.OwnerPromptSHA256}, plan.RuntimeID, plan.ReusedTerminalPTYID, operationID, resumeDifferentialClock)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeDifferentialEffectState(state)
}

func (e resumeDifferentialEffects) Read(_ context.Context, id, operationID string) (leaseoutbound.ResumeEffectState, error) {
	state, err := ReadExecutionResumeIntent(e.stateRoot, id, operationID)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	if e.checkpoints != nil {
		*e.checkpoints = append(*e.checkpoints, resumeDifferentialCheckpointAt(e.t, e.stateRoot, id, operationID, state.Stage))
	}
	return resumeDifferentialEffectState(state)
}

func (e resumeDifferentialEffects) MarkInvoking(_ context.Context, state leaseoutbound.ResumeEffectState) (leaseoutbound.ResumeEffectState, error) {
	coreState, err := resumeDifferentialCoreState(state)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	next, err := MarkExecutionResumeIntentInvoking(e.stateRoot, coreState)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeDifferentialEffectState(next)
}

func (e resumeDifferentialEffects) RecordFailure(_ context.Context, state leaseoutbound.ResumeEffectState, invocation string, cause error) error {
	coreState, err := resumeDifferentialCoreState(state)
	if err != nil {
		return err
	}
	return RecordExecutionResumeIntentFailure(e.stateRoot, coreState, invocation, cause, resumeDifferentialClock)
}

func (e resumeDifferentialEffects) ApplyReceipt(ctx context.Context, state leaseoutbound.ResumeEffectState, receipt leasecontract.ResumeStageReceipt) (leaseoutbound.ResumeEffectState, error) {
	coreState, err := resumeDifferentialCoreState(state)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	portReceipt := port.ExecutionOrcaIntentReceipt{}
	switch coreState.Stage {
	case port.ExecutionOrcaIntentTerminal:
		portReceipt.TerminalPTYID = receipt.TerminalPTYID
	case port.ExecutionOrcaIntentRun:
		portReceipt.RunID = receipt.RunID
	case port.ExecutionOrcaIntentRunBind:
		portReceipt.RunID, portReceipt.RunBound = receipt.RunID, receipt.RunBound
	case port.ExecutionOrcaIntentTask:
		portReceipt.TaskID = receipt.TaskID
	case port.ExecutionOrcaIntentDispatch:
		portReceipt.TaskID, portReceipt.DispatchID = receipt.TaskID, receipt.DispatchID
	}
	next, err := ApplyExecutionResumeIntentReceipt(ctx, e.stateRoot, coreState, portReceipt, resumeDifferentialClock)
	if err != nil {
		return leaseoutbound.ResumeEffectState{}, err
	}
	return resumeDifferentialEffectState(next)
}

func resumeDifferentialCoreRecord(record leasecontract.Record) (IssueOpsRecord, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	var coreRecord IssueOpsRecord
	if err := json.Unmarshal(data, &coreRecord); err != nil {
		return IssueOpsRecord{}, err
	}
	return coreRecord, nil
}

func resumeDifferentialEffectState(state ExecutionResumeIntentState) (leaseoutbound.ResumeEffectState, error) {
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

func resumeDifferentialCoreState(state leaseoutbound.ResumeEffectState) (ExecutionResumeIntentState, error) {
	record, err := resumeDifferentialCoreRecord(state.Record)
	if err != nil {
		return ExecutionResumeIntentState{}, err
	}
	return ExecutionResumeIntentState{Record: record, RecordRaw: append([]byte(nil), state.RecordRaw...), IntentRaw: append([]byte(nil), state.IntentRaw...), OperationID: state.OperationID, Stage: port.ExecutionOrcaIntentStage(state.Stage), InvocationState: state.InvocationState, InvocationAttempts: state.InvocationAttempts, Pending: state.Pending}, nil
}

type resumeDifferentialFence struct{}

func (resumeDifferentialFence) Within(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}

type resumeDifferentialOperationIDs struct{}

func (resumeDifferentialOperationIDs) New() (string, error) { return strings.Repeat("a", 32), nil }

type resumeDifferentialPathMatcher struct{}

func (resumeDifferentialPathMatcher) Matches(left, right string) bool { return left == right }

var resumeDifferentialClock = func() time.Time { return time.Date(2026, time.July, 31, 2, 20, 0, 0, time.UTC) }

func resumeDifferentialTamperToken(t *testing.T, record IssueOpsRecord) {
	t.Helper()
	if err := os.WriteFile(claimTokenPath(record), []byte("tampered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resumeDifferentialTamperPacket(t *testing.T, record IssueOpsRecord) {
	t.Helper()
	path, _ := executionOwnerArtifactPaths(record)
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resumeDifferentialTamperPrompt(t *testing.T, record IssueOpsRecord) {
	t.Helper()
	_, path := executionOwnerArtifactPaths(record)
	if err := os.WriteFile(path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resumeDifferentialTamperManifest(t *testing.T, record IssueOpsRecord) {
	t.Helper()
	path := filepath.Join(record.Execution.Workspace.Root, IssueOpsArtifactDir, "plan.md")
	if err := os.WriteFile(path, []byte("tampered plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
