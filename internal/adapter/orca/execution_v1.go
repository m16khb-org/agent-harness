package orca

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"agent-harness/internal/port"
)

type ExecutionV1Provisioner struct {
	client executionV1Client
}

type executionV1Client interface {
	Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error)
	CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error)
	CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
	Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
}

type executionV1InventoryClient interface {
	listTerminalsInventory(context.Context, string) (executionV1TerminalInventory, error)
	listAllTasksInventory(context.Context) (executionV1TaskInventory, error)
	showDispatchInventory(context.Context, string) (executionV1DispatchInventory, error)
}

type executionV1TerminalInventory struct {
	RuntimeID string
	Rows      []port.OrcaTerminal
}

type executionV1TaskInventory struct {
	RuntimeID string
	Rows      []port.OrcaTask
}

type executionV1DispatchInventory struct {
	RuntimeID string
	Dispatch  *port.OrcaDispatch
}

func NewExecutionV1() *ExecutionV1Provisioner {
	return NewExecutionV1Client(New())
}

func NewExecutionV1Client(client executionV1Client) *ExecutionV1Provisioner {
	return &ExecutionV1Provisioner{client: client}
}

func (p *ExecutionV1Provisioner) Probe(ctx context.Context, req port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaProbeResult{Code: "orca_adapter_unavailable"}, nil
	}
	result, err := p.client.Probe(ctx, port.OrcaProbeRequest{Repo: req.Repo, Agent: req.Host, Provider: req.Provider})
	return port.ExecutionOrcaProbeResult{Available: result.Available, Ready: result.Ready, Code: result.Code}, err
}

func (p *ExecutionV1Provisioner) InspectIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionV1IntentRequest(req); err != nil {
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
			if err := validateExecutionV1Worktree(row, req.Workspace, req.Probe); err != nil {
				return port.ExecutionOrcaIntentInventory{}, err
			}
			receipt := executionV1WorkspaceReceipt(req.Workspace, row)
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{Workspace: &receipt})
		}
		return executionV1IntentInventory(candidates), nil
	case port.ExecutionOrcaIntentTerminal:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionV1InventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
		for _, row := range inventory.Rows {
			if strings.TrimSpace(row.Title) != req.Marker && strings.TrimSpace(row.StableTabTitle) != req.Marker {
				continue
			}
			if err := validateExecutionV1IntentTerminal(row, *req.Prepared, req.Marker); err != nil {
				return port.ExecutionOrcaIntentInventory{}, err
			}
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TerminalPTYID: row.PTYID})
		}
		return executionV1IntentInventory(candidates), nil
	case port.ExecutionOrcaIntentTask:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.listAllTasksInventory(ctx)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionV1InventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		title := executionV1TaskTitle(req.Marker, req.Launch.PromptSHA256)
		candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
		for _, row := range inventory.Rows {
			candidateTitle := strings.TrimSpace(row.Title)
			if candidateTitle != title {
				continue
			}
			if err := validateExecutionV1IntentTask(row, *req.Prepared, candidateTitle, req.Workspace.Branch); err != nil {
				return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca owner task candidate does not match the sealed intent")
			}
			candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TaskID: row.ID})
		}
		return executionV1IntentInventory(candidates), nil
	case port.ExecutionOrcaIntentDispatch:
		client, err := p.intentInventoryClient()
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		inventory, err := client.showDispatchInventory(ctx, req.TaskID)
		if err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if err := validateExecutionV1InventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		if inventory.Dispatch == nil {
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
		}
		dispatch := *inventory.Dispatch
		if err := validateExecutionV1ObservedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{TaskID: dispatch.TaskID, DispatchID: dispatch.ID}}}, nil
	default:
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
}

