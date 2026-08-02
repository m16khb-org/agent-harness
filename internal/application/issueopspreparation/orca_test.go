package issueopspreparation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

func TestOrcaPreviewAndAutoFallback(t *testing.T) {
	t.Run("ready preview reads owner without mutation", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK || !result.Preview || result.ResolvedMode != preparationcontract.ModeOrca {
			t.Fatalf("result=%+v", result)
		}
		assertOrcaTrace(t, fixture.trace, []string{"load", "workspace", "root", "orca.probe", "evidence.owner"})
	})

	t.Run("auto falls back before owner evidence", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.gateway.probe = preparationcontract.ProbeResult{Available: true, Ready: false, Code: "orca_unready"}
		result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeAuto))
		if err != nil {
			t.Fatal(err)
		}
		if result.ResolvedMode != preparationcontract.ModeDirect || result.FallbackCode != "orca_unready" {
			t.Fatalf("result=%+v", result)
		}
		assertOrcaTrace(t, fixture.trace, []string{"load", "workspace", "root", "orca.probe", "direct.prepare", "clock"})
	})

	t.Run("explicit unavailable is denied", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.gateway.probe = preparationcontract.ProbeResult{Code: "orca_adapter_unavailable"}
		result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
		if err == nil || result.OK || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		assertOrcaTrace(t, fixture.trace, []string{"load", "workspace", "root", "orca.probe"})
	})
}

func TestOrcaIntentRunsSixDurableBeforeEffectStages(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	result, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Execution == nil || result.Execution.Lease.Status != "claimable" || result.Execution.Lease.Generation != 1 || result.Execution.Lease.Holder != nil {
		t.Fatalf("result=%+v", result)
	}
	if fixture.repository.beginIndex < 0 || fixture.gateway.firstEffectIndex < 0 || fixture.repository.beginIndex >= fixture.gateway.firstEffectIndex {
		t.Fatalf("begin=%d first effect=%d trace=%v", fixture.repository.beginIndex, fixture.gateway.firstEffectIndex, fixture.trace)
	}
	for _, stage := range allOrcaStages() {
		mark := traceIndex(fixture.trace, "mark:"+string(stage))
		invoke := traceIndex(fixture.trace, "invoke:"+string(stage))
		apply := traceIndex(fixture.trace, "apply:"+string(stage))
		if mark < 0 || invoke < 0 || apply < 0 || !(mark < invoke && invoke < apply) {
			t.Fatalf("stage=%s mark=%d invoke=%d apply=%d trace=%v", stage, mark, invoke, apply, fixture.trace)
		}
	}
	owner := traceIndex(fixture.trace, "evidence.prepare-owner")
	worktreeApply := traceIndex(fixture.trace, "apply:"+string(preparationcontract.IntentStageWorktree))
	if owner < 0 || owner >= worktreeApply {
		t.Fatalf("owner=%d worktree apply=%d trace=%v", owner, worktreeApply, fixture.trace)
	}
}

