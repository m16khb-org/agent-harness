package orca

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/port"
)

const (
	readTimeout   = 10 * time.Second
	createTimeout = 2 * time.Minute
)

type Client struct {
	runner Runner
}

func New() *Client {
	return NewClient(ExecRunner{})
}

func NewClient(runner Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Status(ctx context.Context) (port.OrcaStatus, error) {
	var payload struct {
		Runtime struct {
			State     string `json:"state"`
			Reachable bool   `json:"reachable"`
			RuntimeID string `json:"runtimeId"`
		} `json:"runtime"`
		Graph struct {
			State string `json:"state"`
		} `json:"graph"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "status", "--json"}, &payload)
	if err != nil {
		return port.OrcaStatus{}, err
	}
	if payload.Runtime.RuntimeID != "" {
		runtimeID = payload.Runtime.RuntimeID
	}
	return port.OrcaStatus{RuntimeID: runtimeID, RuntimeReachable: payload.Runtime.Reachable, RuntimeState: payload.Runtime.State, GraphState: payload.Graph.State}, nil
}

func (c *Client) Probe(ctx context.Context, req port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	agent := strings.TrimSpace(req.Agent)
	if agent == "" {
		agent = "codex"
	}
	command, ok := hostCommand(agent)
	if !ok {
		return port.OrcaProbeResult{Code: "unsupported_agent", Agent: agent}, nil
	}
	if _, err := c.runner.LookPath("orca"); err != nil {
		return port.OrcaProbeResult{Code: "orca_not_found", Agent: agent}, nil
	}
	result := port.OrcaProbeResult{Available: true, Agent: agent}
	status, err := c.Status(ctx)
	if err != nil {
		result.Code = "status_failed"
		result.Detail = boundedDiagnostic(err.Error())
		return result, nil
	}
	result.RuntimeID = status.RuntimeID
	if strings.TrimSpace(status.RuntimeID) == "" {
		result.Code = "runtime_id_unresolved"
		return result, nil
	}
	if !status.RuntimeReachable {
		result.Code = "runtime_unreachable"
		return result, nil
	}
	if status.RuntimeState != "ready" {
		result.Code = "runtime_not_ready"
		return result, nil
	}
	if status.GraphState != "ready" {
		result.Code = "graph_not_ready"
		return result, nil
	}
	repo, err := c.showRepo(ctx, req.Repo)
	if err != nil {
		result.Code = "repo_unresolved"
		result.Detail = boundedDiagnostic(err.Error())
		return result, nil
	}
	result.RepoID = repo.ID
	result.RepoPath = repo.Path
	result.RepoRemoteName = repo.RemoteName
	result.WorktreeBasePath = repo.WorktreeBasePath
	if strings.TrimSpace(repo.ID) == "" || strings.TrimSpace(repo.Path) == "" {
		result.Code = "repo_identity_unresolved"
		return result, nil
	}
	if !samePath(repo.Path, req.Repo) {
		result.Code = "repo_path_mismatch"
		return result, nil
	}
	if strings.TrimSpace(repo.RemoteName) == "" {
		result.Code = "repo_remote_unresolved"
		return result, nil
	}
	if strings.TrimSpace(repo.WorktreeBasePath) == "" {
		result.Code = "worktree_base_unresolved"
		return result, nil
	}
	basePath := repo.WorktreeBasePath
	if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(repo.Path, basePath)
	}
	if !samePath(basePath, filepath.Clean(req.Repo)+".worktrees") {
		result.Code = "worktree_base_mismatch"
		return result, nil
	}
	for _, capability := range []struct {
		argv []string
		want []string
	}{
		{argv: []string{"orca", "worktree", "create", "--help"}, want: []string{"--repo", "--name", "--base-branch", "--no-parent", "--setup", "--comment", "--issue", "--json"}},
		{argv: []string{"orca", "worktree", "list", "--help"}, want: []string{"--repo", "--limit", "--json"}},
		{argv: []string{"orca", "terminal", "create", "--help"}, want: []string{"--worktree", "--command", "--title", "--json"}},
		{argv: []string{"orca", "terminal", "list", "--help"}, want: []string{"--worktree", "--limit", "--json"}},
		{argv: []string{"orca", "orchestration", "task-create", "--help"}, want: []string{"--spec", "--task-title", "--display-name", "--json"}},
		{argv: []string{"orca", "orchestration", "task-list", "--help"}, want: []string{"--ready", "--json"}},
		{argv: []string{"orca", "orchestration", "task-update", "--help"}, want: []string{"--id", "--status", "--result", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch", "--help"}, want: []string{"--task", "--to", "--from", "--inject", "--return-preamble", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch-show", "--help"}, want: []string{"--task", "--json"}},
		{argv: []string{"orca", "worktree", "rm", "--help"}, want: []string{"--worktree", "--force", "--json"}},
	} {
		text, err := c.runText(ctx, "", readTimeout, capability.argv)
		if err != nil || !containsAllHelpFlags(text, capability.want) {
			result.Code = "capability_missing"
			return result, nil
		}
	}
	if _, err := c.runner.LookPath(command); err != nil {
		result.Code = "agent_not_found"
		return result, nil
	}
	if agent == "codex" {
		help, err := c.runText(ctx, "", readTimeout, []string{"codex", "--help"})
		if err != nil || !containsAllHelpFlags(help, []string{"--dangerously-bypass-hook-trust"}) {
			result.Code = "codex_hook_trust_bypass_unsupported"
			return result, nil
		}
	}
	if _, err := c.ListTasks(ctx); err != nil {
		result.Code = "orchestration_unready"
		result.Detail = boundedDiagnostic(err.Error())
		return result, nil
	}
	result.Ready = true
	return result, nil
}

func (c *Client) showRepo(ctx context.Context, repo string) (port.OrcaRepo, error) {
	var payload struct {
		Repo struct {
			ID                string `json:"id"`
			Path              string `json:"path"`
			DisplayName       string `json:"displayName"`
			WorktreeBasePath  string `json:"worktreeBasePath"`
			GitRemoteIdentity struct {
				RemoteName string `json:"remoteName"`
			} `json:"gitRemoteIdentity"`
		} `json:"repo"`
	}
	_, err := c.runJSON(ctx, repo, readTimeout, []string{"orca", "repo", "show", "--repo", pathSelector(repo), "--json"}, &payload)
	return port.OrcaRepo{ID: payload.Repo.ID, Path: payload.Repo.Path, Name: payload.Repo.DisplayName, RemoteName: payload.Repo.GitRemoteIdentity.RemoteName, WorktreeBasePath: payload.Repo.WorktreeBasePath}, err
}

func (c *Client) ListWorktrees(ctx context.Context, repo string) ([]port.OrcaWorktree, error) {
	var payload struct {
		Worktrees  []worktreePayload `json:"worktrees"`
		TotalCount *int              `json:"totalCount"`
		Truncated  bool              `json:"truncated"`
	}
	_, err := c.runJSON(ctx, repo, readTimeout, []string{"orca", "worktree", "list", "--repo", pathSelector(repo), "--limit", strconv.Itoa(port.OrcaMaxBaselineIDs), "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireCompleteList("worktree", len(payload.Worktrees), payload.TotalCount, payload.Truncated); err != nil {
		return nil, err
	}
	result := make([]port.OrcaWorktree, 0, len(payload.Worktrees))
	for _, item := range payload.Worktrees {
		result = append(result, item.portValue())
	}
	return result, nil
}

func (c *Client) CreateWorktree(ctx context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	argv := []string{"orca", "worktree", "create", "--repo", pathSelector(req.Repo), "--name", strings.TrimSpace(req.Name), "--base-branch", strings.TrimSpace(req.BaseBranch), "--no-parent", "--setup", "skip", "--comment", strings.TrimSpace(req.Comment), "--json"}
	if req.Issue > 0 {
		argv = append(argv[:len(argv)-1], "--issue", strconv.Itoa(req.Issue), "--json")
	}
	var payload struct {
		Worktree worktreePayload `json:"worktree"`
	}
	_, err := c.runJSON(ctx, req.Repo, createTimeout, argv, &payload)
	return payload.Worktree.portValue(), err
}

func (c *Client) RemoveWorktree(ctx context.Context, id string, force bool) error {
	argv := []string{"orca", "worktree", "rm", "--worktree", idSelector(id)}
	if force {
		argv = append(argv, "--force")
	}
	argv = append(argv, "--json")
	_, err := c.runJSON(ctx, "", createTimeout, argv, &struct{}{})
	return err
}

func (c *Client) ListTerminals(ctx context.Context, worktreeID string) ([]port.OrcaTerminal, error) {
	var payload struct {
		Terminals  []terminalPayload `json:"terminals"`
		TotalCount *int              `json:"totalCount"`
		Truncated  bool              `json:"truncated"`
	}
	_, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "terminal", "list", "--worktree", idSelector(worktreeID), "--limit", strconv.Itoa(port.OrcaMaxBaselineIDs), "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireCompleteList("terminal", len(payload.Terminals), payload.TotalCount, payload.Truncated); err != nil {
		return nil, err
	}
	result := make([]port.OrcaTerminal, 0, len(payload.Terminals))
	for _, item := range payload.Terminals {
		result = append(result, item.portValue())
	}
	return result, nil
}

func (c *Client) CreateTerminal(ctx context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	command, ok := hostCommand(req.Agent)
	if !ok {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "unsupported_agent", Detail: req.Agent}
	}
	if strings.EqualFold(strings.TrimSpace(req.Agent), "codex") && req.AllowCodexHookTrustBypass {
		command = "codex --dangerously-bypass-hook-trust"
	}
	argv := []string{"orca", "terminal", "create", "--worktree", idSelector(req.WorktreeID), "--command", command}
	if title := strings.TrimSpace(req.Title); title != "" {
		argv = append(argv, "--title", title)
	}
	argv = append(argv, "--json")
	var payload struct {
		Terminal terminalPayload `json:"terminal"`
	}
	_, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	created := payload.Terminal.portValue()
	if strings.TrimSpace(created.Handle) == "" || strings.TrimSpace(created.WorktreeID) != strings.TrimSpace(req.WorktreeID) {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: "terminal identity returned by create is incomplete", Invoked: true}
	}
	return created, nil
}

func (c *Client) RefreshTerminal(ctx context.Context, worktreeID, ptyID string) (port.OrcaTerminal, error) {
	terminals, err := c.ListTerminals(ctx, worktreeID)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	for _, terminal := range terminals {
		if terminal.WorktreeID == strings.TrimSpace(worktreeID) && terminal.PTYID == strings.TrimSpace(ptyID) {
			return terminal, nil
		}
	}
	return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_not_found"}
}

func (c *Client) ListTasks(ctx context.Context) ([]port.OrcaTask, error) {
	var payload struct {
		Tasks []taskPayload `json:"tasks"`
		Count *int          `json:"count"`
	}
	_, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "task-list", "--ready", "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if payload.Count == nil || *payload.Count != len(payload.Tasks) {
		count := -1
		if payload.Count != nil {
			count = *payload.Count
		}
		return nil, fmt.Errorf("Orca task list is incomplete: count=%d returned=%d", count, len(payload.Tasks))
	}
	result := make([]port.OrcaTask, 0, len(payload.Tasks))
	for _, task := range payload.Tasks {
		result = append(result, task.portValue())
	}
	return result, nil
}

func (c *Client) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	argv := []string{"orca", "orchestration", "task-create", "--spec", req.Spec, "--task-title", req.Title, "--display-name", req.DisplayName, "--json"}
	var payload struct {
		Task taskPayload `json:"task"`
	}
	_, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	return payload.Task.portValue(), err
}

func (c *Client) UpdateTask(ctx context.Context, id, status, result string) error {
	argv := []string{"orca", "orchestration", "task-update", "--id", id, "--status", status}
	if result != "" {
		argv = append(argv, "--result", result)
	}
	argv = append(argv, "--json")
	_, err := c.runJSON(ctx, "", readTimeout, argv, &struct{}{})
	return err
}

func (c *Client) Dispatch(ctx context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	argv := []string{"orca", "orchestration", "dispatch", "--task", req.TaskID, "--to", req.ToHandle}
	if req.FromHandle != "" {
		argv = append(argv, "--from", req.FromHandle)
	}
	if req.Inject {
		argv = append(argv, "--inject")
	}
	if req.ReturnPreamble {
		argv = append(argv, "--return-preamble")
	}
	argv = append(argv, "--json")
	return c.dispatchResult(ctx, argv)
}

func (c *Client) ShowDispatch(ctx context.Context, taskID string) (port.OrcaDispatch, error) {
	return c.dispatchResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--json"})
}

func (c *Client) dispatchResult(ctx context.Context, argv []string) (port.OrcaDispatch, error) {
	var payload struct {
		Dispatch struct {
			ID             string `json:"id"`
			TaskID         string `json:"task_id"`
			AssigneeHandle string `json:"assignee_handle"`
			Status         string `json:"status"`
		} `json:"dispatch"`
		Injected bool   `json:"injected"`
		Preamble string `json:"preamble"`
	}
	_, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	return port.OrcaDispatch{ID: payload.Dispatch.ID, TaskID: payload.Dispatch.TaskID, AssigneeHandle: payload.Dispatch.AssigneeHandle, Status: payload.Dispatch.Status, Injected: payload.Injected, Preamble: payload.Preamble}, err
}

func (c *Client) runJSON(ctx context.Context, cwd string, timeout time.Duration, argv []string, target any) (string, error) {
	output, err := c.runner.Run(ctx, cwd, timeout, argv)
	if err != nil {
		return "", err
	}
	return decodeResult(output, target)
}

func (c *Client) runText(ctx context.Context, cwd string, timeout time.Duration, argv []string) (string, error) {
	output, err := c.runner.Run(ctx, cwd, timeout, argv)
	if err != nil {
		return "", err
	}
	if len(output.Stdout) > MaxEnvelopeBytes {
		return "", fmt.Errorf("Orca output exceeds %d bytes", MaxEnvelopeBytes)
	}
	return string(output.Stdout), nil
}

type worktreePayload struct {
	ID          string `json:"id"`
	InstanceID  string `json:"instanceId"`
	RepoID      string `json:"repoId"`
	Path        string `json:"path"`
	Head        string `json:"head"`
	Branch      string `json:"branch"`
	DisplayName string `json:"displayName"`
	Comment     string `json:"comment"`
	BaseRef     string `json:"baseRef"`
	LinkedIssue int    `json:"linkedIssue"`
}

func (w worktreePayload) portValue() port.OrcaWorktree {
	return port.OrcaWorktree{ID: w.ID, InstanceID: w.InstanceID, RepoID: w.RepoID, Path: w.Path, Head: w.Head, Branch: w.Branch, Name: w.DisplayName, Comment: w.Comment, BaseRef: w.BaseRef, Issue: w.LinkedIssue}
}

type terminalPayload struct {
	Handle       string `json:"handle"`
	PTYID        string `json:"ptyId"`
	WorktreeID   string `json:"worktreeId"`
	WorktreePath string `json:"worktreePath"`
	Connected    bool   `json:"connected"`
	Writable     bool   `json:"writable"`
}

type taskPayload struct {
	ID          string `json:"id"`
	TaskTitle   string `json:"task_title"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}

func (t taskPayload) portValue() port.OrcaTask {
	return port.OrcaTask{ID: t.ID, Title: t.TaskTitle, DisplayName: t.DisplayName, Status: t.Status}
}

func requireCompleteList(kind string, length int, total *int, truncated bool) error {
	if total == nil {
		return &port.OrcaError{Code: "incomplete_list", Detail: fmt.Sprintf("Orca %s list completeness metadata is missing", kind), Invoked: true}
	}
	if truncated || *total != length {
		return &port.OrcaError{Code: "incomplete_list", Detail: fmt.Sprintf("Orca %s list is incomplete: totalCount=%d returned=%d truncated=%t", kind, *total, length, truncated), Invoked: true}
	}
	return nil
}

func (t terminalPayload) portValue() port.OrcaTerminal {
	return port.OrcaTerminal{Handle: t.Handle, PTYID: t.PTYID, WorktreeID: t.WorktreeID, WorktreePath: t.WorktreePath, Connected: t.Connected, Writable: t.Writable}
}

func hostCommand(agent string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(agent)) {
	case "codex":
		return "codex", true
	case "claude":
		return "claude", true
	case "gjc":
		return "gjc", true
	default:
		return "", false
	}
}

func pathSelector(path string) string { return "path:" + strings.TrimSpace(path) }
func idSelector(id string) string     { return "id:" + strings.TrimSpace(id) }

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(strings.TrimSpace(left))
	rightAbs, rightErr := filepath.Abs(strings.TrimSpace(right))
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func containsAllHelpFlags(value string, items []string) bool {
	present := map[string]struct{}{}
	for _, field := range strings.Fields(value) {
		field = strings.Trim(field, "[](),;:")
		if index := strings.IndexByte(field, '='); index >= 0 {
			field = field[:index]
		}
		if strings.HasPrefix(field, "--") {
			present[field] = struct{}{}
		}
	}
	for _, item := range items {
		if _, ok := present[item]; !ok {
			return false
		}
	}
	return true
}

var _ port.OrcaClient = (*Client)(nil)
