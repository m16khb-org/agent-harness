package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/port"
)

const (
	readTimeout   = 10 * time.Second
	createTimeout = 2 * time.Minute
)

var concreteTerminalHandlePattern = regexp.MustCompile(`^term_[A-Za-z0-9_-]+$`)

type Client struct {
	runner Runner
}

func New() *Client {
	return NewClient(ExecRunner{})
}

func NewClient(runner Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) Available() bool {
	_, err := c.runner.LookPath("orca")
	return err == nil
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
	provider, providerOK := orcaIssueProvider(req.Provider)
	if !providerOK {
		return port.OrcaProbeResult{Code: "unsupported_provider", Agent: agent, Provider: strings.ToLower(strings.TrimSpace(req.Provider))}, nil
	}
	command, ok := hostCommand(agent)
	if !ok {
		return port.OrcaProbeResult{Code: "unsupported_agent", Agent: agent, Provider: provider}, nil
	}
	if _, err := c.runner.LookPath("orca"); err != nil {
		return port.OrcaProbeResult{Code: "orca_not_found", Agent: agent, Provider: provider}, nil
	}
	result := port.OrcaProbeResult{Available: true, Agent: agent, Provider: provider}
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
	worktreeCreateFlags := []string{"--repo", "--name", "--base-branch", "--no-parent", "--setup", "--comment", "--json"}
	if provider == "github" {
		worktreeCreateFlags = append(worktreeCreateFlags, "--issue")
	}
	for _, capability := range []struct {
		argv    []string
		want    []string
		wantAny [][]string
	}{
		{argv: []string{"orca", "worktree", "create", "--help"}, want: worktreeCreateFlags},
		{argv: []string{"orca", "worktree", "list", "--help"}, want: []string{"--repo", "--limit", "--json"}},
		{argv: []string{"orca", "terminal", "create", "--help"}, wantAny: [][]string{{"--worktree", "--agent", "--title", "--json"}, {"--worktree", "--command", "--title", "--json"}}},
		{argv: []string{"orca", "terminal", "list", "--help"}, want: []string{"--worktree", "--limit", "--json"}},
		{argv: []string{"orca", "orchestration", "task-create", "--help"}, want: []string{"--spec", "--task-title", "--display-name", "--json"}},
		{argv: []string{"orca", "orchestration", "task-list", "--help"}, want: []string{"--ready", "--status", "--json"}},
		{argv: []string{"orca", "orchestration", "task-update", "--help"}, want: []string{"--id", "--status", "--result", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch", "--help"}, want: []string{"--task", "--to", "--from", "--inject", "--return-preamble", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch-show", "--help"}, want: []string{"--task", "--preamble", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "send", "--help"}, want: []string{"--to", "--from", "--type", "--subject", "--body", "--task-id", "--dispatch-id", "--files-modified", "--report-path", "--json"}},
		{argv: []string{"orca", "worktree", "rm", "--help"}, want: []string{"--worktree", "--force", "--json"}},
	} {
		text, err := c.runText(ctx, "", readTimeout, capability.argv)
		matched := len(capability.want) > 0 && containsAllHelpFlags(text, capability.want)
		for _, alternative := range capability.wantAny {
			matched = matched || containsAllHelpFlags(text, alternative)
		}
		if err != nil || !matched {
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
	runtimeID, err := c.runJSON(ctx, repo, readTimeout, []string{"orca", "repo", "show", "--repo", pathSelector(repo), "--json"}, &payload)
	return port.OrcaRepo{RuntimeID: runtimeID, ID: payload.Repo.ID, Path: payload.Repo.Path, Name: payload.Repo.DisplayName, RemoteName: payload.Repo.GitRemoteIdentity.RemoteName, WorktreeBasePath: payload.Repo.WorktreeBasePath}, err
}

func (c *Client) ResolveRepo(ctx context.Context, repo string) (port.OrcaRepo, error) {
	repo = strings.TrimSpace(repo)
	if !filepath.IsAbs(repo) {
		return port.OrcaRepo{}, &port.OrcaError{Code: "repo_identity_invalid", Detail: "absolute repo path is required"}
	}
	resolved, err := c.showRepo(ctx, repo)
	if err != nil {
		return port.OrcaRepo{}, err
	}
	if strings.TrimSpace(resolved.ID) == "" || strings.TrimSpace(resolved.Path) == "" || !samePath(resolved.Path, repo) {
		return port.OrcaRepo{}, &port.OrcaError{Code: "repo_identity_mismatch", Detail: "resolved repo identity does not match the requested path", Invoked: true}
	}
	return resolved, nil
}

func (c *Client) ListWorktrees(ctx context.Context, repo string) ([]port.OrcaWorktree, error) {
	var payload struct {
		Worktrees  []worktreePayload `json:"worktrees"`
		TotalCount *int              `json:"totalCount"`
		Truncated  bool              `json:"truncated"`
	}
	runtimeID, err := c.runJSON(ctx, repo, readTimeout, []string{"orca", "worktree", "list", "--repo", pathSelector(repo), "--limit", strconv.Itoa(port.OrcaMaxBaselineIDs), "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireCompleteList("worktree", len(payload.Worktrees), payload.TotalCount, payload.Truncated); err != nil {
		return nil, err
	}
	result := make([]port.OrcaWorktree, 0, len(payload.Worktrees))
	for _, item := range payload.Worktrees {
		value := item.portValue()
		value.RuntimeID = runtimeID
		result = append(result, value)
	}
	return result, nil
}

func (c *Client) ShowWorktree(ctx context.Context, path string) (port.OrcaWorktree, error) {
	var payload struct {
		Worktree worktreePayload `json:"worktree"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "worktree", "show", "--worktree", pathSelector(path), "--json"}, &payload)
	shown := payload.Worktree.portValue()
	shown.RuntimeID = runtimeID
	return shown, err
}

func (c *Client) CreateWorktree(ctx context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	provider, ok := orcaIssueProvider(req.Provider)
	if !ok {
		return port.OrcaWorktree{}, &port.OrcaError{Code: "unsupported_provider", Detail: strings.ToLower(strings.TrimSpace(req.Provider))}
	}
	argv := []string{"orca", "worktree", "create", "--repo", pathSelector(req.Repo), "--name", strings.TrimSpace(req.Name), "--base-branch", strings.TrimSpace(req.BaseBranch), "--no-parent", "--setup", "skip", "--comment", strings.TrimSpace(req.Comment), "--json"}
	if provider == "github" {
		if req.Issue <= 0 {
			return port.OrcaWorktree{}, &port.OrcaError{Code: "github_issue_required", Detail: "a positive linked GitHub issue number is required"}
		}
		argv = append(argv[:len(argv)-1], "--issue", strconv.Itoa(req.Issue), "--json")
	}
	var payload struct {
		Worktree worktreePayload `json:"worktree"`
	}
	runtimeID, err := c.runJSON(ctx, req.Repo, createTimeout, argv, &payload)
	created := payload.Worktree.portValue()
	created.RuntimeID = runtimeID
	return created, err
}

func (c *Client) AdoptWorktree(ctx context.Context, req port.OrcaAdoptWorktreeRequest) (port.OrcaWorktree, error) {
	provider, ok := orcaIssueProvider(req.Provider)
	if !ok {
		return port.OrcaWorktree{}, &port.OrcaError{Code: "unsupported_provider", Detail: strings.ToLower(strings.TrimSpace(req.Provider))}
	}
	if strings.TrimSpace(req.WorktreeID) == "" || strings.TrimSpace(req.Comment) == "" {
		return port.OrcaWorktree{}, &port.OrcaError{Code: "worktree_adopt_invalid", Detail: "worktree id and comment are required"}
	}
	argv := []string{"orca", "worktree", "set", "--worktree", idSelector(req.WorktreeID), "--comment", strings.TrimSpace(req.Comment)}
	if provider == "github" {
		if req.Issue <= 0 {
			return port.OrcaWorktree{}, &port.OrcaError{Code: "github_issue_required", Detail: "a positive linked GitHub issue number is required"}
		}
		argv = append(argv, "--issue", strconv.Itoa(req.Issue))
	}
	argv = append(argv, "--json")
	var payload struct {
		Worktree worktreePayload `json:"worktree"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	adopted := payload.Worktree.portValue()
	adopted.RuntimeID = runtimeID
	return adopted, err
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
		Terminals     []terminalPayload     `json:"terminals"`
		VisualLayouts []visualLayoutPayload `json:"visualLayouts"`
		TotalCount    *int                  `json:"totalCount"`
		Truncated     bool                  `json:"truncated"`
	}
	argv := []string{"orca", "terminal", "list"}
	if strings.TrimSpace(worktreeID) != "" {
		argv = append(argv, "--worktree", idSelector(worktreeID))
	}
	argv = append(argv, "--limit", strconv.Itoa(port.OrcaMaxBaselineIDs), "--json")
	runtimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireCompleteList("terminal", len(payload.Terminals), payload.TotalCount, payload.Truncated); err != nil {
		return nil, err
	}
	stableTitles, err := stableVisualTabTitles(payload.VisualLayouts)
	if err != nil {
		return nil, err
	}
	result := make([]port.OrcaTerminal, 0, len(payload.Terminals))
	for _, item := range payload.Terminals {
		value := item.portValue()
		value.RuntimeID = runtimeID
		value.StableTabTitle = stableTitles[visualTabKey(value.TabID, value.LeafID)]
		result = append(result, value)
	}
	return result, nil
}

func (c *Client) CreateTerminal(ctx context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	command, ok := hostCommand(req.Agent)
	if !ok {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "unsupported_agent", Detail: req.Agent}
	}
	model, effort, err := port.NormalizeCodexLaunchOptions(req.CodexModel, req.CodexReasoningEffort)
	if err != nil || (!strings.EqualFold(strings.TrimSpace(req.Agent), "codex") && (model != "" || effort != "")) {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "codex_launch_options_invalid", Detail: boundedDiagnostic(fmt.Sprint(err))}
	}
	pinnedCodex := model != ""
	if pinnedCodex {
		command += " -m " + model + " -c model_reasoning_effort=" + effort
	}
	help, err := c.runText(ctx, "", readTimeout, []string{"orca", "terminal", "create", "--help"})
	if err != nil {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_create_capability_unavailable", Detail: boundedDiagnostic(err.Error())}
	}
	argv := []string{"orca", "terminal", "create", "--worktree", idSelector(req.WorktreeID)}
	switch {
	case !pinnedCodex && containsAllHelpFlags(help, []string{"--worktree", "--agent", "--title", "--json"}):
		argv = append(argv, "--agent", command)
	case containsAllHelpFlags(help, []string{"--worktree", "--command", "--title", "--json"}):
		if strings.EqualFold(strings.TrimSpace(req.Agent), "codex") && req.AllowCodexHookTrustBypass {
			command += " --dangerously-bypass-hook-trust"
		}
		argv = append(argv, "--command", command)
	default:
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_create_capability_missing", Detail: "installed Orca exposes neither the fixed --agent nor fixed --command launch shape"}
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		argv = append(argv, "--title", title)
	}
	argv = append(argv, "--json")
	var payload struct {
		Terminal terminalPayload `json:"terminal"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	created := payload.Terminal.portValue()
	created.RuntimeID = runtimeID
	if strings.TrimSpace(created.Handle) == "" || strings.TrimSpace(created.WorktreeID) != strings.TrimSpace(req.WorktreeID) {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_identity_mismatch", Detail: "terminal identity returned by create is incomplete", Invoked: true}
	}
	return created, nil
}

// BootstrapTerminalAgent turns an exact, already-owned legacy terminal into
// an Orca-recognized agent target before inject dispatch. The worker terminal
// is selected and sole-writer-attested by IssueOps; this adapter only emits a
// fixed host command and waits for Orca to settle its TUI state.
func (c *Client) BootstrapTerminalAgent(ctx context.Context, req port.OrcaBootstrapTerminalAgentRequest) error {
	if strings.TrimSpace(req.TerminalHandle) == "" {
		return &port.OrcaError{Code: "terminal_agent_bootstrap_invalid", Detail: "terminal handle is required"}
	}
	command, ok := hostCommand(req.Agent)
	if !ok {
		return &port.OrcaError{Code: "unsupported_agent", Detail: req.Agent}
	}
	model, effort, err := port.NormalizeCodexLaunchOptions(req.CodexModel, req.CodexReasoningEffort)
	if err != nil || (!strings.EqualFold(strings.TrimSpace(req.Agent), "codex") && (model != "" || effort != "")) {
		return &port.OrcaError{Code: "codex_launch_options_invalid", Detail: boundedDiagnostic(fmt.Sprint(err))}
	}
	if model != "" {
		command += " -m " + model + " -c model_reasoning_effort=" + effort
	}
	if strings.EqualFold(strings.TrimSpace(req.Agent), "codex") && req.AllowCodexHookTrustBypass {
		command += " --dangerously-bypass-hook-trust"
	}
	var send struct {
		Send struct {
			Accepted bool `json:"accepted"`
		} `json:"send"`
	}
	if _, err := c.runJSON(ctx, "", createTimeout, []string{"orca", "terminal", "send", "--terminal", strings.TrimSpace(req.TerminalHandle), "--text", command, "--enter", "--json"}, &send); err != nil {
		return err
	}
	if !send.Send.Accepted {
		return &port.OrcaError{Code: "terminal_agent_bootstrap_rejected", Detail: "Orca did not accept the exact terminal bootstrap", Invoked: true}
	}
	var wait struct {
		Wait struct {
			Satisfied bool `json:"satisfied"`
		} `json:"wait"`
	}
	if _, err := c.runJSON(ctx, "", createTimeout, []string{"orca", "terminal", "wait", "--terminal", strings.TrimSpace(req.TerminalHandle), "--for", "tui-idle", "--timeout-ms", "10000", "--json"}, &wait); err != nil {
		return err
	}
	if !wait.Wait.Satisfied {
		return &port.OrcaError{Code: "terminal_agent_bootstrap_timeout", Detail: "agent terminal did not reach Orca TUI idle state", Invoked: true}
	}
	return nil
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
	return c.listTasks(ctx, []string{"orca", "orchestration", "task-list", "--ready", "--json"})
}

func (c *Client) ListDispatchedTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasks(ctx, []string{"orca", "orchestration", "task-list", "--status", "dispatched", "--json"})
}

func (c *Client) ListAllTasks(ctx context.Context) ([]port.OrcaTask, error) {
	return c.listTasks(ctx, []string{"orca", "orchestration", "task-list", "--brief", "--json"})
}

func (c *Client) listTasks(ctx context.Context, argv []string) ([]port.OrcaTask, error) {
	var payload struct {
		Tasks []taskPayload `json:"tasks"`
		Count *int          `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireReturnedCount("task", len(payload.Tasks), payload.Count); err != nil {
		return nil, err
	}
	result := make([]port.OrcaTask, 0, len(payload.Tasks))
	for _, task := range payload.Tasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Status) == "" {
			return nil, fmt.Errorf("Orca task row identity is incomplete")
		}
		value := task.portValue()
		value.RuntimeID = runtimeID
		result = append(result, value)
	}
	return result, nil
}

