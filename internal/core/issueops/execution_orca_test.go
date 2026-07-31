package issueops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestExecutionOrcaPersistsIntentBeforeExternalMutationAndCASReceipt(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		pending, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		if pending.Execution == nil || pending.Execution.Pending == nil || pending.Execution.Pending.Kind != "worktree_create" {
			t.Fatalf("external mutation ran before its durable intent: %#v", pending.Execution)
		}
		if request.Marker != pending.Execution.Pending.Marker || request.Marker == "" {
			t.Fatalf("adapter marker must equal the durable intent marker: request=%q pending=%#v", request.Marker, pending.Execution.Pending)
		}
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}

	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "claude-fable-5", OwnerEffort: "high",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if fake.prepareCalls != 1 || got.Execution == nil || got.Execution.Pending != nil || got.Execution.Orca == nil {
		t.Fatalf("Orca receipt was not CAS-persisted exactly once: calls=%d result=%#v", fake.prepareCalls, got)
	}
	if got.Execution.Lease.Status != model.LeaseStatusClaimable || got.ClaimTokenPath == "" {
		t.Fatalf("verified dispatch must produce one claimable lease: %#v", got)
	}
	if got.Execution.Orca.OwnerModel != "claude-fable-5" || got.Execution.Orca.OwnerEffort != "high" {
		t.Fatalf("caller owner profile was not preserved: %#v", got.Execution.Orca)
	}
}

func TestExecutionOrcaPrepareAllowsGitHubLinkVerificationAfterLocalBranchCreation(t *testing.T) {
	for _, mode := range []string{ExecutionModeAuto, string(model.ExecutionModeOrca)} {
		for _, confirm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/confirm=%t", mode, confirm), func(t *testing.T) {
				stateRoot, record := orcaPrepareRecord(t)
				record.BranchPrepare.LinkVerified = false
				if _, err := writeIssueOps(stateRoot, record); err != nil {
					t.Fatal(err)
				}
				fake := readyOrcaFake()

				got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
					ID: record.ID, Mode: mode, CWD: record.Repo, Confirm: confirm,
					Actor: executionActor("codex", "coordinator"), OwnerHost: "codex",
				}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
				if err != nil {
					t.Fatal(err)
				}
				if got.ResolvedMode != string(model.ExecutionModeOrca) {
					t.Fatalf("resolved mode = %q, want orca", got.ResolvedMode)
				}
				if !confirm {
					if got.Execution != nil || fake.prepareCalls != 0 || fake.launchCalls != 0 {
						t.Fatalf("preview가 execution 또는 Orca mutation을 만들었다: %#v", got)
					}
					return
				}
				if got.Execution == nil || got.Execution.Orca == nil || got.Execution.Pending != nil {
					t.Fatalf("GitHub linked branch 생성 전 Orca prepare가 완료돼야 한다: %#v", got.Execution)
				}
				if fake.prepareCalls != 1 || fake.launchCalls != 1 {
					t.Fatalf("Orca stage calls = prepare:%d launch:%d, want 1/1", fake.prepareCalls, fake.launchCalls)
				}
				prompt, readErr := os.ReadFile(got.OwnerPromptPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				for _, want := range []string{"issueops branch prepare", "--link-verified"} {
					if !strings.Contains(string(prompt), want) {
						t.Fatalf("owner prompt가 branch link 후속 명령 %q을 포함하지 않는다", want)
					}
				}
			})
		}
	}
}

func TestExecutionOrcaPrepareRejectsUnverifiedGitLabBranch(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = record.IssueURL
	record.BranchPrepare.LinkVerified = false
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	fake := readyOrcaFake()

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err == nil || !strings.Contains(err.Error(), "verified branch issue identity") {
		t.Fatalf("미검증 GitLab prepare error = %v", err)
	}
	if fake.prepareCalls != 0 || fake.launchCalls != 0 {
		t.Fatalf("거부된 GitLab prepare가 Orca mutation을 실행했다: %#v", fake)
	}
}

