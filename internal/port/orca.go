package port

import (
	"context"
	"fmt"
)

const (
	OrcaMaxBaselineIDs = 512

	// IssueOps implementer(하위 세션 execution owner)의 host별 기본 모델.
	// execution prepare가 --owner-model/--owner-effort 미지정 호출에 적용한다.
	IssueOpsImplementerModelCodex  = "gpt-5.6-terra"
	IssueOpsImplementerEffortCodex = "xhigh"
	IssueOpsImplementerModelClaude = "claude-opus-4-8"
	// claude CLI의 --effort <level> 플래그 실지원을 확인함(2026-07-24).
	// 플래그가 제거되면 ownerAgentCommand(adapter/orca/client.go)의 claude
	// 분기에서 effort 인자를 조건부 생략으로 되돌린다.
	IssueOpsImplementerEffortClaude = "high"

	// IssueOps planner(계획/리뷰 세션)의 host별 기본 모델. 하위 세션이 구현
	// diff의 brooks 적대 리뷰 서브에이전트를 띄울 때 사용한다(설계 v5 WS5).
	IssueOpsPlannerModelCodex   = "gpt-5.6-sol"
	IssueOpsPlannerEffortCodex  = "xhigh"
	IssueOpsPlannerModelClaude  = "claude-fable-5"
	IssueOpsPlannerEffortClaude = "high"
)

// IssueOpsPlannerDefaults는 host별 planner(reviewer급) 기본 모델/effort를
// 반환한다.
func IssueOpsPlannerDefaults(host string) (model string, effort string, ok bool) {
	switch host {
	case "codex":
		return IssueOpsPlannerModelCodex, IssueOpsPlannerEffortCodex, true
	case "claude":
		return IssueOpsPlannerModelClaude, IssueOpsPlannerEffortClaude, true
	}
	return "", "", false
}

// IssueOpsImplementerDefaults는 host별 implementer 기본 모델/effort를 반환한다.
// 지원하지 않는 host면 ok=false를 반환하고 호출자가 host 검증 에러를 처리한다.
func IssueOpsImplementerDefaults(host string) (model string, effort string, ok bool) {
	switch host {
	case "codex":
		return IssueOpsImplementerModelCodex, IssueOpsImplementerEffortCodex, true
	case "claude":
		return IssueOpsImplementerModelClaude, IssueOpsImplementerEffortClaude, true
	}
	return "", "", false
}

type OrcaError struct {
	Code    string `json:"code"`
	Detail  string `json:"detail,omitempty"`
	Invoked bool   `json:"invoked,omitempty"`
	Timeout bool   `json:"timeout,omitempty"`
}

