package orca

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/port"
)

type ExecutionProvisioner struct {
	client executionClient
	// terminalSettleBudget과 terminalSettleInterval이 0이면 기본 상수를 쓴다.
	// 테스트가 대기 상한을 밀리초로 줄여 실제로 기다리지 않게 하는 지점이다.
	terminalSettleBudget   time.Duration
	terminalSettleInterval time.Duration
}

type executionClient interface {
	Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error)
	CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error)
	CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
	Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
}

type executionInventoryClient interface {
	listTerminalsInventory(context.Context, string) (executionTerminalInventory, error)
	listAllTasksInventory(context.Context) (executionTaskInventory, error)
	showDispatchInventory(context.Context, string) (executionDispatchInventory, error)
}

type executionTerminalDetailClient interface {
	showTerminalInventory(context.Context, string) (executionTerminalDetailInventory, error)
}

type executionTerminalInventory struct {
	RuntimeID string
	Rows      []port.OrcaTerminal
}

type executionTerminalDetailInventory struct {
	RuntimeID     string
	Terminal      port.OrcaTerminal
	PaneRuntimeID *int
}

type executionTaskInventory struct {
	RuntimeID string
	Rows      []port.OrcaTask
}

type executionDispatchInventory struct {
	RuntimeID string
	Dispatch  *port.OrcaDispatch
}

func NewExecution() *ExecutionProvisioner {
	return NewExecutionClient(New())
}

func NewExecutionClient(client executionClient) *ExecutionProvisioner {
	return &ExecutionProvisioner{client: client}
}

func (p *ExecutionProvisioner) Probe(ctx context.Context, req port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaProbeResult{Code: "orca_adapter_unavailable"}, nil
	}
	result, err := p.client.Probe(ctx, port.OrcaProbeRequest{Repo: req.Repo, Agent: req.Host, Provider: req.Provider})
	return port.ExecutionOrcaProbeResult{Available: result.Available, Ready: result.Ready, Code: result.Code}, err
}

func (p *ExecutionProvisioner) InspectIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionIntentInspectionRequest(req); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		rows, err := p.client.ListWorktrees(ctx, req.Workspace.SourceRoot)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
		for _, row := range rows {
			if !samePath(row.Path, req.Workspace.Root) && strings.TrimSpace(row.Comment) != req.Marker {
				continue
			}
			if err := validateExecutionWorktree(row, req.Workspace, req.Probe); err != nil {
				return port.ExecutionOrcaIntentInventory{}, err
			}
			receipt := executionWorkspaceReceipt(req.Workspace, row)
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{Workspace: &receipt})
		}
		return executionIntentInventory(candidates), nil
	case port.ExecutionOrcaIntentTerminal:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
		for _, row := range inventory.Rows {
			if strings.TrimSpace(row.Title) != req.Marker && strings.TrimSpace(row.StableTabTitle) != req.Marker {
				continue
			}
			if err := validateExecutionIntentTerminal(row, *req.Prepared, req.Marker); err != nil {
				return port.ExecutionOrcaIntentInventory{}, err
			}
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TerminalPTYID: row.PTYID})
		}
		return executionIntentInventory(candidates), nil
	case port.ExecutionOrcaIntentTask:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.listAllTasksInventory(ctx)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		title := executionTaskTitle(req.Marker, req.Launch.PromptSHA256)
		candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
		for _, row := range inventory.Rows {
			candidateTitle := strings.TrimSpace(row.Title)
			if candidateTitle != title {
				continue
			}
			if err := validateExecutionIntentTask(row, *req.Prepared, candidateTitle, req.Workspace.Branch); err != nil {
				return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca owner task candidate does not match the sealed intent")
			}
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TaskID: row.ID})
		}
		return executionIntentInventory(candidates), nil
	case port.ExecutionOrcaIntentDispatch:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.showDispatchInventory(ctx, req.TaskID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if inventory.Dispatch == nil {
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
		}
		dispatch := *inventory.Dispatch
		if err := validateExecutionObservedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TaskID: dispatch.TaskID, DispatchID: dispatch.ID}}}, nil
	default:
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
}