func (c *Client) ListGates(ctx context.Context) ([]port.OrcaGate, error) {
	var payload struct {
		Gates []struct {
			ID     string `json:"id"`
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		} `json:"gates"`
		Count *int `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "gate-list", "--json"}, &payload)
	if err != nil {
		return nil, err
	}
	if err := requireReturnedCount("gate", len(payload.Gates), payload.Count); err != nil {
		return nil, err
	}
	result := make([]port.OrcaGate, 0, len(payload.Gates))
	for _, gate := range payload.Gates {
		if strings.TrimSpace(gate.ID) == "" || strings.TrimSpace(gate.TaskID) == "" || strings.TrimSpace(gate.Status) == "" {
			return nil, fmt.Errorf("Orca gate row identity is incomplete")
		}
		result = append(result, port.OrcaGate{RuntimeID: runtimeID, ID: gate.ID, TaskID: gate.TaskID, Status: gate.Status})
	}
	return result, nil
}

func (c *Client) InboxPresence(ctx context.Context) (port.OrcaInboxPresence, error) {
	var payload struct {
		Messages []struct{} `json:"messages"`
		Count    *int       `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "inbox", "--limit", "1", "--json"}, &payload)
	if err != nil {
		return port.OrcaInboxPresence{}, err
	}
	if payload.Count == nil {
		return port.OrcaInboxPresence{}, fmt.Errorf("Orca inbox completeness metadata is missing")
	}
	count := *payload.Count
	rows := len(payload.Messages)
	return port.OrcaInboxPresence{RuntimeID: runtimeID, Count: count, RowCount: rows, CompleteAbsence: count == 0 && rows == 0}, nil
}