func TestOrcaIntentInventoryAndRetryGuards(t *testing.T) {
	tests := []struct {
		name            string
		configure       func(*orcaApplicationFixture)
		wantError       string
		wantWorktreeInv bool
	}{
		{name: "one candidate is adopted without repeating", configure: func(f *orcaApplicationFixture) {
			f.gateway.candidateStage = preparationcontract.IntentStageWorktree
		}, wantWorktreeInv: false},
		{name: "multiple candidates", configure: func(f *orcaApplicationFixture) {
			f.gateway.inventory = preparationcontract.IntentInventory{Candidates: []preparationcontract.IntentReceipt{{}, {}}}
			f.gateway.inventorySet = true
		}, wantError: "multiple candidates"},
		{name: "non authoritative zero", configure: func(f *orcaApplicationFixture) {
			f.gateway.inventory = preparationcontract.IntentInventory{}
			f.gateway.inventorySet = true
		}, wantError: "non-authoritative zero"},
		{name: "unknown outcome is not repeated", configure: func(f *orcaApplicationFixture) {
			f.repository.initialInvocation = preparationcontract.InvocationUnknown
		}, wantError: "absence was not proven"},
		{name: "two attempt bound", configure: func(f *orcaApplicationFixture) {
			f.repository.initialAttempts = preparationcontract.MaxInvocationAttempts
		}, wantError: "retry is exhausted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newOrcaApplicationFixture()
			test.configure(fixture)
			_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("err=%v trace=%v", err, fixture.trace)
				}
				if traceIndex(fixture.trace, "invoke:"+string(preparationcontract.IntentStageWorktree)) >= 0 {
					t.Fatalf("guard repeated worktree mutation: %v", fixture.trace)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			invoked := traceIndex(fixture.trace, "invoke:"+string(preparationcontract.IntentStageWorktree)) >= 0
			if invoked != test.wantWorktreeInv {
				t.Fatalf("worktree invoked=%v trace=%v", invoked, fixture.trace)
			}
		})
	}
}

func TestOrcaIntentBoundsAndOwnerFailures(t *testing.T) {
	t.Run("fixed stage bound", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.repository.stickPending = true
		_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
		if err == nil || !strings.Contains(err.Error(), "fixed external intent stage count") {
			t.Fatalf("err=%v trace=%v", err, fixture.trace)
		}
		if got := countTracePrefix(fixture.trace, "invoke:"); got != 6 {
			t.Fatalf("invoke count=%d trace=%v", got, fixture.trace)
		}
	})

	t.Run("owner artifact failure is recorded before receipt", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.evidence.prepareErr = errors.New("remote issue body drifted before owner launch recovery")
		_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
		if err == nil || !strings.Contains(err.Error(), "drifted") {
			t.Fatalf("err=%v", err)
		}
		failure := traceIndex(fixture.trace, "failure:unknown")
		apply := traceIndex(fixture.trace, "apply:"+string(preparationcontract.IntentStageWorktree))
		if failure < 0 || apply >= 0 {
			t.Fatalf("trace=%v", fixture.trace)
		}
	})
}

func TestOrcaBranchAndParentValidationStopBeforeIntent(t *testing.T) {
	t.Run("branch precheck", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.gateway.probe.Code = "orca_branch_name_taken"
		fixture.gateway.probeErr = errors.New("branch already exists")
		_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
		if err == nil || !strings.Contains(err.Error(), "branch already exists") || traceIndex(fixture.trace, "begin") >= 0 {
			t.Fatalf("err=%v trace=%v", err, fixture.trace)
		}
	})

	t.Run("sealed parent cwd", func(t *testing.T) {
		fixture := newOrcaApplicationFixture()
		fixture.evidence.ownerErr = errors.New("Orca prepare cwd must be source_root, the canonical worktree, or the sealed parent worktree")
		_, err := fixture.service.Prepare(context.Background(), orcaCommand(true, preparationcontract.ModeOrca))
		if err == nil || !strings.Contains(err.Error(), "sealed parent worktree") || traceIndex(fixture.trace, "begin") >= 0 {
			t.Fatalf("err=%v trace=%v", err, fixture.trace)
		}
	})
}

type orcaApplicationFixture struct {
	trace      []string
	repository *orcaApplicationRepositoryFake
	gateway    *orcaApplicationGatewayFake
	evidence   *orcaApplicationEvidenceFake
	service    *Service
}