func (p *ExecutionV1Provisioner) InvokeIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentReceipt{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionV1IntentRequest(req); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "execution_intent_invalid", Detail: err.Error()}
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		created, err := p.client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
			Repo: req.Workspace.SourceRoot, Name: req.Workspace.Branch, BaseBranch: req.Workspace.BaseHead,
			Provider: req.Probe.Provider, Issue: req.Probe.Issue, Comment: req.Marker,
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		if err := validateExecutionV1Worktree(created, req.Workspace, req.Probe); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "worktree_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		receipt := executionV1WorkspaceReceipt(req.Workspace, created)
		return port.ExecutionOrcaIntentReceipt{Workspace: &receipt}, nil
	case port.ExecutionOrcaIntentTerminal:
		created, err := p.client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
			WorktreeID: req.Prepared.WorktreeID, Agent: req.Probe.Host, Model: req.Probe.Model, ReasoningEffort: req.Probe.Effort,
			Title: req.Marker, AllowCodexHookTrustBypass: req.Probe.Host == "codex",
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		if err := validateExecutionV1IntentTerminal(created, *req.Prepared, req.Marker); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: created.PTYID}, nil
	case port.ExecutionOrcaIntentTask:
		created, err := p.client.CreateTask(ctx, port.OrcaCreateTaskRequest{
			Spec: req.Launch.Prompt, Title: executionV1TaskTitle(req.Marker, req.Launch.PromptSHA256), DisplayName: req.Workspace.Branch,
		})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		title := executionV1TaskTitle(req.Marker, req.Launch.PromptSHA256)
		if err := validateExecutionV1IntentTask(created, *req.Prepared, title, req.Workspace.Branch); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "task_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: created.ID}, nil
	case port.ExecutionOrcaIntentDispatch:
		terminal, err := p.resolveIntentTerminal(ctx, req)
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "terminal_reconcile_failed", Detail: err.Error()}
		}
		dispatch, err := p.client.Dispatch(ctx, port.OrcaDispatchRequest{TaskID: req.TaskID, ToHandle: terminal.Handle, Inject: true, ReturnPreamble: true})
		if err != nil {
			return port.ExecutionOrcaIntentReceipt{}, err
		}
		if err := validateExecutionV1InvokedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID, terminal.Handle); err != nil {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "dispatch_identity_mismatch", Detail: err.Error(), Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{TaskID: dispatch.TaskID, DispatchID: dispatch.ID}, nil
	default:
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "execution_intent_invalid", Detail: fmt.Sprintf("unsupported stage %q", req.Stage)}
	}
}

func (p *ExecutionV1Provisioner) PrepareWorkspace(ctx context.Context, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionV1Prepare(workspace, req); err != nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, err
	}
	worktree, err := p.prepareWorktree(ctx, workspace, req)
	if err != nil {
		return port.ExecutionOrcaWorkspaceReceipt{}, err
	}
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: filepath.Clean(worktree.Path), Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: worktree.RuntimeID, RepoID: worktree.RepoID, WorktreeID: worktree.ID,
		WorktreeInstanceID: worktree.InstanceID,
	}, nil
}