func (p *ExecutionProvisioner) InvokeIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(fmt.Errorf("Orca client is unavailable"))
	}
	if err := validateExecutionIntentRequest(req); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(err)
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		created, err := p.client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
			Repo: req.Workspace.SourceRoot, Name: req.Workspace.Branch, BaseBranch: req.Workspace.BaseHead,
			ParentWorktree: req.Workspace.ParentWorktree,
			Provider:       req.Probe.Provider, Issue: req.Probe.Issue, Comment: req.Marker,
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		if err := validateExecutionWorktree(created, req.Workspace, req.Probe); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "worktree_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		receipt := executionWorkspaceReceipt(req.Workspace, created)
		return port.ExecutionOrcaIntentReceipt{Workspace: &receipt}, nil
	case port.ExecutionOrcaIntentTerminal:
		created, err := p.client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
			WorktreeID: req.Prepared.WorktreeID, Agent: req.Probe.Host, Model: req.Probe.Model, ReasoningEffort: req.Probe.Effort,
			Title: req.Marker, AllowCodexHookTrustBypass: req.Probe.Host == "codex",
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		created, err = p.reconcileCreatedTerminal(ctx, created, *req.Prepared, req.Marker)
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: created.PTYID}, nil
	case port.ExecutionOrcaIntentTask:
		created, err := p.client.CreateTask(ctx, port.OrcaCreateTaskRequest{
			Spec: req.Launch.Prompt, Title: executionTaskTitle(req.Marker, req.Launch.PromptSHA256), DisplayName: req.Workspace.Branch,
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		title := executionTaskTitle(req.Marker, req.Launch.PromptSHA256)
		if err := validateExecutionIntentTask(created, *req.Prepared, title, req.Workspace.Branch); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "task_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: created.ID}, nil
	case port.ExecutionOrcaIntentDispatch:
		terminal, err := p.resolveIntentTerminal(ctx, req)
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(err)
		}
		dispatch, err := p.client.Dispatch(ctx, port.OrcaDispatchRequest{TaskID: req.TaskID, ToHandle: terminal.Handle, Inject: true, ReturnPreamble: true})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		if err := validateExecutionInvokedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID, terminal.Handle); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "dispatch_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: dispatch.TaskID, DispatchID: dispatch.ID}, nil
	default:
		return port.ExecutionOrcaIntentReceipt{}, executionPreflightError(fmt.Errorf("unsupported stage %q", req.Stage))
	}
}

func executionPreflightError(err error) error {
	if err == nil {
		return nil
	}
	return &port.OrcaError{Code: "intent_preflight_rejected", Detail: err.Error(), Invoked: false}
}

func (p *ExecutionProvisioner) PrepareWorkspace(ctx context.Context, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionPrepare(workspace, req); err != nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, err
	}
	worktree, err := p.prepareWorktree(ctx, workspace, req)
	if err != nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, err
	}
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: filepath.Clean(worktree.Path), Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree,
			Driver: "orca", Exists: true,
		},
		RuntimeID: worktree.RuntimeID, RepoID: worktree.RepoID, WorktreeID: worktree.ID,
		WorktreeInstanceID: worktree.InstanceID,
	}, nil
}

func (p *ExecutionProvisioner) LaunchOwner(ctx context.Context, prepared port.ExecutionOrcaWorkspaceReceipt, req port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) (port.ExecutionOrcaReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaReceipt{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionOwnerLaunch(prepared, req, launch); err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	terminal, err := p.client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
		WorktreeID: prepared.WorktreeID, Agent: req.Host, Model: req.Model, ReasoningEffort: req.Effort,
		Title: req.Marker, AllowCodexHookTrustBypass: req.Host == "codex",
	})
	if err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	terminal, err = p.reconcileCreatedTerminal(ctx, terminal, prepared, req.Marker)
	if err != nil {
		return port.ExecutionOrcaReceipt{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: err.Error(), Invoked: true}
	}
	task, err := p.client.CreateTask(ctx, port.OrcaCreateTaskRequest{
		Spec: launch.Prompt, Title: executionTaskTitle(req.Marker, launch.PromptSHA256), DisplayName: prepared.Workspace.Branch,
	})
	if err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	dispatch, err := p.client.Dispatch(ctx, port.OrcaDispatchRequest{
		TaskID: task.ID, ToHandle: terminal.Handle, Inject: true, ReturnPreamble: true,
	})
	if err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	if err := validateExecutionLaunch(prepared.WorktreeID, terminal, task, dispatch); err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	return port.ExecutionOrcaReceipt{
		Workspace: prepared.Workspace,
		RuntimeID: prepared.RuntimeID, RepoID: prepared.RepoID, WorktreeID: prepared.WorktreeID,
		WorktreeInstanceID: prepared.WorktreeInstanceID, TaskID: task.ID, DispatchID: dispatch.ID, TerminalPTYID: terminal.PTYID,
	}, nil
}