func TestExecutionOrcaPrepareAppliesHostImplementerDefaults(t *testing.T) {
	cases := []struct {
		host       string
		wantModel  string
		wantEffort string
	}{
		{host: "codex", wantModel: port.IssueOpsImplementerModelCodex, wantEffort: port.IssueOpsImplementerEffortCodex},
		{host: "claude", wantModel: "claude-sonnet-5", wantEffort: port.IssueOpsImplementerEffortClaude},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			stateRoot, record := orcaPrepareRecord(t)
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.prepare = func(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
				if request.Model != tc.wantModel || request.Effort != tc.wantEffort {
					t.Fatalf("probe request must carry host implementer defaults: %#v", request)
				}
				if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
					return port.ExecutionOrcaWorkspaceReceipt{}, err
				}
				return executionOrcaWorkspaceReceipt(workspace), nil
			}
			got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: tc.host,
			}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
			if err != nil {
				t.Fatal(err)
			}
			if got.Execution == nil || got.Execution.Orca == nil {
				t.Fatalf("prepare must record Orca binding: %#v", got)
			}
			if got.Execution.Orca.OwnerModel != tc.wantModel || got.Execution.Orca.OwnerEffort != tc.wantEffort {
				t.Fatalf("empty owner model/effort must resolve to host implementer defaults: %#v", got.Execution.Orca)
			}
		})
	}
}

func TestExecutionGitLabOrcaSealsIssueMetadataAndUsesOrca(t *testing.T) {
	for _, mode := range []string{ExecutionModeAuto, string(model.ExecutionModeOrca)} {
		for _, confirm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/confirm=%t", mode, confirm), func(t *testing.T) {
				stateRoot, record := executionPrepareRecord(t)
				record.BranchPrepare.Provider = "gitlab"
				record.BranchPrepare.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/69"
				record.IssueURL = record.BranchPrepare.IssueURL
				if _, err := writeIssueOps(stateRoot, record); err != nil {
					t.Fatal(err)
				}
				orca := readyOrcaFake()
				var marker string
				prepare := orca.prepare
				orca.prepare = func(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
					marker = request.Marker
					return prepare(workspace, request)
				}
				direct := &executionDirectCountingFake{}
				got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
					ID: record.ID, Mode: mode, CWD: record.Repo, Confirm: confirm,
					Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
				}, ExecutionPrepareDependencies{Direct: direct, Orca: orca, ReadIssue: executionIssueSnapshotReader})
				if err != nil || got.ResolvedMode != "orca" || got.FallbackCode != "" {
					t.Fatalf("GitLab 준비는 Orca로 해석돼야 한다: result=%#v err=%v", got, err)
				}
				if orca.probeCalls != 1 || direct.calls != 0 {
					t.Fatalf("GitLab Orca 판정은 probe 한 번만 사용하고 direct를 호출하지 않아야 한다: orca=%#v direct=%d", orca, direct.calls)
				}
				current, readErr := ReadIssueOps(stateRoot, record.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !confirm {
					if current.Execution != nil {
						t.Fatalf("preview가 durable execution을 바꿨다: %#v", current.Execution)
					}
					if orca.prepareCalls != 0 || orca.launchCalls != 0 {
						t.Fatalf("preview가 Orca mutation을 실행했다: %#v", orca)
					}
					return
				}
				if current.Execution == nil || current.Execution.Mode != model.ExecutionModeOrca || current.Execution.Orca == nil {
					t.Fatalf("confirm이 Orca execution을 봉인하지 못했다: %#v", current.Execution)
				}
				for _, want := range []string{"provider=gitlab", "issue=69"} {
					if !strings.Contains(marker, want) {
						t.Fatalf("GitLab IID marker %q에 %q가 없다", marker, want)
					}
				}
			})
		}
	}
}

func TestExecutionOrcaAmbiguityNeverFallsBackOrRepeatsExternalMutation(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	direct := &executionDirectCountingFake{}
	fake := &executionOrcaFake{
		probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true},
		prepare: func(port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
			return port.ExecutionOrcaWorkspaceReceipt{}, errors.New("ambiguous external outcome")
		},
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	if _, err := PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Direct: direct, Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("ambiguous Orca outcome must require reconcile")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution == nil || pending.Execution.Mode != model.ExecutionModeOrca || pending.Execution.Pending == nil {
		t.Fatalf("ambiguous outcome must remain Orca with a durable pending intent: %#v", pending.Execution)
	}
	if _, err := PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Direct: direct, Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("second prepare must direct the caller to reconcile")
	}
	if fake.prepareCalls != 1 || direct.calls != 0 {
		t.Fatalf("external ambiguity repeated mutation or fell back: orca=%d direct=%d", fake.prepareCalls, direct.calls)
	}
}