func newOrcaApplicationFixture() *orcaApplicationFixture {
	fixture := &orcaApplicationFixture{}
	record := leasecontract.Record{
		SchemaVersion: 1, ID: "io-orca", Repo: "/repo", Branch: "199-orca", Phase: "implement",
		IssueURL:      "https://github.com/example/repo/issues/199",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/199","branch":"199-orca","base_branch":"main","base_sha":"base","link_verified":true}`),
	}
	fixture.repository = &orcaApplicationRepositoryFake{trace: &fixture.trace, snapshot: preparationcontract.Snapshot{Record: record, RecordRaw: []byte("raw")}, beginIndex: -1}
	fixture.gateway = &orcaApplicationGatewayFake{trace: &fixture.trace, probe: preparationcontract.ProbeResult{Available: true, Ready: true}, firstEffectIndex: -1}
	fixture.evidence = &orcaApplicationEvidenceFake{
		trace:     &fixture.trace,
		workspace: preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: record.Repo, Root: "/repo.worktrees/199-orca", Branch: record.Branch, BaseBranch: "main", BaseHead: "base", Confirm: true},
		owner:     preparationcontract.OwnerEvidence{IssueURL: record.IssueURL, IssueBody: "body", BodySHA256: strings.Repeat("a", 64), Source: "github"},
	}
	direct := &applicationDirectFake{trace: &fixture.trace, access: preparationcontract.AccessResult{Allowed: true}, receipt: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/199-orca", Branch: "199-orca", BaseHead: "base", Driver: "git"}}
	fixture.service = NewService(fixture.repository, &applicationClockFake{trace: &fixture.trace}, &orcaOperationIDFake{trace: &fixture.trace}, direct, fixture.gateway, fixture.evidence)
	return fixture
}

func orcaCommand(confirm bool, mode string) preparationcontract.Command {
	return preparationcontract.Command{
		ID: "io-orca", Mode: mode, CWD: "/repo", OwnerHost: "codex", Confirm: confirm,
		Actor: leasecontract.Actor{Host: "codex", SessionID: "session", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}},
	}
}

type orcaApplicationRepositoryFake struct {
	trace             *[]string
	snapshot          preparationcontract.Snapshot
	initialInvocation string
	initialAttempts   int
	stickPending      bool
	beginIndex        int
}