func (c *Client) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	argv := []string{"orca", "orchestration", "task-create", "--spec", req.Spec, "--task-title", req.Title, "--display-name", req.DisplayName, "--json"}
	var payload struct {
		Task taskPayload `json:"task"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	created := payload.Task.portValue()
	created.RuntimeID = runtimeID
	return created, err
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

func (c *Client) ShowDispatchFrom(ctx context.Context, taskID, fromHandle string) (port.OrcaDispatch, error) {
	return c.dispatchResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--preamble", "--from", fromHandle, "--json"})
}

func (c *Client) SendWorkerDone(ctx context.Context, req port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error) {
	if err := validateWorkerDoneRequest(req); err != nil {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_invalid", Detail: err.Error()}
	}
	argv := []string{
		"orca", "orchestration", "send",
		"--to", req.ToHandle,
		"--from", req.FromHandle,
		"--type", "worker_done",
		"--subject", req.Subject,
		"--body", req.Body,
		"--task-id", req.TaskID,
		"--dispatch-id", req.DispatchID,
	}
	if len(req.ChangedFiles) > 0 {
		argv = append(argv, "--files-modified", strings.Join(req.ChangedFiles, ","))
	}
	argv = append(argv, "--report-path", req.ReportPath, "--json")
	output, err := c.runner.Run(ctx, "", createTimeout, argv)
	if err != nil {
		return port.OrcaWorkerDoneResult{}, err
	}
	var payload struct {
		Message struct {
			ID         string `json:"id"`
			FromHandle string `json:"from_handle"`
			ToHandle   string `json:"to_handle"`
			Type       string `json:"type"`
			Subject    string `json:"subject"`
			Body       string `json:"body"`
			Payload    string `json:"payload"`
			Sequence   int64  `json:"sequence"`
		} `json:"message"`
	}
	if _, err := decodeResult(output, &payload); err != nil {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_malformed", Detail: boundedDiagnostic(err.Error()), Invoked: output.Invoked}
	}
	message := payload.Message
	if message.ID == "" || len(message.ID) > 1024 || message.Sequence <= 0 || message.FromHandle != req.FromHandle || message.ToHandle != req.ToHandle || message.Type != "worker_done" || message.Subject != req.Subject || message.Body != req.Body {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_mismatch", Detail: "Orca message identity or evidence does not match the requested projection", Invoked: true}
	}
	var evidence struct {
		TaskID        string   `json:"taskId"`
		DispatchID    string   `json:"dispatchId"`
		FilesModified []string `json:"filesModified"`
		ReportPath    string   `json:"reportPath"`
	}
	if len(message.Payload) > 64*1024 || json.Unmarshal([]byte(message.Payload), &evidence) != nil || evidence.TaskID != req.TaskID || evidence.DispatchID != req.DispatchID || !slices.Equal(evidence.FilesModified, req.ChangedFiles) || evidence.ReportPath != req.ReportPath {
		return port.OrcaWorkerDoneResult{}, &port.OrcaError{Code: "worker_done_response_mismatch", Detail: "Orca message payload does not match the requested projection", Invoked: true}
	}
	return port.OrcaWorkerDoneResult{MessageID: message.ID, Sequence: message.Sequence}, nil
}

func validateWorkerDoneRequest(req port.OrcaWorkerDoneRequest) error {
	if !concreteTerminalHandlePattern.MatchString(req.FromHandle) || !concreteTerminalHandlePattern.MatchString(req.ToHandle) || req.FromHandle == req.ToHandle || len(req.FromHandle) > 256 || len(req.ToHandle) > 256 {
		return fmt.Errorf("worker_done requires distinct concrete bounded Orca terminal handles")
	}
	for name, value := range map[string]struct {
		value string
		limit int
	}{
		"subject": {req.Subject, 256}, "body": {req.Body, 4096}, "task id": {req.TaskID, 1024}, "dispatch id": {req.DispatchID, 1024},
	} {
		if strings.TrimSpace(value.value) == "" || value.value != strings.TrimSpace(value.value) || len(value.value) > value.limit || strings.ContainsRune(value.value, 0) {
			return fmt.Errorf("worker_done %s is missing, non-canonical, or unbounded", name)
		}
	}
	if len(req.ChangedFiles) > 512 {
		return fmt.Errorf("worker_done changed files are unbounded")
	}
	for _, path := range req.ChangedFiles {
		if path == "" || strings.ContainsAny(path, ",\x00") {
			return fmt.Errorf("worker_done changed files cannot be represented exactly")
		}
	}
	if !filepath.IsAbs(req.ReportPath) || filepath.Clean(req.ReportPath) != req.ReportPath || len(req.ReportPath) > 4096 || strings.ContainsRune(req.ReportPath, 0) {
		return fmt.Errorf("worker_done report path must be an exact bounded absolute path")
	}
	return nil
}

func (c *Client) dispatchResult(ctx context.Context, argv []string) (port.OrcaDispatch, error) {
	var payload struct {
		Dispatch *struct {
			ID             string `json:"id"`
			TaskID         string `json:"task_id"`
			AssigneeHandle string `json:"assignee_handle"`
			Status         string `json:"status"`
		} `json:"dispatch"`
		Injected bool   `json:"injected"`
		Preamble string `json:"preamble"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if payload.Dispatch == nil {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "not_found"}
	}
	return port.OrcaDispatch{RuntimeID: runtimeID, ID: payload.Dispatch.ID, TaskID: payload.Dispatch.TaskID, AssigneeHandle: payload.Dispatch.AssigneeHandle, Status: payload.Dispatch.Status, Injected: payload.Injected, Preamble: payload.Preamble}, nil
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
	ID                string `json:"id"`
	InstanceID        string `json:"instanceId"`
	RepoID            string `json:"repoId"`
	Path              string `json:"path"`
	Head              string `json:"head"`
	Branch            string `json:"branch"`
	DisplayName       string `json:"displayName"`
	Comment           string `json:"comment"`
	BaseRef           string `json:"baseRef"`
	LinkedIssue       int    `json:"linkedIssue"`
	LinkedGitLabIssue *int   `json:"linkedGitLabIssue"`
}