func TestExecutionConcurrentOrcaPrepareInvokesDriverOnce(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	entered, release := make(chan struct{}), make(chan struct{})
	fake := &executionOrcaFake{
		probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true},
		prepare: func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
			close(entered)
			<-release
			return port.ExecutionOrcaWorkspaceReceipt{}, errors.New("interrupted")
		},
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	var firstErr error
	done := make(chan struct{})
	go func() {
		_, firstErr = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
		close(done)
	}()
	<-entered
	if _, err := PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("concurrent retry must observe pending intent and stop")
	}
	close(release)
	<-done
	if firstErr == nil || fake.prepareCalls != 1 {
		t.Fatalf("driver must be invoked once: calls=%d firstErr=%v", fake.prepareCalls, firstErr)
	}
}

type executionOrcaFake struct {
	mu           sync.Mutex
	probe        port.ExecutionOrcaProbeResult
	prepare      func(port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error)
	launch       func(port.ExecutionOrcaWorkspaceReceipt, port.ExecutionOrcaProbeRequest, port.ExecutionOrcaLaunchRequest) (port.ExecutionOrcaReceipt, error)
	inspect      func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error)
	invoke       func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error)
	prepareCalls int
	launchCalls  int
	probeCalls   int
}

func (f *executionOrcaFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	f.probeCalls++
	return f.probe, nil
}

func (f *executionOrcaFake) InspectIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if f.inspect != nil {
		return f.inspect(request)
	}
	return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
}

func (f *executionOrcaFake) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if f.invoke != nil {
		return f.invoke(request)
	}
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		f.mu.Lock()
		f.prepareCalls++
		f.mu.Unlock()
		var prepared port.ExecutionOrcaWorkspaceReceipt
		var err error
		if f.prepare != nil {
			prepared, err = f.prepare(request.Workspace, request.Probe)
		} else {
			prepared = executionOrcaWorkspaceReceipt(request.Workspace)
		}
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		return port.ExecutionOrcaIntentReceipt{Workspace: &prepared}, nil
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "terminal-1"}, nil
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-1"}, nil
	case port.ExecutionOrcaIntentDispatch:
		f.mu.Lock()
		f.launchCalls++
		f.mu.Unlock()
		if f.launch != nil {
			receipt, err := f.launch(*request.Prepared, request.Probe, *request.Launch)
			if err != nil {
				return port.ExecutionOrcaIntentReceipt{}, err
			}
			return port.ExecutionOrcaIntentReceipt{TaskID: receipt.TaskID, DispatchID: receipt.DispatchID}, nil
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-1"}, nil
	default:
		return port.ExecutionOrcaIntentReceipt{}, errors.New("unsupported fake Orca intent stage")
	}
}

func executionOrcaWorkspaceReceipt(workspace port.ExecutionWorkspaceRequest) port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1", WorktreeInstanceID: "instance-1",
	}
}

func executionOrcaReceipt(prepared port.ExecutionOrcaWorkspaceReceipt) port.ExecutionOrcaReceipt {
	return port.ExecutionOrcaReceipt{
		Workspace: prepared.Workspace,
		RuntimeID: prepared.RuntimeID, RepoID: prepared.RepoID, WorktreeID: prepared.WorktreeID, WorktreeInstanceID: prepared.WorktreeInstanceID,
		TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
	}
}

func executionIssueSnapshotReader(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	body := "## acceptance criteria\n\n- [ ] AC-01: first\n- [ ] AC-23: last\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\ngo vet ./...\n```\n"
	return port.ExecutionIssueSnapshot{URL: request.URL, Body: body}, nil
}

type executionDirectCountingFake struct{ calls int }

func (f *executionDirectCountingFake) ProbeAccess(context.Context, port.ExecutionWorkspaceRequest, string) (port.ExecutionWorkspaceAccessResult, error) {
	return port.ExecutionWorkspaceAccessResult{Allowed: true}, nil
}

func (f *executionDirectCountingFake) Prepare(_ context.Context, req port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	f.calls++
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: req.SourceRoot, Root: req.Root, Branch: req.Branch,
		BaseHead: req.BaseHead, Driver: "git", Exists: req.Confirm,
	}, nil
}