func (fake *orcaApplicationRepositoryFake) Load(context.Context, string) (preparationcontract.Snapshot, error) {
	*fake.trace = append(*fake.trace, "load")
	return fake.snapshot.Clone(), nil
}
func (fake *orcaApplicationRepositoryFake) EnsureRootUnclaimed(context.Context, string, string) error {
	*fake.trace = append(*fake.trace, "root")
	return nil
}
func (*orcaApplicationRepositoryFake) CommitDirect(context.Context, DirectCommit) (preparationcontract.Result, error) {
	return preparationcontract.Result{}, errors.New("unexpected direct commit")
}
func (fake *orcaApplicationRepositoryFake) BeginIntent(_ context.Context, begin OrcaBegin) (IntentState, error) {
	*fake.trace = append(*fake.trace, "begin")
	fake.beginIndex = len(*fake.trace) - 1
	invocation := fake.initialInvocation
	if invocation == "" {
		invocation = preparationcontract.InvocationNotInvoked
	}
	intent := preparationcontract.Intent{
		SchemaVersion: 1, Purpose: preparationcontract.PurposePrepare, OperationID: begin.OperationID,
		LifecycleID: begin.Snapshot.Record.ID, Generation: 1, Stage: preparationcontract.IntentStageWorktree,
		StartedAt: begin.StartedAt, InvocationState: invocation, InvocationAttempts: fake.initialAttempts,
		Workspace: begin.Workspace, Probe: begin.Probe, IssueBodySHA256: begin.Owner.BodySHA256,
	}
	return IntentState{Snapshot: begin.Snapshot.Clone(), Intent: intent, IntentRaw: []byte("intent"), Pending: true}, nil
}
func (fake *orcaApplicationRepositoryFake) MarkInvoking(_ context.Context, state IntentState) (IntentState, error) {
	*fake.trace = append(*fake.trace, "mark:"+string(state.Intent.Stage))
	state.Intent.InvocationState = preparationcontract.InvocationUnknown
	state.Intent.InvocationAttempts++
	return state, nil
}
func (fake *orcaApplicationRepositoryFake) RecordFailure(_ context.Context, _ IntentState, invocation string, _ error) error {
	*fake.trace = append(*fake.trace, "failure:"+invocation)
	return nil
}
func (fake *orcaApplicationRepositoryFake) ApplyReceipt(_ context.Context, state IntentState, receipt preparationcontract.IntentReceipt) (IntentProgress, error) {
	stage := state.Intent.Stage
	*fake.trace = append(*fake.trace, "apply:"+string(stage))
	state.Intent.InvocationState = preparationcontract.InvocationNotInvoked
	state.Intent.InvocationAttempts = 0
	switch stage {
	case preparationcontract.IntentStageWorktree:
		state.Intent.Prepared = receipt.Workspace
		state.Intent.Launch = &preparationcontract.LaunchIdentity{PromptPath: "/prompt", PromptSHA256: strings.Repeat("b", 64), ContextPacketPath: "/packet", ContextPacketSHA256: strings.Repeat("c", 64)}
		state.Intent.ClaimTokenSHA256 = strings.Repeat("d", 64)
		state.Intent.Stage = preparationcontract.IntentStageTerminal
	case preparationcontract.IntentStageTerminal:
		state.Intent.TerminalPTYID = receipt.TerminalPTYID
		state.Intent.Stage = preparationcontract.IntentStageRun
	case preparationcontract.IntentStageRun:
		state.Intent.RunID = receipt.RunID
		state.Intent.Stage = preparationcontract.IntentStageRunBind
	case preparationcontract.IntentStageRunBind:
		state.Intent.RunBound = receipt.RunBound
		state.Intent.Stage = preparationcontract.IntentStageTask
	case preparationcontract.IntentStageTask:
		state.Intent.TaskID = receipt.TaskID
		state.Intent.Stage = preparationcontract.IntentStageDispatch
	case preparationcontract.IntentStageDispatch:
		if fake.stickPending {
			state.Intent.Stage = preparationcontract.IntentStageWorktree
			return IntentProgress{State: state, Pending: true}, nil
		}
		execution := &leasecontract.Execution{
			Mode:      preparationcontract.ModeOrca,
			Workspace: leasecontract.Workspace{SourceRoot: state.Intent.Workspace.SourceRoot, Root: state.Intent.Workspace.Root, Branch: state.Intent.Workspace.Branch, BaseHead: state.Intent.Workspace.BaseHead, Driver: "orca", LinkedAt: state.Intent.StartedAt},
			Lease:     leasecontract.Lease{Generation: 1, Status: "claimable", ClaimTokenSHA256: state.Intent.ClaimTokenSHA256},
		}
		result := preparationcontract.Result{OK: true, ID: state.Intent.LifecycleID, RequestedMode: preparationcontract.ModeOrca, ResolvedMode: preparationcontract.ModeOrca, Workspace: execution.Workspace, Execution: execution, ClaimTokenPath: "/claim", IssueBodySHA256: state.Intent.IssueBodySHA256}
		return IntentProgress{State: state, Result: result}, nil
	}
	return IntentProgress{State: state, Pending: true}, nil
}

type orcaApplicationGatewayFake struct {
	trace            *[]string
	probe            preparationcontract.ProbeResult
	inventory        preparationcontract.IntentInventory
	inventorySet     bool
	candidateStage   preparationcontract.IntentStage
	firstEffectIndex int
	probeErr         error
}

func (fake *orcaApplicationGatewayFake) Probe(context.Context, preparationcontract.ProbeRequest) (preparationcontract.ProbeResult, error) {
	*fake.trace = append(*fake.trace, "orca.probe")
	return fake.probe, fake.probeErr
}
func (fake *orcaApplicationGatewayFake) Inspect(_ context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentInventory, error) {
	*fake.trace = append(*fake.trace, "inspect:"+string(request.Stage))
	if fake.inventorySet {
		return fake.inventory, nil
	}
	if fake.candidateStage == request.Stage {
		return preparationcontract.IntentInventory{Candidates: []preparationcontract.IntentReceipt{orcaApplicationReceipt(request.Stage)}}, nil
	}
	return preparationcontract.IntentInventory{AuthoritativeZero: true}, nil
}
func (fake *orcaApplicationGatewayFake) Invoke(_ context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentReceipt, error) {
	*fake.trace = append(*fake.trace, "invoke:"+string(request.Stage))
	if fake.firstEffectIndex < 0 {
		fake.firstEffectIndex = len(*fake.trace) - 1
	}
	return orcaApplicationReceipt(request.Stage), nil
}

