package lifecycle

type NativeProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

type DocUpkeepEvent struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	TargetDocs []string `json:"target_docs"`
	Summary    string   `json:"summary"`
	Evidence   []string `json:"evidence,omitempty"`
	Source     string   `json:"source,omitempty"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
}

type HookToolUseLifecycleRequest struct {
	Repo                 string         `json:"repo,omitempty"`
	CWD                  string         `json:"cwd,omitempty"`
	Host                 string         `json:"host,omitempty"`
	SessionID            string         `json:"session_id,omitempty"`
	AgentID              string         `json:"agent_id,omitempty"`
	Tool                 string         `json:"tool,omitempty"`
	ToolInput            map[string]any `json:"tool_input,omitempty"`
	Paths                []string       `json:"paths,omitempty"`
	Command              string         `json:"command,omitempty"`
	Source               string         `json:"source,omitempty"`
	EnforceWorktree      bool           `json:"enforce_worktree,omitempty"`
	EnforceKoreanRemote  bool           `json:"enforce_korean_remote,omitempty"`
	EnforceVCSLinking    bool           `json:"enforce_vcs_linking,omitempty"`
	EnforceGitOpsKubectl bool           `json:"enforce_gitops_kubectl,omitempty"`
	EnforceStagedChecks  bool           `json:"enforce_staged_checks,omitempty"`
	ExpectedWorktree     string         `json:"expected_worktree,omitempty"`
	SourceCheckout       string         `json:"source_checkout,omitempty"`
	ProjectPath          string         `json:"project_path,omitempty"`
	// NativeProcessAncestry is collected locally by the hook process. It is not
	// accepted from hook JSON because payload-supplied identity is not authority.
	NativeProcessAncestry []NativeProcessReceipt `json:"-"`
}

type HookToolUseLifecycleResult struct {
	OK       bool           `json:"ok"`
	Recorded bool           `json:"recorded"`
	Event    DocUpkeepEvent `json:"event,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}

type IssueOpsDenyReason struct {
	Code              string `json:"code"`
	LifecycleID       string `json:"lifecycle_id"`
	ExpectedRoot      string `json:"expected_root"`
	CurrentGeneration uint64 `json:"current_generation"`
	NextCommand       string `json:"next_command"`
	IdentityMismatch  string `json:"identity_mismatch,omitempty"`
	ObservedActor     string `json:"observed_actor,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

type HookPreToolUseDecisionResult struct {
	OK       bool                `json:"ok"`
	Decision string              `json:"decision"`
	Reason   string              `json:"reason,omitempty"`
	Deny     *IssueOpsDenyReason `json:"deny,omitempty"`
	Tool     string              `json:"tool,omitempty"`
	Paths    []string            `json:"paths,omitempty"`
	Command  string              `json:"command,omitempty"`
	Source   string              `json:"source,omitempty"`
}

type StopNextActionRelayCandidate struct {
	Index       int    `json:"index"`
	Recommended bool   `json:"recommended,omitempty"`
	Text        string `json:"text"`
}

type StopNextActionRelayRecord struct {
	SchemaVersion    int                            `json:"schema_version"`
	Fingerprint      string                         `json:"fingerprint"`
	RecommendedIndex int                            `json:"recommended_index,omitempty"`
	RecommendedText  string                         `json:"recommended_text,omitempty"`
	Candidates       []StopNextActionRelayCandidate `json:"candidates,omitempty"`
	UpdatedAt        string                         `json:"updated_at"`
}

type StopNextActionRelayResult struct {
	OK          bool     `json:"ok"`
	ShouldRelay bool     `json:"should_relay"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Path        string   `json:"path,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}
