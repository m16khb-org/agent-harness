package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	preparationoutbound "agent-harness/internal/adapter/outbound/issueopspreparation"
	preparationapp "agent-harness/internal/application/issueopspreparation"
	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

type preparationDifferentialObservation struct {
	result      []byte
	err         string
	record      []byte
	holder      []byte
	holderFound bool
	intent      []byte
	intentFound bool
	trace       []string
}

func TestPreparationDifferentialDirectMatrix(t *testing.T) {
	cases := []struct {
		name    string
		request func(IssueOpsRecord) ExecutionPrepareRequest
		access  port.ExecutionWorkspaceAccessResult
		err     error
	}{
		{name: "preview", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			return preparationDifferentialRequest(record, "direct", false)
		}, access: port.ExecutionWorkspaceAccessResult{Allowed: true}},
		{name: "success", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			return preparationDifferentialRequest(record, "direct", true)
		}, access: port.ExecutionWorkspaceAccessResult{Allowed: true}},
		{name: "auto fallback", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			return preparationDifferentialRequest(record, "auto", true)
		}, access: port.ExecutionWorkspaceAccessResult{Allowed: true}},
		{name: "access denial", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			return preparationDifferentialRequest(record, "direct", true)
		}, access: port.ExecutionWorkspaceAccessResult{Code: "canonical_worktree_base_inaccessible", RelaunchCommand: "codex relaunch"}},
		{name: "provision failure", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			return preparationDifferentialRequest(record, "direct", true)
		}, access: port.ExecutionWorkspaceAccessResult{Allowed: true}, err: errors.New("prepare failed")},
		{name: "cwd denial", request: func(record IssueOpsRecord) ExecutionPrepareRequest {
			request := preparationDifferentialRequest(record, "direct", true)
			request.CWD = "/foreign"
			return request
		}, access: port.ExecutionWorkspaceAccessResult{Allowed: true}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			legacyRoot, record := executionPrepareRecord(t)
			verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
			request := test.request(record)
			legacyTrace, verticalTrace := []string{}, []string{}
			legacyProvider := &preparationDifferentialDirect{access: test.access, err: test.err, trace: &legacyTrace}
			verticalProvider := &preparationDifferentialDirect{access: test.access, err: test.err, trace: &verticalTrace}

			legacy := runLegacyPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
			vertical := runVerticalPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
			assertPreparationDifferentialEqual(t, legacy, vertical)
		})
	}
}

func TestPreparationDifferentialExistingExecutionMatrix(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		confirm bool
		mutate  func(*IssueOpsRecord)
	}{
		{name: "idempotent", mode: "direct", confirm: true},
		{name: "idempotent auto", mode: "auto", confirm: true},
		{name: "preview writerless", mode: "direct", confirm: false, mutate: func(record *IssueOpsRecord) {
			record.Execution.Lease = model.WriteLease{Generation: 1, Status: model.LeaseStatusClaimable, ClaimTokenSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
		}},
		{name: "pending", mode: "direct", confirm: true, mutate: func(record *IssueOpsRecord) {
			record.Execution.Pending = &model.ExternalIntent{OperationID: preparationDifferentialOperation, Kind: "dispatch", Marker: "marker", StartedAt: "2026-08-02T00:00:00Z"}
		}},
		{name: "mode mismatch", mode: "orca", confirm: true},
		{name: "claimable writerless", mode: "direct", confirm: true, mutate: func(record *IssueOpsRecord) {
			record.Execution.Lease = model.WriteLease{Generation: 1, Status: model.LeaseStatusClaimable, ClaimTokenSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
		}},
		{name: "released writerless", mode: "direct", confirm: true, mutate: func(record *IssueOpsRecord) {
			record.Execution.Lease = model.WriteLease{Generation: 2, Status: model.LeaseStatusReleased}
		}},
		{name: "revoking writerless", mode: "direct", confirm: true, mutate: func(record *IssueOpsRecord) {
			record.Execution.Lease.Generation = 3
			record.Execution.Lease.Status = model.LeaseStatusRevoking
			record.Execution.Lease.ClaimTokenSHA256 = ""
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyRoot, record := executionPrepareRecord(t)
			workspace, err := executionWorkspaceRequest(record, true)
			if err != nil {
				t.Fatal(err)
			}
			actor := executionActor("codex", "preparation-differential")
			record.WorktreePath = workspace.Root
			record.Execution = &model.Execution{
				Mode: model.ExecutionModeDirect,
				Workspace: model.Workspace{
					SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
					BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, Driver: "git", LinkedAt: "2026-08-02T00:00:00Z",
				},
				Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &actor, ClaimedAt: "2026-08-02T00:00:00Z"},
			}
			if test.mutate != nil {
				test.mutate(&record)
			}
			if _, err := writeIssueOps(legacyRoot, record); err != nil {
				t.Fatal(err)
			}
			verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
			request := preparationDifferentialRequest(record, test.mode, test.confirm)
			legacyTrace, verticalTrace := []string{}, []string{}
			legacyProvider := &preparationDifferentialDirect{access: port.ExecutionWorkspaceAccessResult{Allowed: true}, trace: &legacyTrace}
			verticalProvider := &preparationDifferentialDirect{access: port.ExecutionWorkspaceAccessResult{Allowed: true}, trace: &verticalTrace}

			legacy := runLegacyPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
			vertical := runVerticalPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
			assertPreparationDifferentialEqual(t, legacy, vertical)
		})
	}
}

