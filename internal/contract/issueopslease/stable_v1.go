package issueopslease

// stableV1Record은 core 모델에 의존하지 않고 persisted v1 JSON의 canonical
// shape를 재현한다. release 전이에서는 lease만 교체하고 나머지 sidecar를 보존한다.
type stableV1Record struct {
	OK                      bool                           `json:"ok"`
	SchemaVersion           int                            `json:"schema_version"`
	ID                      string                         `json:"id"`
	Repo                    string                         `json:"repo"`
	Branch                  string                         `json:"branch,omitempty"`
	Phase                   string                         `json:"phase"`
	Intent                  *stableV1Intent                `json:"intent,omitempty"`
	DesignReview            *stableV1DesignReview          `json:"design_review,omitempty"`
	DomainReview            *stableV1DomainReview          `json:"domain_review,omitempty"`
	IssueURL                string                         `json:"issue_url,omitempty"`
	PlanPath                string                         `json:"plan_path,omitempty"`
	WorktreePath            string                         `json:"worktree_path,omitempty"`
	IssueLinks              []stableV1IssueLink            `json:"issue_links,omitempty"`
	BranchPrepare           *stableV1BranchPrepare         `json:"branch_prepare,omitempty"`
	RemoteArtifact          *stableV1RemoteArtifact        `json:"remote_artifact,omitempty"`
	Decisions               []stableV1Decision             `json:"decisions,omitempty"`
	PlanPrep                *stableV1PlanPrep              `json:"plan_prep,omitempty"`
	CompatibilityReview     *stableV1CompatibilityReview   `json:"compatibility_review,omitempty"`
	DevilsAdvocateReview    *stableV1DevilsAdvocateReview  `json:"devils_advocate_review,omitempty"`
	Feedback                []stableV1Feedback             `json:"feedback,omitempty"`
	RegressEvents           []stableV1RegressEvent         `json:"regress_events,omitempty"`
	Delegation              *stableV1Delegation            `json:"delegation,omitempty"`
	ChildCycles             []stableV1ChildCycle           `json:"child_cycles,omitempty"`
	Execution               *stableV1Execution             `json:"execution,omitempty"`
	RemoteCompletion        *stableV1RemoteCompletion      `json:"remote_completion,omitempty"`
	SourceMisdirectWarnings int                            `json:"source_misdirect_warnings,omitempty"`
	CleanupFinishFailure    *stableV1CleanupFinishFailure  `json:"cleanup_finish_failure,omitempty"`
	ImplementationReview    *stableV1ImplementationReview  `json:"implementation_review,omitempty"`
	RoutingTrace            []stableV1SkillRouting         `json:"routing_trace,omitempty"`
	AISlopCleanAt           string                         `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         string                         `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  string                         `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   []string                       `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification []string                       `json:"ai_slop_clean_verification,omitempty"`
	PhaseLedger             map[string]stableV1LedgerEntry `json:"phase_ledger,omitempty"`
	CreatedAt               string                         `json:"created_at"`
	UpdatedAt               string                         `json:"updated_at"`
}

