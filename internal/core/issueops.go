package core

type IssueOpsStartRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
}

type IssueOpsFeedbackItem struct {
	Source         string `json:"source"`
	Body           string `json:"body"`
	Classification string `json:"classification,omitempty"`
	CreatedAt      string `json:"created_at"`
	IssueUpdatedAt string `json:"issue_updated_at,omitempty"`
}

type IssueOpsIssueLink struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Provider  string `json:"provider,omitempty"`
	CreatedAt string `json:"created_at"`
}

type IssueOpsBranchPrepareStep struct {
	Order         int            `json:"order"`
	Strategy      string         `json:"strategy"`
	Tool          string         `json:"tool,omitempty"`
	ToolArguments map[string]any `json:"tool_arguments,omitempty"`
	Command       []string       `json:"command,omitempty"`
	Description   string         `json:"description"`
}

type IssueOpsBranchPrepare struct {
	Provider        string                      `json:"provider"`
	IssueURL        string                      `json:"issue_url"`
	Branch          string                      `json:"branch"`
	BaseBranch      string                      `json:"base_branch"`
	BaseSHA         string                      `json:"base_sha,omitempty"`
	RemoteBranchURL string                      `json:"remote_branch_url,omitempty"`
	LinkVerified    bool                        `json:"link_verified"`
	Steps           []IssueOpsBranchPrepareStep `json:"steps"`
	CreatedAt       string                      `json:"created_at"`
}

type IssueOpsBranchPrepareRequest struct {
	Provider        string `json:"provider"`
	IssueURL        string `json:"issue_url"`
	Branch          string `json:"branch"`
	BaseBranch      string `json:"base_branch"`
	BaseSHA         string `json:"base_sha,omitempty"`
	RemoteBranchURL string `json:"remote_branch_url,omitempty"`
	LinkVerified    bool   `json:"link_verified,omitempty"`
}

type IssueOpsRemoteArtifactVerification struct {
	Provider   string   `json:"provider"`
	Kind       string   `json:"kind"`
	URL        string   `json:"url"`
	Labels     []string `json:"labels"`
	Assignees  []string `json:"assignees"`
	VerifiedAt string   `json:"verified_at"`
}

type IssueOpsRemoteArtifactVerificationRequest struct {
	Provider  string
	Kind      string
	URL       string
	Labels    []string
	Assignees []string
}

type IssueOpsRecord struct {
	OK                     bool                                `json:"ok"`
	ID                     string                              `json:"id"`
	Repo                   string                              `json:"repo"`
	Branch                 string                              `json:"branch,omitempty"`
	Phase                  IssueOpsPhase                       `json:"phase"`
	IssueURL               string                              `json:"issue_url,omitempty"`
	PlanPath               string                              `json:"plan_path,omitempty"`
	WorktreePath           string                              `json:"worktree_path,omitempty"`
	IssueLinks             []IssueOpsIssueLink                 `json:"issue_links,omitempty"`
	BranchPrepare          *IssueOpsBranchPrepare              `json:"branch_prepare,omitempty"`
	RemoteArtifact         *IssueOpsRemoteArtifactVerification `json:"remote_artifact,omitempty"`
	Feedback               []IssueOpsFeedbackItem              `json:"feedback,omitempty"`
	AISlopCleanAt          string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead        string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	CreatedAt              string                              `json:"created_at"`
	UpdatedAt              string                              `json:"updated_at"`
}

type IssueOpsReadiness struct {
	OK                     bool     `json:"ok"`
	Ready                  bool     `json:"ready"`
	Strict                 bool     `json:"strict,omitempty"`
	Missing                []string `json:"missing"`
	IssueURL               string   `json:"issue_url,omitempty"`
	PlanPath               string   `json:"plan_path,omitempty"`
	WorktreePath           string   `json:"worktree_path,omitempty"`
	Branch                 string   `json:"branch,omitempty"`
	AISlopCleanHead        string   `json:"ai_slop_clean_head,omitempty"`
	CurrentHead            string   `json:"current_head,omitempty"`
	AISlopCleanFingerprint string   `json:"ai_slop_clean_fingerprint,omitempty"`
	CurrentFingerprint     string   `json:"current_fingerprint,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type IssueOpsCleanupStatusRequest struct {
	Merged bool `json:"merged"`
}

type IssueOpsCleanupStatus struct {
	OK                bool     `json:"ok"`
	Ready             bool     `json:"ready"`
	ID                string   `json:"id"`
	Merged            bool     `json:"merged"`
	Missing           []string `json:"missing"`
	Warnings          []string `json:"warnings,omitempty"`
	Choices           []string `json:"choices"`
	WorktreePath      string   `json:"worktree_path,omitempty"`
	Branch            string   `json:"branch,omitempty"`
	RemoteArtifactURL string   `json:"remote_artifact_url,omitempty"`
}
