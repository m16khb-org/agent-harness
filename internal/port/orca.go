package port

import (
	"context"
	"fmt"
)

const OrcaMaxBaselineIDs = 512

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
	Repo       string `json:"repo"`
	Name       string `json:"name"`
	BaseBranch string `json:"base_branch"`
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
	Title                     string `json:"title,omitempty"`
	AllowCodexHookTrustBypass bool   `json:"allow_codex_hook_trust_bypass,omitempty"`
}

type OrcaTask struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status,omitempty"`
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
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	AssigneeHandle string `json:"assignee_handle,omitempty"`
	Status         string `json:"status,omitempty"`
	Injected       bool   `json:"injected,omitempty"`
	Preamble       string `json:"preamble,omitempty"`
}

type OrcaMessage struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

type OrcaClient interface {
	Probe(context.Context, OrcaProbeRequest) (OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]OrcaWorktree, error)
	CreateWorktree(context.Context, OrcaCreateWorktreeRequest) (OrcaWorktree, error)
	RemoveWorktree(context.Context, string, bool) error
	ListTerminals(context.Context, string) ([]OrcaTerminal, error)
	CreateTerminal(context.Context, OrcaCreateTerminalRequest) (OrcaTerminal, error)
	RefreshTerminal(context.Context, string, string) (OrcaTerminal, error)
	ListTasks(context.Context) ([]OrcaTask, error)
	CreateTask(context.Context, OrcaCreateTaskRequest) (OrcaTask, error)
	UpdateTask(context.Context, string, string, string) error
	Dispatch(context.Context, OrcaDispatchRequest) (OrcaDispatch, error)
	ShowDispatch(context.Context, string) (OrcaDispatch, error)
}
