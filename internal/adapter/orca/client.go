package orca

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"issueops/internal/port"
)

const (
	readTimeout   = 10 * time.Second
	createTimeout = 2 * time.Minute
)

var (
	concreteTerminalHandlePattern = regexp.MustCompile(`^term_[A-Za-z0-9_-]+$`)
	exactGitObjectIDPattern       = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)
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

func (c *Client) Available() bool {
	_, err := c.runner.LookPath("orca")
	return err == nil
}

func (c *Client) Status(ctx context.Context) (port.OrcaStatus, error) {
	var payload struct {
		App struct {
			PID int `json:"pid"`
		} `json:"app"`
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
	return port.OrcaStatus{RuntimeID: runtimeID, RuntimeReachable: payload.Runtime.Reachable, RuntimeState: payload.Runtime.State, GraphState: payload.Graph.State, AppPID: payload.App.PID}, nil
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
		{argv: []string{"orca", "terminal", "create", "--help"}, want: []string{"--worktree", "--command", "--title", "--json"}},
		{argv: []string{"orca", "terminal", "list", "--help"}, want: []string{"--worktree", "--limit", "--json"}},
		{argv: []string{"orca", "orchestration", "run-create", "--help"}, want: []string{"--objective", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "run-list", "--help"}, want: []string{"--cursor", "--json"}},
		{argv: []string{"orca", "orchestration", "run-current", "--help"}, want: []string{"--from", "--json"}},
		{argv: []string{"orca", "orchestration", "run-use", "--help"}, want: []string{"--id", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "task-create", "--help"}, want: []string{"--spec", "--task-title", "--display-name", "--run", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "task-list", "--help"}, want: []string{"--ready", "--status", "--run", "--json"}},
		{argv: []string{"orca", "orchestration", "gate-list", "--help"}, want: []string{"--run", "--json"}},
		{argv: []string{"orca", "orchestration", "task-update", "--help"}, want: []string{"--id", "--status", "--result", "--run", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch", "--help"}, want: []string{"--task", "--to", "--run", "--from", "--inject", "--return-preamble", "--json"}},
		{argv: []string{"orca", "orchestration", "dispatch-show", "--help"}, want: []string{"--task", "--preamble", "--from", "--json"}},
		{argv: []string{"orca", "orchestration", "send", "--help"}, want: []string{"--run", "--to", "--from", "--type", "--subject", "--body", "--task-id", "--dispatch-id", "--outcome", "--files-modified", "--report-path", "--json"}},
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
		if !containsAllHelpFlags(help, []string{"--model", "--config"}) {
			result.Code = "host_model_selection_unsupported"
			return result, nil
		}
	}
	if agent == "claude" || agent == "omo" {
		help, err := c.runText(ctx, "", readTimeout, []string{agent, "--help"})
		if err != nil || !containsAllHelpFlags(help, []string{"--model"}) {
			result.Code = "host_model_selection_unsupported"
			return result, nil
		}
	}
	currentRun, err := c.currentRunInventory(ctx)
	if err == nil {
		err = validateExecutionInventoryRuntime(currentRun.RuntimeID, status.RuntimeID)
	}
	if err != nil {
		result.Code = "orchestration_unready"
		result.Detail = boundedDiagnostic(err.Error())
		return result, nil
	}
	runs, err := c.listRunsInventory(ctx)
	if err == nil {
		err = validateExecutionInventoryRuntime(runs.RuntimeID, status.RuntimeID)
	}
	if err != nil {
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
	preparedRemote := ""
	if provider == "gitlab" && exactGitObjectIDPattern.MatchString(strings.TrimSpace(req.BaseBranch)) {
		candidate := "refs/remotes/origin/" + strings.TrimSpace(req.Name)
		if c.gitRefMatches(ctx, req.Repo, candidate, req.BaseBranch) {
			preparedRemote = candidate
		}
	}
	argv := []string{"orca", "worktree", "create", "--repo", pathSelector(req.Repo), "--name", strings.TrimSpace(req.Name), "--base-branch", strings.TrimSpace(req.BaseBranch)}
	if parent := strings.TrimSpace(req.ParentWorktree); parent != "" {
		argv = append(argv, "--parent-worktree", pathSelector(parent))
	} else {
		argv = append(argv, "--no-parent")
	}
	argv = append(argv, "--setup", "skip", "--comment", strings.TrimSpace(req.Comment), "--json")
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
	if err != nil {
		return created, err
	}
	requestedBranch := strings.TrimSpace(req.Name)
	if created.Branch == "" || created.Branch == requestedBranch {
		return created, nil
	}
	upstream := strings.TrimSpace(req.UpstreamBranch)
	allowNumericSuffix := false
	if upstream == "" && preparedRemote != "" &&
		strings.EqualFold(strings.TrimSpace(created.Head), strings.TrimSpace(req.BaseBranch)) &&
		c.gitRefMatches(ctx, req.Repo, preparedRemote, req.BaseBranch) {
		upstream = preparedRemote
		allowNumericSuffix = true
	}
	if upstream == "" && !exactGitObjectIDPattern.MatchString(strings.TrimSpace(req.BaseBranch)) {
		upstream = strings.TrimSpace(req.BaseBranch)
	}
	return c.canonicalizeWorktreeBranch(ctx, created, requestedBranch, upstream, allowNumericSuffix)
}

// CanonicalizeWorktreeBranch는 Orca가 만든 브랜치가 정확히
// <namespace>/<provider-branch>일 때만 namespace를 제거한다. GitLab 예약
// 브랜치의 숫자 접미사 허용과 upstream 복원은 내부 호출에서 원격 SHA까지
// 증명한 경우에만 수행한다.
func (c *Client) CanonicalizeWorktreeBranch(ctx context.Context, created port.OrcaWorktree, requestedBranch, upstream string) (port.OrcaWorktree, error) {
	return c.canonicalizeWorktreeBranch(ctx, created, requestedBranch, upstream, false)
}

func (c *Client) canonicalizeWorktreeBranch(ctx context.Context, created port.OrcaWorktree, requestedBranch, upstream string, allowNumericSuffix bool) (port.OrcaWorktree, error) {
	requestedBranch = strings.TrimSpace(requestedBranch)
	upstream = strings.TrimSpace(upstream)
	namespaced := strings.HasSuffix(created.Branch, "/"+requestedBranch) &&
		strings.TrimSuffix(created.Branch, "/"+requestedBranch) != ""
	numericSuffix := allowNumericSuffix && exactNumericBranchSuffix(created.Branch, requestedBranch)
	if requestedBranch == "" || (!namespaced && !numericSuffix) || !filepath.IsAbs(strings.TrimSpace(created.Path)) {
		return created, &port.OrcaError{Code: "worktree_branch_mismatch", Detail: fmt.Sprintf("created branch %q does not match requested branch %q", created.Branch, requestedBranch), Invoked: true}
	}
	gitCommands := [][]string{{"git", "branch", "-m", requestedBranch}}
	if upstream != "" {
		gitCommands = append(gitCommands, []string{"git", "branch", "--set-upstream-to", upstream, requestedBranch})
	}
	for _, gitArgv := range gitCommands {
		if _, runErr := c.runner.Run(ctx, filepath.Clean(created.Path), createTimeout, gitArgv); runErr != nil {
			return created, fmt.Errorf("canonicalize Orca worktree branch: %w", runErr)
		}
	}
	created.Branch = requestedBranch
	return created, nil
}

func (c *Client) gitRefMatches(ctx context.Context, repo, ref, wantOID string) bool {
	output, err := c.runner.Run(ctx, strings.TrimSpace(repo), readTimeout,
		[]string{"git", "rev-parse", "--verify", "--quiet", strings.TrimSpace(ref)})
	return err == nil && strings.EqualFold(strings.TrimSpace(string(output.Stdout)), strings.TrimSpace(wantOID))
}

func exactNumericBranchSuffix(observed, requested string) bool {
	suffix, ok := strings.CutPrefix(strings.TrimSpace(observed), strings.TrimSpace(requested)+"-")
	if !ok || suffix == "" {
		return false
	}
	number, err := strconv.Atoi(suffix)
	return err == nil && number >= 2 && strconv.Itoa(number) == suffix
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
	inventory, err := c.listTerminalsInventory(ctx, worktreeID)
	return inventory.Rows, err
}

func (c *Client) listTerminalsInventory(ctx context.Context, worktreeID string) (executionTerminalInventory, error) {
	selector := ""
	if strings.TrimSpace(worktreeID) != "" {
		selector = idSelector(worktreeID)
	}
	return c.listTerminalsBySelector(ctx, selector)
}

// ListAllTerminals는 런타임의 모든 터미널 행이다. cleanup이 요청자 터미널을
// ORCA_PANE_KEY/ORCA_TERMINAL_HANDLE env와 join해 확정하는 데 쓴다(#477).
func (c *Client) ListAllTerminals(ctx context.Context) ([]port.OrcaTerminal, error) {
	inventory, err := c.listTerminalsBySelector(ctx, "")
	return inventory.Rows, err
}

// ListWorktreeTerminalsByPath는 경로 선택자로 그 워크트리의 터미널을 돌려준다.
// Orca에 등록되지 않은 워크트리는 구조화된 selector_not_found를 주며, 그것은
// "터미널 없음"이지 관측 실패가 아니다(2026-08-27 실측).
func (c *Client) ListWorktreeTerminalsByPath(ctx context.Context, path string) ([]port.OrcaTerminal, error) {
	inventory, err := c.listTerminalsBySelector(ctx, pathSelector(path))
	if isOrcaSelectorNotFound(err) {
		return nil, nil
	}
	return inventory.Rows, err
}

// closeTerminalTimeout은 terminal close 상한이다. 실측 약 2초에 여유를 두되
// createTimeout(2분)처럼 armed cleanup을 오래 붙잡지 않는다.
const closeTerminalTimeout = 15 * time.Second

// CloseTerminal은 fingerprint가 승인한 exact handle만 닫고, Orca가 같은 handle의
// PTY 종료를 확인한 receipt를 돌려준 경우에만 성공한다.
func (c *Client) CloseTerminal(ctx context.Context, handle string) error {
	handle = strings.TrimSpace(handle)
	if !concreteTerminalHandlePattern.MatchString(handle) {
		return fmt.Errorf("invalid Orca terminal handle %q", handle)
	}
	var payload struct {
		Close struct {
			Handle         string `json:"handle"`
			PTYKilled      bool   `json:"ptyKilled"`
			PTYStopVerdict string `json:"ptyStopVerdict"`
			PTYStopReason  string `json:"ptyStopReason"`
		} `json:"close"`
	}
	_, err := c.runJSON(ctx, "", closeTerminalTimeout, []string{"orca", "terminal", "close", "--terminal", handle, "--json"}, &payload)
	if err != nil {
		// background 워크트리 터미널(가시 탭 없음)은 close가 PTY를 죽인 뒤에도
		// tab 조회 경합으로 runtime_error/tab_not_found를 돌려준다(2026-08-27
		// 실측: zsh 종료·inventory 공함). 부재로 정규화하고, 실제 생존 여부는
		// 호출자의 최종 worktree inventory 증명이 거부한다.
		if isOrcaTabNotFound(err) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(payload.Close.Handle) != handle {
		return fmt.Errorf("Orca terminal close receipt handle %q does not match %q", payload.Close.Handle, handle)
	}
	if !payload.Close.PTYKilled {
		return fmt.Errorf("Orca terminal %s PTY death is unconfirmed: verdict=%s reason=%s", handle, payload.Close.PTYStopVerdict, payload.Close.PTYStopReason)
	}
	return nil
}

func isOrcaSelectorNotFound(err error) bool {
	orcaErr, ok := errors.AsType[*port.OrcaError](err)
	return ok && orcaErr.Code == "selector_not_found"
}

func isOrcaTabNotFound(err error) bool {
	orcaErr, ok := errors.AsType[*port.OrcaError](err)
	return ok && orcaErr.Code == "runtime_error" && orcaErr.Detail == "tab_not_found"
}

func (c *Client) listTerminalsBySelector(ctx context.Context, selector string) (executionTerminalInventory, error) {
	var payload struct {
		Terminals     []terminalPayload     `json:"terminals"`
		VisualLayouts []visualLayoutPayload `json:"visualLayouts"`
		TotalCount    *int                  `json:"totalCount"`
		Truncated     bool                  `json:"truncated"`
	}
	argv := []string{"orca", "terminal", "list"}
	if selector != "" {
		argv = append(argv, "--worktree", selector)
	}
	argv = append(argv, "--limit", strconv.Itoa(port.OrcaMaxBaselineIDs), "--json")
	runtimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
	if err != nil {
		return executionTerminalInventory{}, err
	}
	if err := requireCompleteList("terminal", len(payload.Terminals), payload.TotalCount, payload.Truncated); err != nil {
		return executionTerminalInventory{}, err
	}
	stableTitles, err := stableVisualTabTitles(payload.VisualLayouts)
	if err != nil {
		return executionTerminalInventory{}, err
	}
	result := make([]port.OrcaTerminal, 0, len(payload.Terminals))
	for _, item := range payload.Terminals {
		value := item.portValue()
		value.RuntimeID = runtimeID
		value.StableTabTitle = stableTitles[visualTabKey(value.TabID, value.LeafID)]
		result = append(result, value)
	}
	return executionTerminalInventory{RuntimeID: runtimeID, Rows: result, Complete: true}, nil
}

func (c *Client) showTerminalInventory(ctx context.Context, handle string) (executionTerminalDetailInventory, error) {
	handle = strings.TrimSpace(handle)
	if !concreteTerminalHandlePattern.MatchString(handle) {
		return executionTerminalDetailInventory{}, &port.OrcaError{Code: "terminal_handle_invalid", Detail: "a concrete terminal handle is required"}
	}
	var payload struct {
		Terminal terminalPayload `json:"terminal"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "terminal", "show", "--terminal", handle, "--json"}, &payload)
	if err != nil {
		return executionTerminalDetailInventory{}, err
	}
	terminal := payload.Terminal.portValue()
	terminal.RuntimeID = runtimeID
	return executionTerminalDetailInventory{
		RuntimeID: runtimeID, Terminal: terminal, PaneRuntimeID: payload.Terminal.PaneRuntimeID,
	}, nil
}

func (c *Client) CreateTerminal(ctx context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	command, ok := ownerAgentCommand(req.Agent, req.Model, req.ReasoningEffort, req.AllowCodexHookTrustBypass)
	if !ok {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "unsupported_agent_profile", Detail: strings.TrimSpace(req.Agent)}
	}
	help, err := c.runText(ctx, "", readTimeout, []string{"orca", "terminal", "create", "--help"})
	if err != nil {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_create_capability_unavailable", Detail: boundedDiagnostic(err.Error())}
	}
	if !containsAllHelpFlags(help, []string{"--worktree", "--command", "--title", "--json"}) {
		return port.OrcaTerminal{}, &port.OrcaError{Code: "terminal_create_capability_missing", Detail: "installed Orca does not expose the fixed --command launch shape"}
	}
	argv := []string{"orca", "terminal", "create", "--worktree", idSelector(req.WorktreeID), "--command", command}
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

// BootstrapTerminalAgent turns an exact, already-owned terminal into
// an Orca-recognized agent target before inject dispatch. The worker terminal
// is selected and sole-writer-attested by IssueOps; this adapter only emits a
// fixed host command and waits for Orca to settle its TUI state.
func (c *Client) BootstrapTerminalAgent(ctx context.Context, req port.OrcaBootstrapTerminalAgentRequest) error {
	if strings.TrimSpace(req.TerminalHandle) == "" {
		return &port.OrcaError{Code: "terminal_agent_bootstrap_invalid", Detail: "terminal handle is required"}
	}
	command, ok := ownerAgentCommand(req.Agent, req.Model, req.ReasoningEffort, req.AllowCodexHookTrustBypass)
	if !ok {
		return &port.OrcaError{Code: "unsupported_agent_profile", Detail: strings.TrimSpace(req.Agent)}
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

func (c *Client) SendTerminalPrompt(ctx context.Context, handle, prompt string) error {
	handle = strings.TrimSpace(handle)
	if !concreteTerminalHandlePattern.MatchString(handle) || strings.TrimSpace(prompt) == "" ||
		strings.ContainsAny(prompt, "\x00\x1b") {
		return &port.OrcaError{Code: "terminal_prompt_invalid"}
	}
	prompt = "\x1b[200~" + prompt + "\x1b[201~"
	var payload struct {
		Send struct {
			Accepted bool `json:"accepted"`
		} `json:"send"`
	}
	if _, err := c.runJSON(ctx, "", createTimeout, []string{
		"orca", "terminal", "send", "--terminal", handle, "--text", prompt, "--enter", "--json",
	}, &payload); err != nil {
		return err
	}
	if !payload.Send.Accepted {
		return &port.OrcaError{Code: "terminal_prompt_rejected", Invoked: true}
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
