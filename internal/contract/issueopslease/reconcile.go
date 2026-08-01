package issueopslease

type ReconcileWorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	Exists         bool   `json:"exists,omitempty"`
}

type ReconcilePreparedReceipt struct {
	Workspace          ReconcileWorkspaceReceipt `json:"workspace"`
	RuntimeID          string                    `json:"runtime_id"`
	RepoID             string                    `json:"repo_id"`
	WorktreeID         string                    `json:"worktree_id"`
	WorktreeInstanceID string                    `json:"worktree_instance_id,omitempty"`
}

type ReconcileStageReceipt struct {
	Workspace      *ReconcilePreparedReceipt `json:"workspace,omitempty"`
	TerminalPTYID  string                    `json:"terminal_pty_id,omitempty"`
	TerminalHandle string                    `json:"terminal_handle,omitempty"`
	RunID          string                    `json:"run_id,omitempty"`
	RunBound       bool                      `json:"run_bound,omitempty"`
	TaskID         string                    `json:"task_id,omitempty"`
	DispatchID     string                    `json:"dispatch_id,omitempty"`
}

type ReconcileStageInventory struct {
	Candidates        []ReconcileStageReceipt `json:"candidates"`
	AuthoritativeZero bool                    `json:"authoritative_zero"`
}