func (p *ExecutionV1Provisioner) LaunchOwner(ctx context.Context, prepared port.ExecutionOrcaWorkspaceReceipt, req port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) (port.ExecutionOrcaReceipt, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaReceipt{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionV1OwnerLaunch(prepared, req, launch); err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	terminal, err := p.client.CreateTerminal(ctx, port.OrcaCreateTerminalRequest{
		WorktreeID: prepared.WorktreeID, Agent: req.Host, Model: req.Model, ReasoningEffort: req.Effort,
		Title: req.Marker, AllowCodexHookTrustBypass: req.Host == "codex",
	})
	if err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	task, err := p.client.CreateTask(ctx, port.OrcaCreateTaskRequest{
		Spec: launch.Prompt, Title: executionV1TaskTitle(req.Marker, launch.PromptSHA256), DisplayName: prepared.Workspace.Branch,
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
	if err := validateExecutionV1Launch(prepared.WorktreeID, terminal, task, dispatch); err != nil {
		return port.ExecutionOrcaReceipt{}, err
	}
	return port.ExecutionOrcaReceipt{
		Workspace: prepared.Workspace,
		RuntimeID: prepared.RuntimeID, RepoID: prepared.RepoID, WorktreeID: prepared.WorktreeID,
		WorktreeInstanceID: prepared.WorktreeInstanceID, TaskID: task.ID, DispatchID: dispatch.ID, TerminalPTYID: terminal.PTYID,
	}, nil
}

func (p *ExecutionV1Provisioner) InspectOwner(ctx context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	client, ok := p.client.(executionV1InventoryClient)
	if !ok {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner inventory is unavailable")
	}
	terminals, err := client.listTerminalsInventory(ctx, req.WorktreeID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionV1InventoryRuntime(terminals.RuntimeID, req.RuntimeID); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	result := port.ExecutionOrcaOwnerInventory{}
	for _, terminal := range terminals.Rows {
		if terminal.PTYID != req.TerminalPTYID {
			continue
		}
		if result.TerminalID != "" {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		if strings.TrimSpace(req.RuntimeID) == "" || terminal.RuntimeID != req.RuntimeID {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner terminal runtime identity changed")
		}
		result.TerminalID = terminal.PTYID
		result.TerminalLive = terminal.Connected && terminal.Writable
	}
	tasks, err := client.listAllTasksInventory(ctx)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionV1InventoryRuntime(tasks.RuntimeID, req.RuntimeID); err != nil {
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
		if strings.TrimSpace(req.RuntimeID) == "" || task.RuntimeID != req.RuntimeID {
			return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner task runtime identity changed")
		}
		result.TaskStatus = strings.ToLower(strings.TrimSpace(task.Status))
		result.TaskLive = !executionV1TerminalTaskStatus(result.TaskStatus)
	}
	dispatch, err := client.showDispatchInventory(ctx, req.TaskID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionV1InventoryRuntime(dispatch.RuntimeID, req.RuntimeID); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if dispatch.Dispatch == nil {
		if !taskFound {
			return result, nil
		}
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner dispatch is absent")
	}
	row := *dispatch.Dispatch
	if row.RuntimeID != req.RuntimeID || row.ID != req.DispatchID || row.TaskID != req.TaskID {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner dispatch identity changed")
	}
	result.DispatchStatus = strings.ToLower(strings.TrimSpace(row.Status))
	if !executionV1TerminalTaskStatus(result.DispatchStatus) {
		result.TaskLive = true
	}
	return result, nil
}

func executionV1TerminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete", "failed", "cancelled", "canceled", "closed":
		return true
	default:
		return false
	}
}

func (p *ExecutionV1Provisioner) prepareWorktree(ctx context.Context, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) (port.OrcaWorktree, error) {
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
		if err := validateExecutionV1Worktree(candidate, workspace, req); err != nil {
			return port.OrcaWorktree{}, err
		}
		return candidate, nil
	}
	created, err := p.client.CreateWorktree(ctx, port.OrcaCreateWorktreeRequest{
		Repo: workspace.SourceRoot, Name: workspace.Branch, BaseBranch: workspace.BaseHead,
		Provider: req.Provider, Issue: req.Issue, Comment: req.Marker,
	})
	if err != nil {
		return port.OrcaWorktree{}, err
	}
	if err := validateExecutionV1Worktree(created, workspace, req); err != nil {
		return port.OrcaWorktree{}, err
	}
	return created, nil
}

func (p *ExecutionV1Provisioner) intentInventoryClient() (executionV1InventoryClient, error) {
	client, ok := p.client.(executionV1InventoryClient)
	if !ok {
		return nil, fmt.Errorf("Orca execution intent inventory is unavailable")
	}
	return client, nil
}

func (p *ExecutionV1Provisioner) resolveIntentTerminal(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.OrcaTerminal, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	if err := validateExecutionV1InventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
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
	if err := validateExecutionV1IntentTerminal(*candidate, *req.Prepared, req.Marker); err != nil {
		return port.OrcaTerminal{}, err
	}
	return *candidate, nil
}

func executionV1WorkspaceReceipt(workspace port.ExecutionWorkspaceRequest, worktree port.OrcaWorktree) port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: filepath.Clean(worktree.Path), Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: worktree.RuntimeID, RepoID: worktree.RepoID, WorktreeID: worktree.ID, WorktreeInstanceID: worktree.InstanceID,
	}
}

func executionV1IntentInventory(candidates []port.ExecutionOrcaIntentReceipt) port.ExecutionOrcaIntentInventory {
	if candidates == nil {
		candidates = []port.ExecutionOrcaIntentReceipt{}
	}
	return port.ExecutionOrcaIntentInventory{Candidates: candidates, AuthoritativeZero: len(candidates) == 0}
}

func executionV1TaskTitle(marker, promptSHA256 string) string {
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

func validateExecutionV1IntentRequest(req port.ExecutionOrcaIntentRequest) error {
	if req.Marker == "" || req.Marker != req.Probe.Marker {
		return fmt.Errorf("Orca intent marker does not match the sealed operation")
	}
	if err := validateExecutionV1Prepare(req.Workspace, req.Probe); err != nil {
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
		if err := validateExecutionV1OwnerLaunch(*req.Prepared, req.Probe, executionV1RequiredLaunch(req)); err != nil {
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

func executionV1RequiredLaunch(req port.ExecutionOrcaIntentRequest) port.ExecutionOrcaLaunchRequest {
	if req.Launch == nil {
		return port.ExecutionOrcaLaunchRequest{}
	}
	return *req.Launch
}

func validateExecutionV1IntentTerminal(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) error {
	if strings.TrimSpace(terminal.Handle) == "" || strings.TrimSpace(terminal.PTYID) == "" || terminal.WorktreeID != prepared.WorktreeID ||
		strings.TrimSpace(prepared.RuntimeID) == "" || terminal.RuntimeID != prepared.RuntimeID || !terminal.Connected || !terminal.Writable ||
		(strings.TrimSpace(terminal.Title) != marker && strings.TrimSpace(terminal.StableTabTitle) != marker) {
		return fmt.Errorf("Orca owner terminal does not match the sealed intent")
	}
	return nil
}

func validateExecutionV1IntentTask(task port.OrcaTask, prepared port.ExecutionOrcaWorkspaceReceipt, title, displayName string) error {
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || task.RuntimeID != prepared.RuntimeID ||
		strings.TrimSpace(task.Title) != strings.TrimSpace(title) || strings.TrimSpace(task.DisplayName) != strings.TrimSpace(displayName) {
		return fmt.Errorf("Orca owner task does not match the sealed runtime and launch identity")
	}
	return nil
}

func validateExecutionV1InvokedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID, terminalHandle string) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(terminalHandle) == "" || dispatch.AssigneeHandle != terminalHandle || !dispatch.Injected {
		return fmt.Errorf("Orca dispatch does not match the sealed task and terminal")
	}
	return nil
}

func validateExecutionV1ObservedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID string) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(dispatch.AssigneeHandle) == "" || strings.TrimSpace(dispatch.Status) == "" {
		return fmt.Errorf("Orca dispatch does not match the sealed task identity")
	}
	return nil
}

func validateExecutionV1InventoryRuntime(observed, sealed string) error {
	if strings.TrimSpace(observed) == "" || strings.TrimSpace(sealed) == "" || observed != sealed {
		return fmt.Errorf("Orca inventory runtime identity changed")
	}
	return nil
}

func validateExecutionV1Prepare(workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(workspace.LifecycleID) == "" || !filepath.IsAbs(workspace.SourceRoot) || !filepath.IsAbs(workspace.Root) || strings.TrimSpace(workspace.Branch) == "" || strings.TrimSpace(workspace.BaseHead) == "" {
		return fmt.Errorf("Orca prepare requires an exact lifecycle and workspace identity")
	}
	if req.Host != "codex" && req.Host != "claude" || strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Marker) == "" {
		return fmt.Errorf("Orca prepare requires codex or claude with explicit model and marker")
	}
	if req.Provider == "github" && req.Issue <= 0 {
		return fmt.Errorf("Orca GitHub prepare requires a positive issue number")
	}
	return nil
}