func (w worktreePayload) portValue() port.OrcaWorktree {
	branch := strings.TrimPrefix(strings.TrimSpace(w.Branch), "refs/heads/")
	return port.OrcaWorktree{ID: w.ID, InstanceID: w.InstanceID, RepoID: w.RepoID, Path: w.Path, Head: w.Head, Branch: branch, Name: w.DisplayName, Comment: w.Comment, BaseRef: w.BaseRef, Issue: w.LinkedIssue, GitLabIssue: w.LinkedGitLabIssue}
}

type terminalPayload struct {
	Handle       string `json:"handle"`
	PTYID        string `json:"ptyId"`
	WorktreeID   string `json:"worktreeId"`
	WorktreePath string `json:"worktreePath"`
	TabID        string `json:"tabId"`
	LeafID       string `json:"leafId"`
	Title        string `json:"title"`
	Connected    bool   `json:"connected"`
	Writable     bool   `json:"writable"`
}

type visualLayoutPayload struct {
	Root struct {
		Tabs []struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		} `json:"tabs"`
	} `json:"root"`
}

type taskPayload struct {
	ID          string          `json:"id"`
	TaskTitle   string          `json:"task_title"`
	DisplayName string          `json:"display_name"`
	Status      string          `json:"status"`
	CompletedAt string          `json:"completed_at"`
	Result      json.RawMessage `json:"result"`
}

