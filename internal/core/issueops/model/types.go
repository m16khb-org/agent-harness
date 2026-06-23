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
	Type            string `json:"type"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	Provider        string `json:"provider,omitempty"`
	CreatedAt       string `json:"created_at"`
	ClosedAt        string `json:"closed_at,omitempty"`
	CloseVerifiedAt string `json:"close_verified_at,omitempty"`
	CloseReason     string `json:"close_reason,omitempty"`
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
	IntentClass       string   `json:"intent_class,omitempty"`
	RecordedAt        string   `json:"recorded_at"`
}

type IssueOpsIntentRecordRequest struct {
	RawRequest        string
	InterpretedIntent string
	SuccessCriteria   []string
	Constraints       []string
	Ambiguities       []string
	NonGoals          []string
	IntentClass       string
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

type IssueOpsPlanPrepItem struct {
	Status      string   `json:"status"`
	Evidence    []string `json:"evidence,omitempty"`
	WaiveReason string   `json:"waive_reason,omitempty"`
}

type IssueOpsPlanPrep struct {
	PriorDecisions IssueOpsPlanPrepItem `json:"prior_decisions"`
	RelatedIssues  IssueOpsPlanPrepItem `json:"related_issues"`
	WebResearch    IssueOpsPlanPrepItem `json:"web_research"`
	RecordedAt     string               `json:"recorded_at"`
}

type IssueOpsPlanPrepItemRequest struct {
	Evidence    []string
	WaiveReason string
}

type IssueOpsPlanPrepRequest struct {
	PriorDecisions IssueOpsPlanPrepItemRequest
	RelatedIssues  IssueOpsPlanPrepItemRequest
	WebResearch    IssueOpsPlanPrepItemRequest
}

type IssueOpsWorktreeToolPreparation struct {
	OK                   bool     `json:"ok"`
	ID                   string   `json:"id"`
	WorktreePath         string   `json:"worktree_path"`
	PackageManager       string   `json:"package_manager,omitempty"`
	DependenciesChecked  bool     `json:"dependencies_checked,omitempty"`
	DependenciesReady    bool     `json:"dependencies_ready,omitempty"`
	DependenciesAction   string   `json:"dependencies_action,omitempty"`
	CodeGraphProjectPath string   `json:"codegraph_project_path"`
	CodeGraphChecked     bool     `json:"codegraph_checked"`
	CodeGraphInitialized bool     `json:"codegraph_initialized,omitempty"`
	CodeGraphReady       bool     `json:"codegraph_ready"`
	Messages             []string `json:"messages,omitempty"`
	PreparedAt           string   `json:"prepared_at,omitempty"`
}

type IssueOpsSubAgentPlan struct {
	Objective            string   `json:"objective"`
	Pattern              string   `json:"pattern"`
	Benefit              string   `json:"benefit"`
	Tradeoffs            []string `json:"tradeoffs"`
	NetPositiveRationale string   `json:"net_positive_rationale"`
	Scope                string   `json:"scope"`
	Verification         string   `json:"verification"`
	Fallback             string   `json:"fallback"`
}

type IssueOpsExecutionDecision struct {
	AutoProceed       []string               `json:"auto_proceed"`
	HookBlocked       []string               `json:"hook_blocked"`
	HumanGates        []string               `json:"human_gates"`
	SubagentUse       string                 `json:"subagent_use"`
	SubagentRationale string                 `json:"subagent_rationale,omitempty"`
	SubagentPlans     []IssueOpsSubAgentPlan `json:"subagent_plans,omitempty"`
	RecordedAt        string                 `json:"recorded_at"`
}

type IssueOpsExecutionDecisionRecordRequest struct {
	AutoProceed       []string
	HookBlocked       []string
	HumanGates        []string
	SubagentUse       string
	SubagentRationale string
	SubagentPlans     []IssueOpsSubAgentPlan
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
	PlanPrep               *IssueOpsPlanPrep                   `json:"plan_prep,omitempty"`
	WorktreeTools          *IssueOpsWorktreeToolPreparation    `json:"worktree_tools,omitempty"`
	ExecutionDecision      *IssueOpsExecutionDecision          `json:"execution_decision,omitempty"`
	Feedback               []IssueOpsFeedbackItem              `json:"feedback,omitempty"`
	RoutingTrace           []SkillRoutingEntry                 `json:"routing_trace,omitempty"`
	AISlopCleanAt          string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead        string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	ForceReleasedAt        string                              `json:"force_released_at,omitempty"`
	ForceReleaseReason     string                              `json:"force_release_reason,omitempty"`
	StaleResetAt           string                              `json:"stale_reset_at,omitempty"`
	StaleResetPriorPhase   string                              `json:"stale_reset_prior_phase,omitempty"`
	OrphanWorktreePath     string                              `json:"orphan_worktree_path,omitempty"`
	LastHeartbeatAt        string                              `json:"last_heartbeat_at,omitempty"`
	CreatedAt              string                              `json:"created_at"`
	UpdatedAt              string                              `json:"updated_at"`
}

// SkillRoutingEntry records that a pioneer/CS skill fired at a given IssueOps
// phase during a real run. Captured live, it lets skill_routing_fidelity be
// scored against observed activation instead of a synthesized trace.
type SkillRoutingEntry struct {
	Phase string `json:"phase"`
	Skill string `json:"skill"`
	At    string `json:"at,omitempty"`
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

type IssueOpsCloseChildrenRequest struct {
	Merged  bool `json:"merged"`
	Confirm bool `json:"confirm"`
}

type IssueOpsCloseChildResult struct {
	URL               string `json:"url"`
	Provider          string `json:"provider,omitempty"`
	Closed            bool   `json:"closed"`
	AlreadyClosed     bool   `json:"already_closed,omitempty"`
	HierarchyVerified bool   `json:"hierarchy_verified"`
	State             string `json:"state,omitempty"`
	Preview           string `json:"preview,omitempty"`
	Error             string `json:"error,omitempty"`
}

type IssueOpsCloseChildrenResult struct {
	OK          bool                       `json:"ok"`
	ID          string                     `json:"id"`
	Merged      bool                       `json:"merged"`
	Confirmed   bool                       `json:"confirmed"`
	DryRun      bool                       `json:"dry_run"`
	ClosedCount int                        `json:"closed_count"`
	Children    []IssueOpsCloseChildResult `json:"children"`
	Missing     []string                   `json:"missing,omitempty"`
}

type IssueOpsResumeResult struct {
	OK              bool               `json:"ok"`
	CycleID         string             `json:"cycle_id,omitempty"`
	Phase           IssueOpsPhase      `json:"phase,omitempty"`
	Repo            string             `json:"repo,omitempty"`
	Branch          string             `json:"branch,omitempty"`
	WorktreePath    string             `json:"worktree_path,omitempty"`
	IssueURL        string             `json:"issue_url,omitempty"`
	PlanPath        string             `json:"plan_path,omitempty"`
	Bound           bool               `json:"bound"`
	SuggestedCycles []string           `json:"suggested_cycles,omitempty"`
	Readiness       *IssueOpsReadiness `json:"readiness,omitempty"`
}