func TestPreparationDifferentialRootCollision(t *testing.T) {
	legacyRoot, record := executionPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	conflict := record
	conflict.ID = "io-conflicting-preparation"
	conflict.Branch = "conflicting-branch"
	conflict.WorktreePath = workspace.Root
	conflict.Execution = nil
	if _, err := writeIssueOps(legacyRoot, conflict); err != nil {
		t.Fatal(err)
	}
	verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
	clonePreparationDifferentialRecord(t, legacyRoot, verticalRoot, conflict.ID)
	request := preparationDifferentialRequest(record, "direct", true)
	legacyTrace, verticalTrace := []string{}, []string{}
	legacyProvider := &preparationDifferentialDirect{access: port.ExecutionWorkspaceAccessResult{Allowed: true}, trace: &legacyTrace}
	verticalProvider := &preparationDifferentialDirect{access: port.ExecutionWorkspaceAccessResult{Allowed: true}, trace: &verticalTrace}

	legacy := runLegacyPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
	vertical := runVerticalPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
	assertPreparationDifferentialEqual(t, legacy, vertical)
}

func TestPreparationDifferentialOrcaMatrix(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		confirm    bool
		probe      port.ExecutionOrcaProbeResult
		failStage  port.ExecutionOrcaIntentStage
		invokeErr  error
		inspection *port.ExecutionOrcaIntentInventory
	}{
		{name: "preview", mode: "orca", probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}},
		{name: "success", mode: "orca", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}},
		{name: "auto ready", mode: "auto", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}},
		{name: "explicit unavailable", mode: "orca", probe: port.ExecutionOrcaProbeResult{Code: "orca_unavailable"}},
		{name: "dispatch unknown", mode: "orca", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}, failStage: port.ExecutionOrcaIntentDispatch, invokeErr: &port.OrcaError{Code: "transport", Invoked: true}},
		{name: "terminal not invoked", mode: "orca", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}, failStage: port.ExecutionOrcaIntentTerminal, invokeErr: &port.OrcaError{Code: "transport", Invoked: false}},
		{name: "non authoritative zero", mode: "orca", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}, inspection: &port.ExecutionOrcaIntentInventory{}},
		{name: "multiple candidates", mode: "orca", confirm: true, probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}, inspection: &port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TerminalPTYID: "one"}, {TerminalPTYID: "two"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyRoot, record := orcaPrepareRecord(t)
			verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
			request := preparationDifferentialRequest(record, test.mode, test.confirm)
			legacyTrace, verticalTrace := []string{}, []string{}
			legacyProvider := &preparationDifferentialOrca{probe: test.probe, failStage: test.failStage, invokeErr: test.invokeErr, inspection: test.inspection, trace: &legacyTrace}
			verticalProvider := &preparationDifferentialOrca{probe: test.probe, failStage: test.failStage, invokeErr: test.invokeErr, inspection: test.inspection, trace: &verticalTrace}

			legacy := runLegacyOrcaPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
			vertical := runVerticalOrcaPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
			assertPreparationDifferentialEqual(t, legacy, vertical)
		})
	}
}

func TestPreparationDifferentialOrcaBranchCollision(t *testing.T) {
	legacyRoot, record := executionPrepareRecord(t)
	verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
	request := preparationDifferentialRequest(record, "orca", true)
	legacyTrace, verticalTrace := []string{}, []string{}
	probe := port.ExecutionOrcaProbeResult{Available: true, Ready: true}
	legacyProvider := &preparationDifferentialOrca{probe: probe, trace: &legacyTrace}
	verticalProvider := &preparationDifferentialOrca{probe: probe, trace: &verticalTrace}

	legacy := runLegacyOrcaPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
	vertical := runVerticalOrcaPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
	assertPreparationDifferentialEqual(t, legacy, vertical)
}

