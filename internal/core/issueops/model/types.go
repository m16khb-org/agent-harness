package model

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

type IssueOpsIntentContract struct {
	RawRequest        string   `json:"raw_request"`
	InterpretedIntent string   `json:"interpreted_intent"`
	SuccessCriteria   []string `json:"success_criteria"`
	Constraints       []string `json:"constraints,omitempty"`
	Ambiguities       []string `json:"ambiguities,omitempty"`
	NonGoals          []string `json:"non_goals,omitempty"`
	RecordedAt        string   `json:"recorded_at"`
}

type IssueOpsIntentRecordRequest struct {
	RawRequest        string
	InterpretedIntent string
	SuccessCriteria   []string
	Constraints       []string
	Ambiguities       []string
	NonGoals          []string
}

type IssueOpsDesignReview struct {
	ProblemSummary string   `json:"problem_summary"`
	ProposedDesign string   `json:"proposed_design"`
	RefactorPlan   string   `json:"refactor_plan,omitempty"`
	Alternatives   []string `json:"alternatives,omitempty"`
	Risks          []string `json:"risks,omitempty"`
	Verification   []string `json:"verification"`
	OpenQuestions  []string `json:"open_questions,omitempty"`
	Approved       bool     `json:"approved"`
	ReviewedAt     string   `json:"reviewed_at"`
}

type IssueOpsDesignReviewRequest struct {
	ProblemSummary string
	ProposedDesign string
	RefactorPlan   string
	Alternatives   []string
	Risks          []string
	Verification   []string
	OpenQuestions  []string
	Approved       bool
}

type IssueOpsDecision struct {
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Kind               string   `json:"kind"`
	Rationale          string   `json:"rationale,omitempty"`
	Alternatives       []string `json:"alternatives,omitempty"`
	AffectedIssueLinks []string `json:"affected_issue_links,omitempty"`
	AffectedArtifacts  []string `json:"affected_artifacts,omitempty"`
	CreatedAt          string   `json:"created_at"`
}

type IssueOpsDecisionRecordRequest struct {
	Title              string
	Body               string
	Kind               string
	Rationale          string
	Alternatives       []string
	AffectedIssueLinks []string
	AffectedArtifacts  []string
}

type IssueOpsRecord struct {
	OK                     bool                                `json:"ok"`
	ID                     string                              `json:"id"`
	Repo                   string                              `json:"repo"`
	Branch                 string                              `json:"branch,omitempty"`
	Phase                  IssueOpsPhase                       `json:"phase"`
	Intent                 *IssueOpsIntentContract             `json:"intent,omitempty"`
	DesignReview           *IssueOpsDesignReview               `json:"design_review,omitempty"`
	IssueURL               string                              `json:"issue_url,omitempty"`
	PlanPath               string                              `json:"plan_path,omitempty"`
	WorktreePath           string                              `json:"worktree_path,omitempty"`
	IssueLinks             []IssueOpsIssueLink                 `json:"issue_links,omitempty"`
	BranchPrepare          *IssueOpsBranchPrepare              `json:"branch_prepare,omitempty"`
	RemoteArtifact         *IssueOpsRemoteArtifactVerification `json:"remote_artifact,omitempty"`
	Decisions              []IssueOpsDecision                  `json:"decisions,omitempty"`
	Feedback               []IssueOpsFeedbackItem              `json:"feedback,omitempty"`
	AISlopCleanAt          string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead        string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	ForceReleasedAt        string                              `json:"force_released_at,omitempty"`
	ForceReleaseReason     string                              `json:"force_release_reason,omitempty"`
	StaleResetAt           string                              `json:"stale_reset_at,omitempty"`
	StaleResetPriorPhase   string                              `json:"stale_reset_prior_phase,omitempty"`
	OrphanWorktreePath     string                              `json:"orphan_worktree_path,omitempty"`
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
	CleanupReady           bool     `json:"cleanup_ready,omitempty"`
	CleanupMissing         []string `json:"cleanup_missing,omitempty"`
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
