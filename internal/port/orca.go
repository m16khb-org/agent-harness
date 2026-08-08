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
	// Claude Code 자동 체인은 Opus 5 planner가 계획·리뷰하고 Sonnet 5
	// implementer가 실행한다. Fable 5는 명시적 수동 지정에만 사용한다.
	IssueOpsImplementerModelClaude = "claude-sonnet-5"
	// claude CLI의 --effort <level> 플래그 실지원을 확인함(2026-07-24).
	// 플래그가 제거되면 ownerAgentCommand(adapter/orca/client.go)의 claude
	// 분기에서 effort 인자를 조건부 생략으로 되돌린다.
	IssueOpsImplementerEffortClaude = "high"

	// IssueOps planner(계획/리뷰 세션)의 host별 기본 모델. 하위 세션이 구현
	// diff의 brooks 적대 리뷰 서브에이전트를 띄울 때 사용한다(설계 v5 WS5).
	IssueOpsPlannerModelCodex   = "gpt-5.6-sol"
	IssueOpsPlannerEffortCodex  = "xhigh"
	IssueOpsPlannerModelClaude  = "claude-opus-5"
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

type OrcaRun struct {
	RuntimeID string `json:"-"`
	ID        string `json:"id"`
	Objective string `json:"objective"`
}

type OrcaCreateRunRequest struct {
	Objective string `json:"objective"`
}

type OrcaWorktree struct {
	RuntimeID         string `json:"runtime_id,omitempty"`
	ID                string `json:"id"`
	InstanceID        string `json:"instance_id,omitempty"`
	RepoID            string `json:"repo_id,omitempty"`
	Path              string `json:"path"`
	Head              string `json:"head,omitempty"`
	Branch            string `json:"branch,omitempty"`
	Name              string `json:"name,omitempty"`
	Comment           string `json:"comment,omitempty"`
	BaseRef           string `json:"base_ref,omitempty"`
	Issue             int    `json:"issue,omitempty"`
	GitLabIssue       *int   `json:"gitlab_issue,omitempty"`
	ParentWorktreeID  string `json:"parent_worktree_id,omitempty"`
	LineageSource     string `json:"lineage_source,omitempty"`
	LineageConfidence string `json:"lineage_confidence,omitempty"`
}

type OrcaCreateWorktreeRequest struct {
	Repo           string `json:"repo"`
	Name           string `json:"name"`
	BaseBranch     string `json:"base_branch"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
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
	RunID       string `json:"run_id,omitempty"`
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
	RunID       string `json:"run_id"`
	Spec        string `json:"spec"`
	Title       string `json:"title"`
	DisplayName string `json:"display_name"`
}

type OrcaDispatchRequest struct {
	RunID          string `json:"run_id"`
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
	RunID        string   `json:"run_id"`
	FromHandle   string   `json:"from_handle"`
	ToHandle     string   `json:"to_handle"`
	Subject      string   `json:"subject"`
	Body         string   `json:"body"`
	TaskID       string   `json:"task_id"`
	DispatchID   string   `json:"dispatch_id"`
	Outcome      string   `json:"outcome"`
	ChangedFiles []string `json:"changed_files"`
	ReportPath   string   `json:"report_path"`
}

type OrcaWorkerDoneResult struct {
	MessageID string `json:"message_id"`
	Sequence  int64  `json:"sequence"`
}

// OrcaWorkerDoneClient는 core와 CLI에서 호출되지 않는다. 의도된 상태이며 dead
// code 스윕이 삭제 후보로 다시 조사하지 않도록 판단 근거를 남긴다(#127에서 보존
// 결정).
//
// 왜 배선이 없는가: IssueOps v1 owner 명령 카탈로그는 execution status/claim,
// remote create-pr, execution complete, implementation-review record 5개만
// 정의하며 Orca 메시지 전송을 요구하지 않는다. owner의 완료 보고는 durable
// state(execution complete)와 원격 artifact(remote create-pr)가 담당한다. Orca는
// workspace/owner adapter이지 두 번째 workflow authority가 아니다.
//
// #130이 그 공백을 UpdateTask로 메웠으므로 이 경로는 배선되지 않는다. 두 방식은
// 목적이 다르다: UpdateTask는 task 상태를 종결시키고, SendWorkerDone은 Orca
// dispatch 프로토콜의 완료 메시지를 보낸다. 후자는 owner 세션이 그 명령을
// 실행해야 하는데 그것은 owner 명령 카탈로그 확장이며 위 계약과 충돌한다.
//
// 어떤 조건에서 배선되는가: Orca dispatch 프로토콜의 메시지 채널이 agent-harness
// 계약에 편입될 때다. 그런 요구가 생기기 전까지 이 인터페이스는 adapter가 이미
// 구현한 능력의 선언으로 남는다.
type OrcaWorkerDoneClient interface {
	SendWorkerDone(context.Context, OrcaWorkerDoneRequest) (OrcaWorkerDoneResult, error)
}

type OrcaClient interface {
	Probe(context.Context, OrcaProbeRequest) (OrcaProbeResult, error)
	ListRuns(context.Context) ([]OrcaRun, error)
	CreateRun(context.Context, OrcaCreateRunRequest) (OrcaRun, error)
	CurrentRun(context.Context) (*OrcaRun, error)
	UseRun(context.Context, string) (OrcaRun, error)
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
	UpdateTask(context.Context, string, string, string, string) error
	Dispatch(context.Context, OrcaDispatchRequest) (OrcaDispatch, error)
	ShowDispatch(context.Context, string) (OrcaDispatch, error)
	ShowDispatchFrom(context.Context, string, string) (OrcaDispatch, error)
}