func (e *OrcaError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

type OrcaProbeRequest struct {
	Repo     string `json:"repo"`
	Agent    string `json:"agent"`
	Provider string `json:"provider,omitempty"`
}

type OrcaProbeResult struct {
	Available        bool   `json:"available"`
	Ready            bool   `json:"ready"`
	Code             string `json:"code,omitempty"`
	Detail           string `json:"detail,omitempty"`
	RuntimeID        string `json:"runtime_id,omitempty"`
	RepoID           string `json:"repo_id,omitempty"`
	RepoPath         string `json:"repo_path,omitempty"`
	RepoRemoteName   string `json:"repo_remote_name,omitempty"`
	WorktreeBasePath string `json:"worktree_base_path,omitempty"`
	Agent            string `json:"agent,omitempty"`
	Provider         string `json:"provider,omitempty"`
}

type OrcaStatus struct {
	RuntimeID        string `json:"runtime_id,omitempty"`
	RuntimeReachable bool   `json:"runtime_reachable"`
	RuntimeState     string `json:"runtime_state,omitempty"`
	GraphState       string `json:"graph_state,omitempty"`
}

type OrcaRepo struct {
	RuntimeID        string `json:"-"`
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name,omitempty"`
	RemoteName       string `json:"remote_name,omitempty"`
	WorktreeBasePath string `json:"worktree_base_path,omitempty"`
}

type OrcaWorktree struct {
	RuntimeID   string `json:"runtime_id,omitempty"`
	ID          string `json:"id"`
	InstanceID  string `json:"instance_id,omitempty"`
	RepoID      string `json:"repo_id,omitempty"`
	Path        string `json:"path"`
	Head        string `json:"head,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Name        string `json:"name,omitempty"`
	Comment     string `json:"comment,omitempty"`
	BaseRef     string `json:"base_ref,omitempty"`
	Issue       int    `json:"issue,omitempty"`
	GitLabIssue *int   `json:"gitlab_issue,omitempty"`
}

type OrcaCreateWorktreeRequest struct {
	Repo           string `json:"repo"`
	Name           string `json:"name"`
	BaseBranch     string `json:"base_branch"`
	UpstreamBranch string `json:"upstream_branch,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Issue          int    `json:"issue,omitempty"`
	Comment        string `json:"comment"`
}

type OrcaAdoptWorktreeRequest struct {
	WorktreeID string `json:"worktree_id"`
	Provider   string `json:"provider,omitempty"`
	Issue      int    `json:"issue,omitempty"`
	Comment    string `json:"comment"`
}

type OrcaTerminal struct {
	RuntimeID      string `json:"runtime_id,omitempty"`
	Handle         string `json:"handle"`
	PTYID          string `json:"pty_id"`
	WorktreeID     string `json:"worktree_id"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	TabID          string `json:"tab_id,omitempty"`
	LeafID         string `json:"leaf_id,omitempty"`
	StableTabTitle string `json:"stable_tab_title,omitempty"`
	Title          string `json:"title,omitempty"`
	Connected      bool   `json:"connected"`
	Writable       bool   `json:"writable"`
}

type OrcaCreateTerminalRequest struct {
	WorktreeID                string `json:"worktree_id"`
	Agent                     string `json:"agent"`
	Model                     string `json:"model,omitempty"`
	ReasoningEffort           string `json:"reasoning_effort,omitempty"`
	Title                     string `json:"title,omitempty"`
	AllowCodexHookTrustBypass bool   `json:"allow_codex_hook_trust_bypass,omitempty"`
}

// OrcaBootstrapTerminalAgentRequest starts the selected supported host agent
// in an already-owned sole-writer terminal. It is limited to recovery of an
// exact owned worktree terminal; it never targets an arbitrary shell.
type OrcaBootstrapTerminalAgentRequest struct {
	TerminalHandle            string `json:"terminal_handle"`
	Agent                     string `json:"agent"`
	Model                     string `json:"model,omitempty"`
	ReasoningEffort           string `json:"reasoning_effort,omitempty"`
	AllowCodexHookTrustBypass bool   `json:"allow_codex_hook_trust_bypass,omitempty"`
}

type OrcaTask struct {
	RuntimeID   string `json:"-"`
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	HasResult   bool   `json:"has_result,omitempty"`
}

type OrcaGate struct {
	RuntimeID string `json:"-"`
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
}

type OrcaInboxPresence struct {
	RuntimeID       string `json:"-"`
	Count           int    `json:"count"`
	RowCount        int    `json:"row_count"`
	CompleteAbsence bool   `json:"complete_absence"`
}

type OrcaCreateTaskRequest struct {
	Spec        string `json:"spec"`
	Title       string `json:"title"`
	DisplayName string `json:"display_name"`
}

type OrcaDispatchRequest struct {
	TaskID         string `json:"task_id"`
	ToHandle       string `json:"to_handle"`
	FromHandle     string `json:"from_handle,omitempty"`
	Inject         bool   `json:"inject"`
	ReturnPreamble bool   `json:"return_preamble"`
}

type OrcaDispatch struct {
	RuntimeID      string `json:"-"`
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	AssigneeHandle string `json:"assignee_handle,omitempty"`
	Status         string `json:"status,omitempty"`
	Injected       bool   `json:"injected,omitempty"`
	Preamble       string `json:"preamble,omitempty"`
}

type OrcaMessage struct {
	ID         string `json:"id,omitempty"`
	FromHandle string `json:"from_handle,omitempty"`
	ToHandle   string `json:"to_handle,omitempty"`
	Type       string `json:"type,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Body       string `json:"body,omitempty"`
	Sequence   int64  `json:"sequence,omitempty"`
}

type OrcaWorkerDoneRequest struct {
	FromHandle   string   `json:"from_handle"`
	ToHandle     string   `json:"to_handle"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	TaskID       string   `json:"task_id"`
	DispatchID   string   `json:"dispatch_id"`
	ChangedFiles []string `json:"changed_files"`
	ReportPath   string   `json:"report_path"`
}

type OrcaWorkerDoneResult struct {
	MessageID string `json:"message_id"`
	Sequence  int64  `json:"sequence"`
}

type OrcaWorkerDoneClient interface {
	SendWorkerDone(context.Context, OrcaWorkerDoneRequest) (OrcaWorkerDoneResult, error)
}

type OrcaClient interface {
	Probe(context.Context, OrcaProbeRequest) (OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]OrcaWorktree, error)
	ShowWorktree(context.Context, string) (OrcaWorktree, error)
	CreateWorktree(context.Context, OrcaCreateWorktreeRequest) (OrcaWorktree, error)
	AdoptWorktree(context.Context, OrcaAdoptWorktreeRequest) (OrcaWorktree, error)
	RemoveWorktree(context.Context, string, bool) error
	ListTerminals(context.Context, string) ([]OrcaTerminal, error)
	CreateTerminal(context.Context, OrcaCreateTerminalRequest) (OrcaTerminal, error)
	RefreshTerminal(context.Context, string, string) (OrcaTerminal, error)
	ListTasks(context.Context) ([]OrcaTask, error)
	ListDispatchedTasks(context.Context) ([]OrcaTask, error)
	CreateTask(context.Context, OrcaCreateTaskRequest) (OrcaTask, error)
	UpdateTask(context.Context, string, string, string) error
	Dispatch(context.Context, OrcaDispatchRequest) (OrcaDispatch, error)
	ShowDispatch(context.Context, string) (OrcaDispatch, error)
	ShowDispatchFrom(context.Context, string, string) (OrcaDispatch, error)
}