type stableV1Feedback struct {
	Source         string `json:"source"`
	Body           string `json:"body"`
	Classification string `json:"classification,omitempty"`
	CreatedAt      string `json:"created_at"`
	IssueUpdatedAt string `json:"issue_updated_at,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
}
type stableV1IssueLink struct {
	Type            string `json:"type"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	Provider        string `json:"provider,omitempty"`
	CreatedAt       string `json:"created_at"`
	ClosedAt        string `json:"closed_at,omitempty"`
	CloseVerifiedAt string `json:"close_verified_at,omitempty"`
	CloseReason     string `json:"close_reason,omitempty"`
}
type stableV1BranchPrepareStep struct {
	Order         int            `json:"order"`
	Strategy      string         `json:"strategy"`
	Tool          string         `json:"tool,omitempty"`
	ToolArguments map[string]any `json:"tool_arguments,omitempty"`
	Command       []string       `json:"command,omitempty"`
	Description   string         `json:"description"`
}
type stableV1BranchPrepare struct {
	Provider        string                      `json:"provider"`
	IssueURL        string                      `json:"issue_url"`
	Branch          string                      `json:"branch"`
	BaseBranch      string                      `json:"base_branch"`
	BaseSHA         string                      `json:"base_sha,omitempty"`
	ParentWorktree  string                      `json:"parent_worktree,omitempty"`
	RemoteBranchURL string                      `json:"remote_branch_url,omitempty"`
	LinkVerified    bool                        `json:"link_verified"`
	Steps           []stableV1BranchPrepareStep `json:"steps"`
	CreatedAt       string                      `json:"created_at"`
}
type stableV1RemoteArtifact struct {
	Provider     string   `json:"provider"`
	Kind         string   `json:"kind"`
	URL          string   `json:"url"`
	Labels       []string `json:"labels"`
	Assignees    []string `json:"assignees"`
	VerifiedAt   string   `json:"verified_at"`
	TargetBranch string   `json:"target_branch,omitempty"`
}
type stableV1Intent struct {
	RawRequest        string   `json:"raw_request"`
	InterpretedIntent string   `json:"interpreted_intent"`
	SuccessCriteria   []string `json:"success_criteria"`
	Constraints       []string `json:"constraints,omitempty"`
	Ambiguities       []string `json:"ambiguities,omitempty"`
	NonGoals          []string `json:"non_goals,omitempty"`
	IntentClass       string   `json:"intent_class,omitempty"`
	RecordedAt        string   `json:"recorded_at"`
}
type stableV1DesignReview struct {
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
type stableV1Decision struct {
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Kind               string   `json:"kind"`
	Rationale          string   `json:"rationale,omitempty"`
	Alternatives       []string `json:"alternatives,omitempty"`
	AffectedIssueLinks []string `json:"affected_issue_links,omitempty"`
	AffectedArtifacts  []string `json:"affected_artifacts,omitempty"`
	CreatedAt          string   `json:"created_at"`
}
type stableV1PlanPrepItem struct {
	Status      string   `json:"status"`
	Evidence    []string `json:"evidence,omitempty"`
	WaiveReason string   `json:"waive_reason,omitempty"`
}
type stableV1PlanPrep struct {
	PriorDecisions stableV1PlanPrepItem `json:"prior_decisions"`
	RelatedIssues  stableV1PlanPrepItem `json:"related_issues"`
	WebResearch    stableV1PlanPrepItem `json:"web_research"`
	CodebaseSurvey stableV1PlanPrepItem `json:"codebase_survey"`
	RecordedAt     string               `json:"recorded_at"`
}
type stableV1CompatibilityReview struct {
	BackwardCompatibility []string `json:"backward_compatibility"`
	SideEffects           []string `json:"side_effects"`
	RollbackPlan          string   `json:"rollback_plan"`
	Verification          []string `json:"verification"`
	Blockers              []string `json:"blockers,omitempty"`
	Approved              bool     `json:"approved"`
	ReviewedAt            string   `json:"reviewed_at"`
}
type stableV1DevilsAdvocateReview struct {
	Verdict          string   `json:"verdict"`
	Findings         []string `json:"findings,omitempty"`
	Waived           bool     `json:"waived,omitempty"`
	WaiverRationale  string   `json:"waiver_rationale,omitempty"`
	ReviewerPattern  string   `json:"reviewer_pattern,omitempty"`
	RecordedAt       string   `json:"recorded_at"`
	IssueReflectedAt string   `json:"issue_reflected_at,omitempty"`
}
type stableV1DomainReview struct {
	Terminology       []string `json:"terminology,omitempty"`
	ModelFit          string   `json:"model_fit,omitempty"`
	Risks             []string `json:"risks,omitempty"`
	OpenUncertainties []string `json:"open_uncertainties,omitempty"`
	ReviewedAt        string   `json:"reviewed_at"`
}
type stableV1LedgerEntry struct {
	Phase       string   `json:"phase"`
	EnteredAt   string   `json:"entered_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}
type stableV1RegressEvent struct {
	Reason    string `json:"reason"`
	FromPhase string `json:"from_phase"`
	At        string `json:"at"`
}
type stableV1Delegation struct {
	ParentCycleID      string   `json:"parent_cycle_id"`
	TaskScope          string   `json:"task_scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ParentPlanPath     string   `json:"parent_plan_path,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	DelegatedAt        string   `json:"delegated_at"`
}
type stableV1ChildCycle struct {
	CycleID            string   `json:"cycle_id"`
	Branch             string   `json:"branch"`
	Title              string   `json:"title,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	CreatedAt          string   `json:"created_at"`
	ValidationVerdict  string   `json:"validation_verdict,omitempty"`
	ValidationReason   string   `json:"validation_reason,omitempty"`
	ValidationEvidence []string `json:"validation_evidence,omitempty"`
	ValidatedAt        string   `json:"validated_at,omitempty"`
}
type stableV1Execution struct {
	Mode              string                      `json:"mode"`
	Workspace         stableV1Workspace           `json:"workspace"`
	Lease             stableV1Lease               `json:"lease"`
	Orca              *stableV1OrcaBinding        `json:"orca,omitempty"`
	Pending           *stableV1ExternalIntent     `json:"pending,omitempty"`
	Completion        *stableV1Completion         `json:"completion,omitempty"`
	CompletionHistory []stableV1CompletionHistory `json:"completion_history,omitempty"`
	Failure           *stableV1Failure            `json:"failure,omitempty"`
	SyncBaseEvents    []stableV1SyncBaseEvent     `json:"sync_base_events,omitempty"`
}
type stableV1Workspace struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	LinkedAt       string `json:"linked_at"`
}
type stableV1Lease struct {
	Generation        uint64               `json:"generation"`
	Status            string               `json:"status"`
	Holder            *stableV1NativeActor `json:"holder,omitempty"`
	ClaimTokenSHA256  string               `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string               `json:"claimed_at,omitempty"`
	ReleasedAt        string               `json:"released_at,omitempty"`
	ReplacedAt        string               `json:"replaced_at,omitempty"`
	ReplacementReason string               `json:"replacement_reason,omitempty"`
}
type stableV1NativeActor struct {
	Host           string                  `json:"host"`
	SessionID      string                  `json:"session_id"`
	AgentID        string                  `json:"agent_id,omitempty"`
	SessionProcess *stableV1ProcessReceipt `json:"session_process,omitempty"`
}
type stableV1ProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}
type stableV1OrcaBinding struct {
	RuntimeID               string `json:"runtime_id"`
	RepoID                  string `json:"repo_id"`
	WorktreeID              string `json:"worktree_id"`
	RunID                   string `json:"run_id,omitempty"`
	WorktreeInstanceID      string `json:"worktree_instance_id,omitempty"`
	LeaseGeneration         uint64 `json:"lease_generation,omitempty"`
	ArtifactIdentityVersion uint64 `json:"artifact_identity_version,omitempty"`
	IssueBodySHA256         string `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256     string `json:"context_packet_sha256,omitempty"`
	OwnerPromptSHA256       string `json:"owner_prompt_sha256,omitempty"`
	OwnerHost               string `json:"owner_host"`
	OwnerModel              string `json:"owner_model"`
	OwnerEffort             string `json:"owner_effort,omitempty"`
	TaskID                  string `json:"task_id"`
	DispatchID              string `json:"dispatch_id"`
	TerminalPTYID           string `json:"terminal_pty_id,omitempty"`
}
type stableV1ExternalIntent struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}
type stableV1Completion struct {
	Generation        uint64   `json:"generation,omitempty"`
	FinalHead         string   `json:"final_head"`
	TuringReportPath  string   `json:"turing_report_path"`
	Verification      []string `json:"verification"`
	RemoteArtifactURL string   `json:"remote_artifact_url"`
	CompletedAt       string   `json:"completed_at"`
}
type stableV1CompletionHistory struct {
	Generation uint64             `json:"generation"`
	Completion stableV1Completion `json:"completion"`
	Reason     string             `json:"reason"`
	ReopenedAt string             `json:"reopened_at"`
}
type stableV1Failure struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}
type stableV1SyncBaseEvent struct {
	Mode          string `json:"mode"`
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}
type stableV1RemoteCompletion struct {
	ReflectedAt   string `json:"reflected_at,omitempty"`
	IssueClosedAt string `json:"issue_closed_at,omitempty"`
}
type stableV1CleanupFinishFailure struct {
	Step    string `json:"step"`
	Message string `json:"message"`
	At      string `json:"at"`
}
type stableV1ImplementationReview struct {
	Verdict             string   `json:"verdict"`
	Findings            []string `json:"findings"`
	Evidence            []string `json:"evidence"`
	ReviewedFingerprint string   `json:"reviewed_fingerprint"`
	ReviewerHost        string   `json:"reviewer_host,omitempty"`
	ReviewerModel       string   `json:"reviewer_model,omitempty"`
	ReviewerEffort      string   `json:"reviewer_effort,omitempty"`
	RecordedAt          string   `json:"recorded_at"`
}
type stableV1SkillRouting struct {
	Phase string `json:"phase"`
	Skill string `json:"skill"`
	At    string `json:"at,omitempty"`
}