func TestPreparationDifferentialParentWorktree(t *testing.T) {
	legacyRoot, record := orcaPrepareRecord(t)
	record.BranchPrepare.BaseBranch = "117-umbrella"
	record.Delegation = &IssueOpsDelegationContract{ParentCycleID: "io-parent"}
	if _, err := writeIssueOps(legacyRoot, record); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(record.Repo+".worktrees", record.BranchPrepare.BaseBranch)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	verticalRoot := clonePreparationDifferentialState(t, legacyRoot, record.ID)
	request := preparationDifferentialRequest(record, "orca", true)
	request.CWD = parent
	legacyTrace, verticalTrace := []string{}, []string{}
	probe := port.ExecutionOrcaProbeResult{Available: true, Ready: true}
	legacyProvider := &preparationDifferentialOrca{probe: probe, trace: &legacyTrace}
	verticalProvider := &preparationDifferentialOrca{probe: probe, trace: &verticalTrace}

	legacy := runLegacyOrcaPreparationDifferential(t, legacyRoot, record.ID, request, legacyProvider, &legacyTrace)
	vertical := runVerticalOrcaPreparationDifferential(t, verticalRoot, record.ID, request, verticalProvider, &verticalTrace)
	assertPreparationDifferentialEqual(t, legacy, vertical)
}

func preparationDifferentialRequest(record IssueOpsRecord, mode string, confirm bool) ExecutionPrepareRequest {
	return ExecutionPrepareRequest{
		ID: record.ID, Mode: mode, Actor: executionActor("codex", "preparation-differential"),
		CWD: record.Repo, OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh", Confirm: confirm,
	}
}

func runLegacyPreparationDifferential(t *testing.T, stateRoot, id string, request ExecutionPrepareRequest, direct port.ExecutionWorkspaceProvisioner, trace *[]string) preparationDifferentialObservation {
	t.Helper()
	clock := &preparationDifferentialClock{trace: trace}
	result, err := prepareExecutionCompatibilityOracle(context.Background(), stateRoot, request, ExecutionPrepareDependencies{Direct: direct, Now: clock.Now})
	return observePreparationDifferential(t, stateRoot, id, request.Actor, result, err, *trace)
}

func runVerticalPreparationDifferential(t *testing.T, stateRoot, id string, request ExecutionPrepareRequest, direct port.ExecutionWorkspaceProvisioner, trace *[]string) preparationDifferentialObservation {
	t.Helper()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	repository := preparationoutbound.NewSQLiteRepository(database, func(context.Context) error {
		return RequireIssueOpsMutationAllowed(stateRoot)
	})
	evidence := preparationDifferentialEvidence{stateRoot: stateRoot, readIssue: executionIssueSnapshotReader}
	service := preparationapp.NewService(
		repository, &preparationDifferentialApplicationClock{trace: trace}, preparationDifferentialOperationID{},
		preparationoutbound.NewDirectWorkspace(direct), nil, evidence,
	)
	result, serviceErr := service.Prepare(context.Background(), preparationDifferentialCommand(request))
	return observePreparationDifferential(t, stateRoot, id, request.Actor, preparationDifferentialPublicResult(t, result), serviceErr, *trace)
}

func runLegacyOrcaPreparationDifferential(t *testing.T, stateRoot, id string, request ExecutionPrepareRequest, orca port.ExecutionOrcaProvisioner, trace *[]string) preparationDifferentialObservation {
	t.Helper()
	clock := &preparationDifferentialClock{trace: trace}
	result, err := prepareExecutionCompatibilityOracle(context.Background(), stateRoot, request, ExecutionPrepareDependencies{
		Orca: orca, ReadIssue: executionIssueSnapshotReader, Now: clock.Now, OperationID: preparationDifferentialOperation,
	})
	return observePreparationDifferential(t, stateRoot, id, request.Actor, result, err, *trace)
}