func (t taskPayload) portValue() port.OrcaTask {
	return port.OrcaTask{ID: t.ID, Title: t.TaskTitle, DisplayName: t.DisplayName, Status: t.Status, CompletedAt: t.CompletedAt, HasResult: hasJSONValue(t.Result)}
}

func hasJSONValue(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text) != ""
	}
	return true
}

func requireReturnedCount(kind string, length int, count *int) error {
	value := -1
	if count != nil {
		value = *count
	}
	if count == nil || value != length {
		return fmt.Errorf("Orca %s list is incomplete: count=%d returned=%d", kind, value, length)
	}
	return nil
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
	return port.OrcaTerminal{Handle: t.Handle, PTYID: t.PTYID, WorktreeID: t.WorktreeID, WorktreePath: t.WorktreePath, TabID: t.TabID, LeafID: t.LeafID, Title: t.Title, Connected: t.Connected, Writable: t.Writable}
}

func stableVisualTabTitles(layouts []visualLayoutPayload) (map[string]string, error) {
	if len(layouts) > port.OrcaMaxBaselineIDs {
		return nil, fmt.Errorf("Orca visual layout inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
	}
	result := make(map[string]string)
	totalTabs := 0
	for _, layout := range layouts {
		if len(layout.Root.Tabs) > port.OrcaMaxBaselineIDs {
			return nil, fmt.Errorf("Orca visual tab inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
		}
		totalTabs += len(layout.Root.Tabs)
		if totalTabs > port.OrcaMaxBaselineIDs {
			return nil, fmt.Errorf("Orca visual tab inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
		}
		for _, tab := range layout.Root.Tabs {
			tabID := strings.TrimSpace(tab.TabID)
			leafID := strings.TrimSpace(tab.ActiveLeafID)
			title := strings.TrimSpace(tab.Title)
			if tabID == "" || leafID == "" || title == "" {
				continue
			}
			if tabID != tab.TabID || leafID != tab.ActiveLeafID || title != tab.Title || len(tabID) > 1024 || len(leafID) > 1024 || len(title) > 4096 {
				return nil, fmt.Errorf("Orca visual tab identity is not canonical and bounded")
			}
			key := visualTabKey(tabID, leafID)
			if previous, ok := result[key]; ok && previous != title {
				return nil, fmt.Errorf("Orca visual tab identity has conflicting titles")
			}
			result[key] = title
		}
	}
	return result, nil
}

func visualTabKey(tabID, leafID string) string {
	if strings.TrimSpace(tabID) == "" || strings.TrimSpace(leafID) == "" {
		return ""
	}
	return strings.TrimSpace(tabID) + "\x00" + strings.TrimSpace(leafID)
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

func orcaIssueProvider(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "github"
	}
	return value, value == "github" || value == "gitlab"
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
