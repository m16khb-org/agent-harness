package port

import "context"

type ExecutionWorkspaceRequest struct {
	LifecycleID    string `json:"lifecycle_id"`
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"base_branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
}

type ExecutionWorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	Exists         bool   `json:"exists,omitempty"`
}

type ExecutionWorkspaceProvisioner interface {
	Prepare(context.Context, ExecutionWorkspaceRequest) (ExecutionWorkspaceReceipt, error)
}

type ExecutionWorkspaceAccessResult struct {
	Allowed         bool   `json:"allowed"`
	Code            string `json:"code,omitempty"`
	RelaunchCommand string `json:"relaunch_command,omitempty"`
}

type ExecutionWorkspaceAccessProber interface {
	ProbeAccess(context.Context, ExecutionWorkspaceRequest, string) (ExecutionWorkspaceAccessResult, error)
}

type ExecutionIssueSnapshotRequest struct {
	Repo string `json:"repo"`
	URL  string `json:"url"`
}

type ExecutionIssueSnapshot struct {
	URL   string `json:"url"`
	Body  string `json:"body"`
	State string `json:"state,omitempty"`
}

type ExecutionIssueSnapshotReader interface {
	ReadIssueSnapshot(context.Context, ExecutionIssueSnapshotRequest) (ExecutionIssueSnapshot, error)
}

type ExecutionOrcaProbeRequest struct {
	Repo     string `json:"repo"`
	Host     string `json:"host"`
	Model    string `json:"model"`
	Effort   string `json:"effort,omitempty"`
	Provider string `json:"provider,omitempty"`
	Issue    int    `json:"issue,omitempty"`
	Marker   string `json:"marker"`
}

type ExecutionOrcaProbeResult struct {
	Available bool   `json:"available"`
	Ready     bool   `json:"ready"`
	Code      string `json:"code,omitempty"`
}

type ExecutionOrcaReceipt struct {
	Workspace          ExecutionWorkspaceReceipt `json:"workspace"`
	RuntimeID          string                    `json:"runtime_id"`
	RepoID             string                    `json:"repo_id"`
	WorktreeID         string                    `json:"worktree_id"`
	WorktreeInstanceID string                    `json:"worktree_instance_id,omitempty"`
	TaskID             string                    `json:"task_id"`
	DispatchID         string                    `json:"dispatch_id"`
	TerminalPTYID      string                    `json:"terminal_pty_id,omitempty"`
}

type ExecutionOrcaWorkspaceReceipt struct {
	Workspace          ExecutionWorkspaceReceipt `json:"workspace"`
	RuntimeID          string                    `json:"runtime_id"`
	RepoID             string                    `json:"repo_id"`
	WorktreeID         string                    `json:"worktree_id"`
	WorktreeInstanceID string                    `json:"worktree_instance_id,omitempty"`
}

type ExecutionOrcaLaunchRequest struct {
	Prompt              string `json:"prompt"`
	PromptPath          string `json:"prompt_path"`
	PromptSHA256        string `json:"prompt_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
}

type ExecutionOrcaIntentStage string

const (
	ExecutionOrcaIntentWorktree ExecutionOrcaIntentStage = "worktree_create"
	ExecutionOrcaIntentTerminal ExecutionOrcaIntentStage = "terminal_create"
	ExecutionOrcaIntentTask     ExecutionOrcaIntentStage = "task_create"
	ExecutionOrcaIntentDispatch ExecutionOrcaIntentStage = "dispatch"
)

// ExecutionOrcaIntentRequest is the complete, durable identity for one Orca
// mutation. The core persists this identity before InvokeIntent is allowed.
type ExecutionOrcaIntentRequest struct {
	Stage         ExecutionOrcaIntentStage       `json:"stage"`
	Marker        string                         `json:"marker"`
	Workspace     ExecutionWorkspaceRequest      `json:"workspace"`
	Probe         ExecutionOrcaProbeRequest      `json:"probe"`
	Prepared      *ExecutionOrcaWorkspaceReceipt `json:"prepared,omitempty"`
	Launch        *ExecutionOrcaLaunchRequest    `json:"launch,omitempty"`
	TerminalPTYID string                         `json:"terminal_pty_id,omitempty"`
	// TerminalHandle is a transient observation only. Adapters must re-resolve
	// the current handle from Prepared.WorktreeID + TerminalPTYID and must not
	// use this value as authority. The core never persists it.
	TerminalHandle string `json:"terminal_handle,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
}

type ExecutionOrcaIntentReceipt struct {
	Workspace      *ExecutionOrcaWorkspaceReceipt `json:"workspace,omitempty"`
	TerminalPTYID  string                         `json:"terminal_pty_id,omitempty"`
	TerminalHandle string                         `json:"terminal_handle,omitempty"`
	TaskID         string                         `json:"task_id,omitempty"`
	DispatchID     string                         `json:"dispatch_id,omitempty"`
}

type ExecutionOrcaIntentInventory struct {
	Candidates        []ExecutionOrcaIntentReceipt `json:"candidates"`
	AuthoritativeZero bool                         `json:"authoritative_zero,omitempty"`
}

type ExecutionOrcaProvisioner interface {
	Probe(context.Context, ExecutionOrcaProbeRequest) (ExecutionOrcaProbeResult, error)
	InspectIntent(context.Context, ExecutionOrcaIntentRequest) (ExecutionOrcaIntentInventory, error)
	InvokeIntent(context.Context, ExecutionOrcaIntentRequest) (ExecutionOrcaIntentReceipt, error)
}

type ExecutionOrcaOwnerInventoryRequest struct {
	RuntimeID            string `json:"runtime_id"`
	WorktreeID           string `json:"worktree_id"`
	TaskID               string `json:"task_id"`
	DispatchID           string `json:"dispatch_id"`
	TerminalPTYID        string `json:"terminal_pty_id,omitempty"`
	AllowRuntimeRollover bool   `json:"allow_runtime_rollover,omitempty"`
}

type ExecutionOrcaOwnerInventory struct {
	RuntimeID      string `json:"runtime_id,omitempty"`
	TerminalLive   bool   `json:"terminal_live"`
	TaskLive       bool   `json:"task_live"`
	TerminalID     string `json:"terminal_id,omitempty"`
	TaskStatus     string `json:"task_status,omitempty"`
	DispatchStatus string `json:"dispatch_status,omitempty"`
}

type ExecutionOrcaOwnerInspector interface {
	InspectOwner(context.Context, ExecutionOrcaOwnerInventoryRequest) (ExecutionOrcaOwnerInventory, error)
}