func runVerticalOrcaPreparationDifferential(t *testing.T, stateRoot, id string, request ExecutionPrepareRequest, orca port.ExecutionOrcaProvisioner, trace *[]string) preparationDifferentialObservation {
	t.Helper()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	repository := preparationoutbound.NewSQLiteRepositoryWithDiagnosticRedactor(database, func(context.Context) error {
		return RequireIssueOpsMutationAllowed(stateRoot)
	}, policy.RedactDiagnostic)
	evidence := preparationDifferentialEvidence{stateRoot: stateRoot, readIssue: executionIssueSnapshotReader}
	gateway := preparationoutbound.NewOrcaGateway(preparationoutbound.OrcaDependencies{
		Provisioner: orca,
		ValidateProbe: func(_ context.Context, probe preparationcontract.ProbeRequest) (string, error) {
			record, readErr := ReadIssueOps(stateRoot, id)
			if readErr != nil {
				return "orca_branch_precheck_failed", readErr
			}
			return "orca_branch_conflict", ensureOrcaBranchIsFree(record, probe.Workspace.Branch)
		},
		HydrateLaunch: func(_ context.Context, intent preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
			return hydratePreparationDifferentialLaunch(stateRoot, id, intent)
		},
	})
	service := preparationapp.NewService(
		repository, &preparationDifferentialApplicationClock{trace: trace}, preparationDifferentialOperationID{},
		nil, gateway, evidence,
	)
	result, serviceErr := service.Prepare(context.Background(), preparationDifferentialCommand(request))
	return observePreparationDifferential(t, stateRoot, id, request.Actor, preparationDifferentialPublicResult(t, result), serviceErr, *trace)
}

func preparationDifferentialPublicResult(t *testing.T, result preparationcontract.Result) ExecutionPrepareResult {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var public ExecutionPrepareResult
	if err := json.Unmarshal(data, &public); err != nil {
		t.Fatal(err)
	}
	return public
}

func observePreparationDifferential(t *testing.T, stateRoot, id string, actor NativeActor, result any, err error, trace []string) preparationDifferentialObservation {
	t.Helper()
	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	database, openErr := sqlstore.Open(stateRoot)
	if openErr != nil {
		t.Fatal(openErr)
	}
	holder, holderFound, getErr := database.Get(leaseHolderBucket, leaseHolderIndexKey(actor))
	if getErr != nil {
		t.Fatal(getErr)
	}
	intent, intentFound, intentErr := database.Get(externalIntentBucket, preparationDifferentialOperation)
	if intentErr != nil {
		t.Fatal(intentErr)
	}
	observation := preparationDifferentialObservation{
		result: resultJSON, record: rawIssueOpsRow(t, stateRoot, id), holder: holder,
		holderFound: holderFound, intent: intent, intentFound: intentFound, trace: append([]string(nil), trace...),
	}
	if err != nil {
		observation.err = err.Error()
	}
	return observation
}

func assertPreparationDifferentialEqual(t *testing.T, legacy, vertical preparationDifferentialObservation) {
	t.Helper()
	if !bytes.Equal(legacy.result, vertical.result) {
		t.Fatalf("result differs\nlegacy=%s\nvertical=%s", legacy.result, vertical.result)
	}
	if legacy.err != vertical.err {
		t.Fatalf("error differs\nlegacy=%q\nvertical=%q", legacy.err, vertical.err)
	}
	if !bytes.Equal(legacy.record, vertical.record) {
		t.Fatalf("record differs\nlegacy=%s\nvertical=%s", legacy.record, vertical.record)
	}
	if legacy.holderFound != vertical.holderFound || !bytes.Equal(legacy.holder, vertical.holder) {
		t.Fatalf("holder differs\nlegacy=%s (%t)\nvertical=%s (%t)", legacy.holder, legacy.holderFound, vertical.holder, vertical.holderFound)
	}
	if legacy.intentFound != vertical.intentFound || !bytes.Equal(legacy.intent, vertical.intent) {
		t.Fatalf("intent differs\nlegacy=%s (%t)\nvertical=%s (%t)", legacy.intent, legacy.intentFound, vertical.intent, vertical.intentFound)
	}
	if !bytes.Equal(mustJSON(t, legacy.trace), mustJSON(t, vertical.trace)) {
		t.Fatalf("trace differs\nlegacy=%v\nvertical=%v", legacy.trace, vertical.trace)
	}
}

func clonePreparationDifferentialState(t *testing.T, sourceRoot, id string) string {
	t.Helper()
	targetRoot := t.TempDir()
	database, err := sqlstore.Open(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueOpsBucket, id, rawIssueOpsRow(t, sourceRoot, id)); err != nil {
		t.Fatal(err)
	}
	return targetRoot
}