func (p *ExecutionProvisioner) InspectOwner(ctx context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	client, ok := p.client.(executionInventoryClient)
	if !ok {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner inventory is unavailable")
	}
	terminals, err := client.listTerminalsInventory(ctx, req.WorktreeID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	currentRuntime, err := executionOwnerInventoryRuntime(terminals.RuntimeID, req)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	result := port.ExecutionOrcaOwnerInventory{RuntimeID: currentRuntime}
	for _, terminal := range terminals.Rows {
		if terminal.PTYID != req.TerminalPTYID {
			continue
		}
		if result.TerminalID != "" {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		if terminal.RuntimeID != currentRuntime {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal runtime identity changed")
		}
		result.TerminalID = terminal.PTYID
		detailClient, ok := p.client.(executionTerminalDetailClient)
		if !ok {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal detail inventory is unavailable")
		}
		detail, err := detailClient.showTerminalInventory(ctx, terminal.Handle)
		if err != nil {
			return port.ExecutionOrcaOwnerInventory{}, err
		}
		if err := validateExecutionInventoryRuntime(detail.RuntimeID, currentRuntime); err != nil {
			return port.ExecutionOrcaOwnerInventory{}, err
		}
		if detail.Terminal.RuntimeID != currentRuntime ||
			detail.Terminal.Handle != terminal.Handle ||
			detail.Terminal.PTYID != terminal.PTYID ||
			detail.Terminal.WorktreeID != req.WorktreeID {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal detail identity changed")
		}
		if detail.PaneRuntimeID == nil {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal pane runtime identity is unavailable")
		}
		// Orca 재시작 뒤 terminal list에는 connected/writable인 장부 행이 남을 수 있다.
		// terminal show의 음수 paneRuntimeId는 현재 렌더러에 실제 pane이 없다는 증거다.
		result.TerminalLive = detail.Terminal.Connected && detail.Terminal.Writable && *detail.PaneRuntimeID >= 0
	}
	tasks, err := client.listAllTasksInventory(ctx)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(tasks.RuntimeID, currentRuntime); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	taskFound := false
	for _, task := range tasks.Rows {
		if task.ID != req.TaskID {
			continue
		}
		if taskFound {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner task inventory is ambiguous")
		}
		taskFound = true
		if task.RuntimeID != currentRuntime {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner task runtime identity changed")
		}
		result.TaskStatus = strings.ToLower(strings.TrimSpace(task.Status))
		result.TaskLive = !executionTerminalTaskStatus(result.TaskStatus)
	}
	dispatch, err := client.showDispatchInventory(ctx, req.TaskID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(dispatch.RuntimeID, currentRuntime); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if dispatch.Dispatch == nil {
		if !taskFound {
			return result, nil
		}
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner dispatch is absent")
	}
	// 거울상의 모순도 같은 ambiguity다. task 행이 완전 목록에서 사라졌는데
	// dispatch 행이 남아 있으면 어느 쪽 인벤토리를 믿어야 하는지 알 수 없다.
	// 예전에는 dispatch 상태가 비종결일 때만 우연히 막혔다 — 상태 어휘를 판정에서
	// 뺀 뒤로는 행의 존재만으로 막는다. 이 검사는 어휘에 의존하지 않는다.
	if !taskFound {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner task is absent while its dispatch row remains")
	}
	row := *dispatch.Dispatch
	if row.RuntimeID != currentRuntime || row.ID != req.DispatchID || row.TaskID != req.TaskID {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner dispatch identity changed")
	}
	// dispatch 상태는 보고만 하고 liveness 판정에는 넣지 않는다(#147 결정 A).
	//
	// 어휘를 몰라서가 아니다. Orca가 공개 저장소이고 그 정의가 소스에 있다(#171):
	//
	//	src/main/runtime/orchestration/types.ts
	//	export type DispatchStatus = 'pending' | 'dispatched' | 'completed' | 'failed' | 'circuit_broken'
	//
	//	src/main/runtime/orchestration/db.ts (dispatch_contexts 스키마)
	//	CHECK(status IN ('pending', 'dispatched', 'completed', 'failed', 'circuit_broken'))
	//
	// core/operationalhealth는 그 어휘를 knownDispatchStatus·settledDispatchStatus로
	// 쓴다. 여기서 쓰지 않는 것은 질문이 다르기 때문이다 — 그쪽은 "이 자원이
	// 잔여물인가"를 묻고 여기는 "이 소유자가 살아 있는가"를 묻는다. 후자에는 아래
	// 이유로 dispatch 상태가 정보를 더하지 않는다.
	//
	// (예전 주석은 "어휘를 확인할 방법이 없다"고 했다. #142 시점에는 CLI 응답만
	// 봤기 때문이고, 그 진술은 #171의 소스 실측으로 무효가 됐다.)
	//
	// 뺀 근거는 손실의 크기다. 이 게이트가 막아야 할 해악은 worker가 아직 자원을
	// 붙든 채로 레코드가 지워지는 것인데, 그 상태는 확인된 증거 셋이 이미 덮는다:
	// 작업 중인 worker의 task 상태는 비종결이라 TaskLive가 잡고, terminal 행이
	// 인벤토리에 있는 한 TerminalLive가 connected/writable 실관측으로 잡으며,
	// 쓰기 권한 자체는 lease가 쥐고 abandon의 lease_terminal 게이트가 독립적으로
	// 막는다. 따라서 dispatch 상태가 추가로 잡아내던 것은 task가 이미 종결되고
	// terminal도 죽은 뒤 남은 장부 행뿐이다 — 잃을 소유자가 없는 잔여물이다.
	//
	// Orca 소스가 그 주장을 증명한다. db.ts의 failDispatch가 두 상태를 함께 움직인다:
	//
	//	const newStatus: DispatchStatus = newFailureCount >= 3 ? 'circuit_broken' : 'failed'
	//	const taskStatus: TaskStatus = newStatus === 'circuit_broken' ? 'failed' : 'ready'
	//
	// circuit_broken이면 task는 failed(종결)이고, failed면 task는 ready(비종결)다.
	// completed는 worker_done이 task와 dispatch를 함께 완료로 만든다. 즉 dispatch가
	// 살아 있음을 말하는 모든 경우에 task도 비종결이므로 TaskLive가 이미 잡는다.
	// dispatch 상태를 판정에 넣는 것은 같은 사실을 두 번 묻는 것이었다.
	//
	// TerminalLive의 경계는 분명히 해둔다: worktree 스코프 목록에 terminal 행이
	// 없으면 이 값은 조용히 false다. 그 창은 이 변경 이전부터 있었다. 구 검사가
	// 그것을 좁혀주지도 않았다 — 비종결 dispatch 상태에서만 우연히 막았고 종결값은
	// 그때도 통과시켰다.
	//
	// 남기는 것과 빼는 것을 구분한다: 위의 identity/absence/모순 검사는 그대로 두어
	// dispatch 조회 자체는 계속 fail-closed다. 판정에서 빠지는 것은 상태 어휘뿐이고,
	// DispatchStatus는 진단용으로 계속 보고한다.
	result.DispatchStatus = strings.ToLower(strings.TrimSpace(row.Status))
	return result, nil
}

func executionOwnerInventoryRuntime(observed string, req port.ExecutionOrcaOwnerInventoryRequest) (string, error) {
	observed = strings.TrimSpace(observed)
	sealed := strings.TrimSpace(req.RuntimeID)
	if observed == "" || sealed == "" || observed != sealed && !req.AllowRuntimeRollover {
		return "", fmt.Errorf("Orca inventory runtime identity changed")
	}
	return observed, nil
}

// executionTerminalTaskStatus reports whether a status means the work is over.
//
// The vocabulary comes from the Orca CLI, not from another Go definition:
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// Only completed and failed are terminal. The other four can still hold or
// acquire a worker. This set used to also list complete/cancelled/canceled/
// closed, which Orca rejects outright — carrying values that cannot be observed
// obscured where the vocabulary comes from without defending against anything,
// since an unrecognised status already falls through to "not terminal" (#145).
//
// core/operationalhealth mirrors this set in settledTaskStatus. The two must
// agree, and the CLI decides on what.
func executionTerminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}

func (p *ExecutionProvisioner) prepareWorktree(ctx context.Context, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) (port.OrcaWorktree, error) {
	rows, err := p.client.ListWorktrees(ctx, workspace.SourceRoot)
	if err != nil {
		return port.OrcaWorktree{}, err
	}
	candidates := make([]port.OrcaWorktree, 0, 1)
	for _, row := range rows {
		if samePath(row.Path, workspace.Root) || strings.TrimSpace(row.Comment) == req.Marker {
			candidates = append(candidates, row)
		}
	}
	if len(candidates) > 1 {
		return port.OrcaWorktree{}, fmt.Errorf("Orca worktree reconciliation is ambiguous")
	}
	if len(candidates) == 1 {
		candidate := candidates[0]
		if err := validateExecutionWorktree(candidate, workspace, req); err != nil {
			return port.OrcaWorktree{}, err
		}
		return candidate, nil
	}
	created, err := p.client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: workspace.SourceRoot, Name: workspace.Branch, BaseBranch: workspace.BaseHead,
		ParentWorktree: workspace.ParentWorktree,
		Provider:       req.Provider, Issue: req.Issue, Comment: req.Marker,
	})
	if err != nil {
		return port.OrcaWorktree{}, err
	}
	if err := validateExecutionWorktree(created, workspace, req); err != nil {
		return port.OrcaWorktree{}, err
	}
	return created, nil
}