func validateExecutionV1Worktree(row port.OrcaWorktree, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(row.RuntimeID) == "" || strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.RepoID) == "" || !samePath(row.Path, workspace.Root) || strings.TrimSpace(row.Branch) != workspace.Branch || strings.TrimSpace(row.Head) != workspace.BaseHead || strings.TrimSpace(row.Comment) != req.Marker {
		return fmt.Errorf("Orca worktree receipt does not match the canonical workspace identity")
	}
	if req.Provider == "github" && row.Issue != req.Issue {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitHub issue")
	}
	if req.Provider == "gitlab" && (row.GitLabIssue == nil || *row.GitLabIssue != req.Issue) {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitLab issue")
	}
	return nil
}

func validateExecutionV1Launch(worktreeID string, terminal port.OrcaTerminal, task port.OrcaTask, dispatch port.OrcaDispatch) error {
	if strings.TrimSpace(terminal.Handle) == "" || terminal.WorktreeID != worktreeID || !terminal.Connected || !terminal.Writable {
		return fmt.Errorf("Orca owner terminal receipt is incomplete")
	}
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(dispatch.ID) == "" || dispatch.TaskID != task.ID || dispatch.AssigneeHandle != terminal.Handle || !dispatch.Injected {
		return fmt.Errorf("Orca task or dispatch receipt is incomplete")
	}
	return nil
}