func clonePreparationDifferentialRecord(t *testing.T, sourceRoot, targetRoot, id string) {
	t.Helper()
	database, err := sqlstore.Open(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueOpsBucket, id, rawIssueOpsRow(t, sourceRoot, id)); err != nil {
		t.Fatal(err)
	}
}

func preparationDifferentialCommand(request ExecutionPrepareRequest) preparationcontract.Command {
	actor := preparationcontract.Actor{Host: request.Actor.Host, SessionID: request.Actor.SessionID, AgentID: request.Actor.AgentID}
	if request.Actor.SessionProcess != nil {
		actor.SessionProcess = &leasecontract.ProcessReceipt{
			PID: request.Actor.SessionProcess.PID, StartedAt: request.Actor.SessionProcess.StartedAt, Executable: request.Actor.SessionProcess.Executable,
		}
	}
	return preparationcontract.Command{
		ID: request.ID, Mode: request.Mode, Actor: actor, CWD: request.CWD,
		OwnerHost: request.OwnerHost, OwnerModel: request.OwnerModel, OwnerEffort: request.OwnerEffort, Confirm: request.Confirm,
	}
}

type preparationDifferentialDirect struct {
	access port.ExecutionWorkspaceAccessResult
	err    error
	trace  *[]string
}

type preparationDifferentialOrca struct {
	probe      port.ExecutionOrcaProbeResult
	failStage  port.ExecutionOrcaIntentStage
	invokeErr  error
	inspection *port.ExecutionOrcaIntentInventory
	trace      *[]string
}

func (orca *preparationDifferentialOrca) Probe(_ context.Context, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	*orca.trace = append(*orca.trace, "orca.probe")
	return orca.probe, nil
}

func (orca *preparationDifferentialOrca) InspectIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	*orca.trace = append(*orca.trace, "orca.inspect."+string(request.Stage))
	if orca.inspection != nil {
		return *orca.inspection, nil
	}
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (orca *preparationDifferentialOrca) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	*orca.trace = append(*orca.trace, "orca.invoke."+string(request.Stage))
	if request.Stage == orca.failStage {
		return port.ExecutionOrcaIntentReceipt{}, orca.invokeErr
	}
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if err := os.MkdirAll(request.Workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		return port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{
				SourceRoot: request.Workspace.SourceRoot, Root: request.Workspace.Root, Branch: request.Workspace.Branch,
				BaseHead: request.Workspace.BaseHead, ParentWorktree: request.Workspace.ParentWorktree, Driver: "orca", Exists: true,
			},
			RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1",
		}}, nil
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "terminal-1"}, nil
	case port.ExecutionOrcaIntentRun:
		return port.ExecutionOrcaIntentReceipt{RunID: "run-1"}, nil
	case port.ExecutionOrcaIntentRunBind:
		return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}, nil
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-1"}, nil
	case port.ExecutionOrcaIntentDispatch:
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-1"}, nil
	default:
		return port.ExecutionOrcaIntentReceipt{}, fmt.Errorf("unexpected Orca stage %q", request.Stage)
	}
}

func (direct *preparationDifferentialDirect) ProbeAccess(_ context.Context, _ port.ExecutionWorkspaceRequest, _ string) (port.ExecutionWorkspaceAccessResult, error) {
	*direct.trace = append(*direct.trace, "direct.probe_access")
	return direct.access, nil
}

func (direct *preparationDifferentialDirect) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	*direct.trace = append(*direct.trace, "direct.prepare")
	if direct.err != nil {
		return port.ExecutionWorkspaceReceipt{}, direct.err
	}
	if err := os.MkdirAll(request.Root, 0o755); err != nil {
		return port.ExecutionWorkspaceReceipt{}, err
	}
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git", Exists: request.Confirm,
	}, nil
}

type preparationDifferentialEvidence struct {
	stateRoot string
	readIssue ExecutionIssueSnapshotReadFunc
}

func (evidence preparationDifferentialEvidence) Workspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	record, err := preparationDifferentialCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	request, err := executionWorkspaceRequest(record, confirm)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	return preparationcontract.WorkspaceRequest{
		LifecycleID: request.LifecycleID, SourceRoot: request.SourceRoot, Root: request.Root,
		Branch: request.Branch, BaseBranch: request.BaseBranch, BaseHead: request.BaseHead,
		ParentWorktree: request.ParentWorktree, Confirm: request.Confirm,
	}, nil
}