// executionTerminalSettleBudget과 executionTerminalSettleInterval은 Orca가 만든
// 터미널의 UI 상태가 반영될 때까지 다시 읽는 상한과 간격이다.
//
// Orca는 터미널을 만든 뒤 탭 제목을 비동기로 설정한다. 마커는 그 탭 제목에 있고
// (터미널 제목은 에이전트가 자기 상태로 덮어쓴다), StableTabTitle은 visualLayouts
// 응답에서 온다. 실측에서 생성 08:32:59.706 → 검증 실패 08:33:02.607로 3초 창에
// 걸렸고, 그 창 때문에 `prepare --mode orca --confirm`이 실환경에서 한 번도
// 완주하지 못했다(이슈 #169, #70·#71이 미검증으로 남긴 경로).
//
// 상한을 실측보다 넉넉하게 잡는 것은 비대칭 때문이다: 넘으면 종전과 같은
// terminal_identity_mismatch로 떨어지므로 과하게 잡아도 손해가 없고, 부족하면
// 지금과 같다. nativeProcessProbeTimeout(execution_process.go)이 같은 성격의
// 상수 선례다.
const (
	executionTerminalSettleBudget   = 12 * time.Second
	executionTerminalSettleInterval = 400 * time.Millisecond
)

func (p *ExecutionProvisioner) reconcileCreatedTerminal(ctx context.Context, created port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) (port.OrcaTerminal, error) {
	if err := validateExecutionIntentTerminal(created, prepared, marker); err == nil {
		return created, nil
	}
	client, ok := p.client.(executionInventoryClient)
	if !ok || strings.TrimSpace(created.Handle) == "" {
		return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal does not match the sealed intent")
	}
	// 반복하는 것은 조회다. CreateTerminal은 이미 한 번 실행됐고 다시 부르지
	// 않는다 — 실패한 mutation을 재시도하면 #90의 잔여물 문제를 되풀이한다.
	budget, interval := p.terminalSettleWindow()
	deadline := time.Now().Add(budget)
	attempts, lastErr := 0, error(nil)
	for {
		attempts++
		inventory, err := client.listTerminalsInventory(ctx, prepared.WorktreeID)
		if err != nil {
			return port.OrcaTerminal{}, err
		}
		candidate, err := executionSoleCreatedTerminal(inventory.Rows, created)
		if err != nil {
			// 모호함과 부재는 대기로 해소되지 않는다 — 그대로 알린다.
			return port.OrcaTerminal{}, err
		}
		lastErr = validateExecutionIntentTerminal(*candidate, prepared, marker)
		if lastErr == nil {
			return *candidate, nil
		}
		if !time.Now().Add(interval).Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return port.OrcaTerminal{}, ctx.Err()
		case <-time.After(interval):
		}
	}
	// 몇 번 얼마나 기다렸는지를 남긴다. 조용한 재시도는 다음 사람이 같은 타이밍
	// 문제를 처음부터 다시 발견하게 만든다.
	return port.OrcaTerminal{}, fmt.Errorf("%w (attempt %d over %s waiting for the Orca tab title to settle)",
		lastErr, attempts, budget)
}