var executionV1PromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)

func validateExecutionV1OwnerLaunch(prepared port.ExecutionOrcaWorkspaceReceipt, req port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) error {
	if strings.TrimSpace(prepared.WorktreeID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || strings.TrimSpace(prepared.RepoID) == "" {
		return fmt.Errorf("Orca workspace receipt is incomplete")
	}
	if req.Host != "codex" && req.Host != "claude" || strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("Orca owner launch requires an explicit first-party owner profile")
	}
	packet, err := readExecutionV1SealedFile(prepared.Workspace.Root, launch.ContextPacketPath)
	if err != nil {
		return fmt.Errorf("sealed context packet is invalid: %w", err)
	}
	if digestExecutionV1Bytes(packet) != strings.ToLower(strings.TrimSpace(launch.ContextPacketSHA256)) {
		return fmt.Errorf("sealed context packet digest mismatch")
	}
	prompt, err := readExecutionV1SealedFile(prepared.Workspace.Root, launch.PromptPath)
	if err != nil {
		return fmt.Errorf("sealed owner prompt is invalid: %w", err)
	}
	if string(prompt) != launch.Prompt || digestExecutionV1Bytes(prompt) != strings.ToLower(strings.TrimSpace(launch.PromptSHA256)) {
		return fmt.Errorf("sealed owner prompt digest mismatch")
	}
	if executionV1PromptPlaceholder.MatchString(launch.Prompt) || !strings.Contains(launch.Prompt, launch.ContextPacketPath) || !strings.Contains(launch.Prompt, launch.ContextPacketSHA256) {
		return fmt.Errorf("owner prompt is unresolved or does not bind the sealed packet")
	}
	return nil
}

func readExecutionV1SealedFile(root, path string) ([]byte, error) {
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

func digestExecutionV1Bytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

var _ port.ExecutionOrcaProvisioner = (*ExecutionV1Provisioner)(nil)
var _ port.ExecutionOrcaOwnerInspector = (*ExecutionV1Provisioner)(nil)