func (evidence preparationDifferentialEvidence) ReadOwner(ctx context.Context, snapshot preparationcontract.Snapshot, _ preparationcontract.Command) (preparationcontract.OwnerEvidence, error) {
	record, err := preparationDifferentialCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	owner, err := readExecutionOwnerSnapshot(ctx, record, evidence.readIssue)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	identity, err := (preparationcontract.IntentCodec{}).PrepareIssueIdentity(snapshot.Record)
	if err != nil {
		return preparationcontract.OwnerEvidence{}, err
	}
	return preparationcontract.OwnerEvidence{
		IssueURL: owner.issue.URL, IssueBody: owner.issue.Body, BodySHA256: owner.issue.BodySHA256,
		Provider: identity.Provider, Issue: identity.Issue,
	}, nil
}

func (evidence preparationDifferentialEvidence) MaterializeDirect(_ context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	record, err := preparationDifferentialCoreRecord(snapshot)
	if err != nil {
		return err
	}
	record.WorktreePath = receipt.Root
	record.Execution = &model.Execution{
		Mode:      model.ExecutionModeDirect,
		Workspace: model.Workspace{SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver},
	}
	_, err = materializeStagedArtifacts(evidence.stateRoot, record)
	return err
}

func (evidence preparationDifferentialEvidence) PrepareOwner(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
	if receipt.Workspace == nil {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("Orca worktree candidate does not match the sealed intent")
	}
	record, err := preparationDifferentialCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	workspace := receipt.Workspace.Workspace
	record.WorktreePath = workspace.Root
	record.Execution.Workspace = model.Workspace{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, Driver: workspace.Driver, LinkedAt: intent.StartedAt,
	}
	owner, err := readExecutionOwnerSnapshot(ctx, record, evidence.readIssue)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	if owner.issue.BodySHA256 != intent.IssueBodySHA256 {
		return preparationcontract.OwnerArtifacts{}, fmt.Errorf("remote issue body drifted before owner launch recovery")
	}
	tokenSHA256, err := createOrAdoptClaimToken(record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	manifest, err := materializeStagedArtifacts(evidence.stateRoot, record)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{
		ID: command.ID, Mode: preparationcontract.ModeOrca,
		OwnerHost: command.OwnerHost, OwnerModel: command.OwnerModel, OwnerEffort: command.OwnerEffort,
	}, owner, manifest)
	if err != nil {
		return preparationcontract.OwnerArtifacts{}, err
	}
	return preparationcontract.OwnerArtifacts{
		ClaimTokenPath: claimTokenPath(record), ClaimTokenSHA256: tokenSHA256,
		ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256,
	}, nil
}

func hydratePreparationDifferentialLaunch(stateRoot, id string, request preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	raw, err := sqlstore.Open(stateRoot)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	intentRaw, ok, err := raw.Get(externalIntentBucket, preparationDifferentialOperation)
	if err != nil || !ok {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed Orca intent is unavailable")
	}
	intent, err := (preparationcontract.IntentCodec{}).Decode(preparationDifferentialOperation, intentRaw)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	token, err := readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil || tokenSHA256(token) != intent.ClaimTokenSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed claim token identity changed")
	}
	prompt, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, request.Launch.PromptPath)
	if err != nil || digestExecutionOwnerBytes(prompt) != request.Launch.PromptSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	packet, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, request.Launch.ContextPacketPath)
	if err != nil || digestExecutionOwnerBytes(packet) != request.Launch.ContextPacketSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed context packet identity changed")
	}
	request.Launch.Prompt = string(prompt)
	return request, nil
}

func preparationDifferentialCoreRecord(snapshot preparationcontract.Snapshot) (IssueOpsRecord, error) {
	var record IssueOpsRecord
	err := json.Unmarshal(snapshot.RecordRaw, &record)
	return record, err
}

type preparationDifferentialClock struct{ trace *[]string }

func (clock *preparationDifferentialClock) Now() time.Time {
	*clock.trace = append(*clock.trace, "clock.now")
	return preparationDifferentialTime
}

type preparationDifferentialApplicationClock struct{ trace *[]string }

func (clock *preparationDifferentialApplicationClock) Now() time.Time {
	*clock.trace = append(*clock.trace, "clock.now")
	return preparationDifferentialTime
}

type preparationDifferentialOperationID struct{}

func (preparationDifferentialOperationID) New() (string, error) {
	return preparationDifferentialOperation, nil
}

var preparationDifferentialTime = time.Date(2026, 8, 2, 3, 4, 5, 123456789, time.UTC)

const preparationDifferentialOperation = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