// terminalSettleWindow는 대기 상한과 간격을 돌려준다. 필드가 0이면 기본 상수를
// 쓴다 — 테스트가 상한을 밀리초 단위로 줄여 12초를 실제로 기다리지 않게 하려고
// 오버라이드 지점을 둔다.
func (p *ExecutionProvisioner) terminalSettleWindow() (time.Duration, time.Duration) {
	budget, interval := executionTerminalSettleBudget, executionTerminalSettleInterval
	if p.terminalSettleBudget > 0 {
		budget = p.terminalSettleBudget
	}
	if p.terminalSettleInterval > 0 {
		interval = p.terminalSettleInterval
	}
	return budget, interval
}

// executionSoleCreatedTerminal은 생성 응답의 PTY가 있으면 PTY를, 없으면 handle을
// 사용해 정확히 하나를 고른다. Orca는 정상 생성 응답에서도 PTY를 생략할 수
// 있으므로 authoritative inventory에서 완전한 identity를 복구한다.
func executionSoleCreatedTerminal(rows []port.OrcaTerminal, created port.OrcaTerminal) (*port.OrcaTerminal, error) {
	var candidate *port.OrcaTerminal
	for index := range rows {
		row := rows[index]
		matches := strings.TrimSpace(created.PTYID) != "" && row.PTYID == created.PTYID
		if strings.TrimSpace(created.PTYID) == "" {
			matches = strings.TrimSpace(created.Handle) != "" && row.Handle == created.Handle
		}
		if !matches {
			continue
		}
		if candidate != nil {
			return nil, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		candidate = &row
	}
	if candidate == nil {
		return nil, fmt.Errorf("Orca owner terminal is absent")
	}
	return candidate, nil
}

func (p *ExecutionProvisioner) intentInventoryClient() (executionInventoryClient, error) {
	client, ok := p.client.(executionInventoryClient)
	if !ok {
		return nil, fmt.Errorf("Orca execution intent inventory is unavailable")
	}
	return client, nil
}

func (p *ExecutionProvisioner) resolveIntentTerminal(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.OrcaTerminal, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.OrcaTerminal{}, err
	}
	var candidate *port.OrcaTerminal
	for index := range inventory.Rows {
		row := inventory.Rows[index]
		if row.PTYID != req.TerminalPTYID {
			continue
		}
		if candidate != nil {
			return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		candidate = &row
	}
	if candidate == nil {
		return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal is absent")
	}
	if err := validateExecutionTerminalReceipt(*candidate, *req.Prepared); err != nil {
		return port.OrcaTerminal{}, err
	}
	return *candidate, nil
}

func executionWorkspaceReceipt(workspace port.ExecutionWorkspaceRequest, worktree port.OrcaWorktree) port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: filepath.Clean(worktree.Path), Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree,
			Driver: "orca", Exists: true,
		},
		RuntimeID: worktree.RuntimeID, RepoID: worktree.RepoID, WorktreeID: worktree.ID, WorktreeInstanceID: worktree.InstanceID,
	}
}