func orcaApplicationReceipt(stage preparationcontract.IntentStage) preparationcontract.IntentReceipt {
	workspace := &preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/199-orca", Branch: "199-orca", BaseHead: "base", Driver: "orca", Exists: true},
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
	}
	switch stage {
	case preparationcontract.IntentStageWorktree:
		return preparationcontract.IntentReceipt{Workspace: workspace}
	case preparationcontract.IntentStageTerminal:
		return preparationcontract.IntentReceipt{TerminalPTYID: "terminal"}
	case preparationcontract.IntentStageRun:
		return preparationcontract.IntentReceipt{RunID: "run"}
	case preparationcontract.IntentStageRunBind:
		return preparationcontract.IntentReceipt{RunID: "run", RunBound: true}
	case preparationcontract.IntentStageTask:
		return preparationcontract.IntentReceipt{TaskID: "task"}
	case preparationcontract.IntentStageDispatch:
		return preparationcontract.IntentReceipt{TaskID: "task", DispatchID: "dispatch"}
	default:
		return preparationcontract.IntentReceipt{}
	}
}

type orcaApplicationEvidenceFake struct {
	trace      *[]string
	workspace  preparationcontract.WorkspaceRequest
	owner      preparationcontract.OwnerEvidence
	ownerErr   error
	prepareErr error
}

func (fake *orcaApplicationEvidenceFake) Workspace(_ preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	*fake.trace = append(*fake.trace, "workspace")
	request := fake.workspace
	request.Confirm = confirm
	return request, nil
}
func (fake *orcaApplicationEvidenceFake) ReadOwner(context.Context, preparationcontract.Snapshot, preparationcontract.Command) (preparationcontract.OwnerEvidence, error) {
	*fake.trace = append(*fake.trace, "evidence.owner")
	return fake.owner, fake.ownerErr
}
func (*orcaApplicationEvidenceFake) MaterializeDirect(context.Context, preparationcontract.Snapshot, preparationcontract.WorkspaceReceipt) error {
	return errors.New("unexpected direct materialization")
}
func (fake *orcaApplicationEvidenceFake) PrepareOwner(context.Context, preparationcontract.Snapshot, preparationcontract.Command, preparationcontract.Intent, preparationcontract.IntentReceipt) (preparationcontract.OwnerArtifacts, error) {
	*fake.trace = append(*fake.trace, "evidence.prepare-owner")
	return preparationcontract.OwnerArtifacts{}, fake.prepareErr
}

type orcaOperationIDFake struct{ trace *[]string }

func (fake *orcaOperationIDFake) New() (string, error) {
	*fake.trace = append(*fake.trace, "operation")
	return "0123456789abcdef0123456789abcdef", nil
}

func allOrcaStages() []preparationcontract.IntentStage {
	return []preparationcontract.IntentStage{
		preparationcontract.IntentStageWorktree, preparationcontract.IntentStageTerminal,
		preparationcontract.IntentStageRun, preparationcontract.IntentStageRunBind,
		preparationcontract.IntentStageTask, preparationcontract.IntentStageDispatch,
	}
}

func assertOrcaTrace(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("trace=%v want=%v", got, want)
	}
}
func traceIndex(trace []string, want string) int {
	for index, item := range trace {
		if item == want {
			return index
		}
	}
	return -1
}
func countTracePrefix(trace []string, prefix string) int {
	count := 0
	for _, item := range trace {
		if strings.HasPrefix(item, prefix) {
			count++
		}
	}
	return count
}