func executionIntentInventory(candidates []port.ExecutionOrcaIntentReceipt) port.ExecutionOrcaIntentInventory {
	if candidates == nil {
		candidates = []port.ExecutionOrcaIntentReceipt{}
	}
	return port.ExecutionOrcaIntentInventory{Candidates: candidates, AuthoritativeZero: len(candidates) == 0}
}

func executionTaskTitle(marker, promptSHA256 string) string {
	marker = strings.TrimSpace(marker)
	lifecycleID := ""
	for _, field := range strings.Fields(marker) {
		if value, ok := strings.CutPrefix(field, "lifecycle="); ok {
			lifecycleID = value
			break
		}
	}
	intentDigest := sha256.Sum256([]byte(marker + "\n" + strings.ToLower(strings.TrimSpace(promptSHA256))))
	return fmt.Sprintf("agent-harness issueops-v1 lifecycle=%s intent=%x", lifecycleID, intentDigest[:8])
}

func validateExecutionIntentRequest(req port.ExecutionOrcaIntentRequest) error {
	if req.Marker == "" || req.Marker != req.Probe.Marker {
		return fmt.Errorf("Orca intent marker does not match the sealed operation")
	}
	if err := validateExecutionPrepare(req.Workspace, req.Probe); err != nil {
		return err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if req.Prepared != nil || req.Launch != nil || req.TerminalPTYID != "" || req.TaskID != "" {
			return fmt.Errorf("worktree intent contains a later-stage receipt")
		}
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch:
		if req.Prepared == nil {
			return fmt.Errorf("owner intent requires a sealed worktree receipt")
		}
		if err := validateExecutionOwnerLaunch(*req.Prepared, req.Probe, executionRequiredLaunch(req)); err != nil {
			return err
		}
		if req.Stage == port.ExecutionOrcaIntentTerminal && (req.TerminalPTYID != "" || req.TaskID != "") {
			return fmt.Errorf("terminal intent contains a later-stage receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentTask && (req.TerminalPTYID == "" || req.TaskID != "") {
			return fmt.Errorf("task intent requires exactly one terminal receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentDispatch && (req.TerminalPTYID == "" || req.TaskID == "") {
			return fmt.Errorf("dispatch intent requires terminal and task receipts")
		}
	default:
		return fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
	return nil
}

// validateExecutionIntentInspectionRequest는 외부 mutation 없이 Orca
// 인벤토리만 조회할 때의 봉인 메타데이터를 검증한다. worktree가 이미 정리된
// 복구 상황에서는 prompt/context 파일을 다시 읽을 수 없으므로 경로와 digest
// 봉인만 확인한다. InvokeIntent는 계속 validateExecutionIntentRequest를 사용해
// 실제 파일 내용까지 검증한다.
func validateExecutionIntentInspectionRequest(req port.ExecutionOrcaIntentRequest) error {
	if req.Marker == "" || req.Marker != req.Probe.Marker {
		return fmt.Errorf("Orca intent marker does not match the sealed operation")
	}
	if err := validateExecutionPrepare(req.Workspace, req.Probe); err != nil {
		return err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if req.Prepared != nil || req.Launch != nil || req.TerminalPTYID != "" || req.TaskID != "" {
			return fmt.Errorf("worktree intent contains a later-stage receipt")
		}
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch:
		if err := validateExecutionInspectionOwnerEnvelope(req); err != nil {
			return err
		}
		if req.Stage == port.ExecutionOrcaIntentTerminal && (req.TerminalPTYID != "" || req.TaskID != "") {
			return fmt.Errorf("terminal intent contains a later-stage receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentTask && (req.TerminalPTYID == "" || req.TaskID != "") {
			return fmt.Errorf("task intent requires exactly one terminal receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentDispatch && (req.TerminalPTYID == "" || req.TaskID == "") {
			return fmt.Errorf("dispatch intent requires terminal and task receipts")
		}
	default:
		return fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
	return nil
}

func validateExecutionInspectionOwnerEnvelope(req port.ExecutionOrcaIntentRequest) error {
	if req.Prepared == nil || req.Launch == nil {
		return fmt.Errorf("owner intent requires sealed worktree and launch receipts")
	}
	prepared := req.Prepared
	if strings.TrimSpace(prepared.WorktreeID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" ||
		strings.TrimSpace(prepared.RepoID) == "" {
		return fmt.Errorf("owner intent worktree receipt is incomplete")
	}
	launch := req.Launch
	if !validExecutionSHA256(launch.PromptSHA256) || !validExecutionSHA256(launch.ContextPacketSHA256) ||
		!executionPathInsideRoot(req.Workspace.Root, launch.PromptPath) ||
		!executionPathInsideRoot(req.Workspace.Root, launch.ContextPacketPath) {
		return fmt.Errorf("owner intent launch receipt is incomplete")
	}
	return nil
}

func validExecutionSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func executionPathInsideRoot(root, path string) bool {
	root, path = filepath.Clean(strings.TrimSpace(root)), filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func executionRequiredLaunch(req port.ExecutionOrcaIntentRequest) port.ExecutionOrcaLaunchRequest {
	if req.Launch == nil {
		return port.ExecutionOrcaLaunchRequest{}
	}
	return *req.Launch
}

func validateExecutionIntentTerminal(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) error {
	if err := validateExecutionTerminalReceipt(terminal, prepared); err != nil {
		return err
	}
	if strings.TrimSpace(terminal.Title) != marker && strings.TrimSpace(terminal.StableTabTitle) != marker {
		return fmt.Errorf("Orca owner terminal does not match the sealed intent")
	}
	return nil
}

func validateExecutionTerminalReceipt(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt) error {
	if strings.TrimSpace(terminal.Handle) == "" || strings.TrimSpace(terminal.PTYID) == "" || terminal.WorktreeID != prepared.WorktreeID ||
		strings.TrimSpace(prepared.RuntimeID) == "" || terminal.RuntimeID != prepared.RuntimeID || !terminal.Connected || !terminal.Writable {
		return fmt.Errorf("Orca owner terminal does not match the sealed intent")
	}
	return nil
}

func validateExecutionIntentTask(task port.OrcaTask, prepared port.ExecutionOrcaWorkspaceReceipt, title, displayName string) error {
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || task.RuntimeID != prepared.RuntimeID ||
		strings.TrimSpace(task.Title) != strings.TrimSpace(title) || strings.TrimSpace(task.DisplayName) != strings.TrimSpace(displayName) {
		return fmt.Errorf("Orca owner task does not match the sealed runtime and launch identity")
	}
	return nil
}

func validateExecutionInvokedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID, terminalHandle string) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(terminalHandle) == "" || dispatch.AssigneeHandle != terminalHandle || !dispatch.Injected {
		return fmt.Errorf("Orca dispatch does not match the sealed task and terminal")
	}
	return nil
}

func validateExecutionObservedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID string) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(dispatch.AssigneeHandle) == "" || strings.TrimSpace(dispatch.Status) == "" {
		return fmt.Errorf("Orca dispatch does not match the sealed task identity")
	}
	return nil
}

func validateExecutionInventoryRuntime(observed, sealed string) error {
	if strings.TrimSpace(observed) == "" || strings.TrimSpace(sealed) == "" || observed != sealed {
		return fmt.Errorf("Orca inventory runtime identity changed")
	}
	return nil
}

func validateExecutionPrepare(workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(workspace.LifecycleID) == "" || !filepath.IsAbs(workspace.SourceRoot) || !filepath.IsAbs(workspace.Root) || strings.TrimSpace(workspace.Branch) == "" || strings.TrimSpace(workspace.BaseHead) == "" {
		return fmt.Errorf("Orca prepare requires an exact lifecycle and workspace identity")
	}
	if req.Host != "codex" && req.Host != "claude" || strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Marker) == "" {
		return fmt.Errorf("Orca prepare requires codex or claude with explicit model and marker")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "github" && provider != "gitlab" {
		return fmt.Errorf("Orca prepare requires github or gitlab issue identity")
	}
	if req.Issue <= 0 {
		return fmt.Errorf("Orca %s prepare requires a positive issue number", provider)
	}
	markerProvider, providerOK := executionMarkerField(req.Marker, "provider")
	markerIssue, issueOK := executionMarkerField(req.Marker, "issue")
	if !providerOK || markerProvider != provider || !issueOK || markerIssue != strconv.Itoa(req.Issue) {
		return fmt.Errorf("Orca %s prepare marker does not seal the exact provider and issue number", provider)
	}
	if parent := strings.TrimSpace(workspace.ParentWorktree); parent != "" &&
		(!filepath.IsAbs(parent) || samePath(parent, workspace.SourceRoot) || samePath(parent, workspace.Root)) {
		return fmt.Errorf("Orca parent worktree must be an isolated absolute path")
	}
	return nil
}

func validateExecutionWorktree(row port.OrcaWorktree, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(row.RuntimeID) == "" || strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.RepoID) == "" || !samePath(row.Path, workspace.Root) || strings.TrimSpace(row.Branch) != workspace.Branch || strings.TrimSpace(row.Head) != workspace.BaseHead || strings.TrimSpace(row.Comment) != req.Marker {
		return fmt.Errorf("Orca worktree receipt does not match the canonical workspace identity")
	}
	if req.Provider == "github" && row.Issue != req.Issue {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitHub issue")
	}
	// 공개 Orca CLI에는 GitLab IID 쓰기 flag가 없다. 정확한 comment marker를
	// 필수 봉인으로 사용하고, native 필드가 관찰되면 추가 교차검증한다.
	if req.Provider == "gitlab" && row.GitLabIssue != nil && *row.GitLabIssue != req.Issue {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitLab issue")
	}
	if strings.TrimSpace(workspace.ParentWorktree) != "" &&
		!explicitExecutionParentLineage(row, workspace.ParentWorktree) {
		return fmt.Errorf("Orca worktree receipt does not prove explicit parent lineage")
	}
	return nil
}

func explicitExecutionParentLineage(row port.OrcaWorktree, parentWorktree string) bool {
	if strings.TrimSpace(row.LineageConfidence) != "explicit" {
		return false
	}
	// create의 --parent-worktree는 explicit-cli-flag를, 이후 명시적 parent
	// 갱신은 manual-action을 기록한다. 둘 다 정확한 parent ID가 일치할 때만
	// 같은 명시적 lineage 증거로 인정한다.
	switch strings.TrimSpace(row.LineageSource) {
	case "explicit-cli-flag", "manual-action":
	default:
		return false
	}
	repoID, parentPath, ok := strings.Cut(strings.TrimSpace(row.ParentWorktreeID), "::")
	return ok && strings.TrimSpace(repoID) == strings.TrimSpace(row.RepoID) &&
		samePath(parentPath, parentWorktree)
}

func executionMarkerField(marker, name string) (string, bool) {
	prefix := name + "="
	value := ""
	seen := false
	for _, field := range strings.Fields(marker) {
		if !strings.HasPrefix(field, prefix) {
			continue
		}
		if seen {
			return "", false
		}
		seen = true
		value = strings.TrimPrefix(field, prefix)
		if value == "" {
			return "", false
		}
	}
	return value, seen
}

func validateExecutionLaunch(worktreeID string, terminal port.OrcaTerminal, task port.OrcaTask, dispatch port.OrcaDispatch) error {
	if strings.TrimSpace(terminal.Handle) == "" || terminal.WorktreeID != worktreeID || !terminal.Connected || !terminal.Writable {
		return fmt.Errorf("Orca owner terminal receipt is incomplete")
	}
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(dispatch.ID) == "" || dispatch.TaskID != task.ID || dispatch.AssigneeHandle != terminal.Handle || !dispatch.Injected {
		return fmt.Errorf("Orca task or dispatch receipt is incomplete")
	}
	return nil
}

var executionPromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)

func validateExecutionOwnerLaunch(prepared port.ExecutionOrcaWorkspaceReceipt, req port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) error {
	if strings.TrimSpace(prepared.WorktreeID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || strings.TrimSpace(prepared.RepoID) == "" {
		return fmt.Errorf("Orca workspace receipt is incomplete")
	}
	if req.Host != "codex" && req.Host != "claude" || strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("Orca owner launch requires an explicit first-party owner profile")
	}
	packet, err := readExecutionSealedFile(prepared.Workspace.Root, launch.ContextPacketPath)
	if err != nil {
		return fmt.Errorf("sealed context packet is invalid: %w", err)
	}
	if digestExecutionBytes(packet) != strings.ToLower(strings.TrimSpace(launch.ContextPacketSHA256)) {
		return fmt.Errorf("sealed context packet digest mismatch")
	}
	prompt, err := readExecutionSealedFile(prepared.Workspace.Root, launch.PromptPath)
	if err != nil {
		return fmt.Errorf("sealed owner prompt is invalid: %w", err)
	}
	if string(prompt) != launch.Prompt || digestExecutionBytes(prompt) != strings.ToLower(strings.TrimSpace(launch.PromptSHA256)) {
		return fmt.Errorf("sealed owner prompt digest mismatch")
	}
	if executionPromptPlaceholder.MatchString(launch.Prompt) || !strings.Contains(launch.Prompt, launch.ContextPacketPath) || !strings.Contains(launch.Prompt, launch.ContextPacketSHA256) {
		return fmt.Errorf("owner prompt is unresolved or does not bind the sealed packet")
	}
	return nil
}

func readExecutionSealedFile(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path must be inside the canonical worktree")
	}
	current := root
	parts := strings.Split(rel, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("path must contain only real directories and a real file")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("path ancestor must be a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 1<<20) {
			return nil, fmt.Errorf("file must be regular, private, and at most 1048576 bytes")
		}
	}
	return os.ReadFile(path)
}

func digestExecutionBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

var _ port.ExecutionOrcaProvisioner = (*ExecutionProvisioner)(nil)
var _ port.ExecutionOrcaOwnerInspector = (*ExecutionProvisioner)(nil)
