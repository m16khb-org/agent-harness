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
	// Resolution records the outcome of the feedback item, distinct from the
	// intake Classification (e.g. valid-defect | question-answered | noise-dismissed).
	Resolution string `json:"resolution,omitempty"`
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
	// TargetBranch is the PR/MR target branch, compared to BranchPrepare.BaseBranch
	// for the target_branch_match check.
	TargetBranch string `json:"target_branch,omitempty"`
}

type IssueOpsRemoteArtifactVerificationRequest struct {
	Provider     string
	Kind         string
	URL          string
	Labels       []string
	Assignees    []string
	TargetBranch string
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
	CodebaseSurvey IssueOpsPlanPrepItem `json:"codebase_survey"`
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
	CodebaseSurvey IssueOpsPlanPrepItemRequest
}

type IssueOpsCompatibilityReview struct {
	BackwardCompatibility []string `json:"backward_compatibility"`
	SideEffects           []string `json:"side_effects"`
	RollbackPlan          string   `json:"rollback_plan"`
	Verification          []string `json:"verification"`
	Blockers              []string `json:"blockers,omitempty"`
	Approved              bool     `json:"approved"`
	ReviewedAt            string   `json:"reviewed_at"`
}

type IssueOpsCompatibilityReviewRequest struct {
	BackwardCompatibility []string
	SideEffects           []string
	RollbackPlan          string
	Verification          []string
	Blockers              []string
	Approved              bool
}

// IssueOpsDevilsAdvocateReview captures the brooks devil's-advocate verdict on
// the completed plan/design. A pass (or a stop/revise explicitly waived with
// rationale) is a fail-closed precondition of implement entry; a stop's findings
// are reflected into the remote issue before the cycle regresses.
type IssueOpsDevilsAdvocateReview struct {
	Verdict          string   `json:"verdict"` // pass | revise | stop
	Findings         []string `json:"findings,omitempty"`
	Waived           bool     `json:"waived,omitempty"`
	WaiverRationale  string   `json:"waiver_rationale,omitempty"`
	ReviewerPattern  string   `json:"reviewer_pattern,omitempty"`
	RecordedAt       string   `json:"recorded_at"`
	IssueReflectedAt string   `json:"issue_reflected_at,omitempty"`
}

type IssueOpsDevilsAdvocateReviewRequest struct {
	Verdict         string
	Findings        []string
	Waived          bool
	WaiverRationale string
}

// IssueOpsDomainReview captures the grill-phase domain grilling outcome:
// terminology, current model fit, risks, and unresolved uncertainties. It is a
// net-new source-of-truth field; grilling produced no record state before.
type IssueOpsDomainReview struct {
	Terminology       []string `json:"terminology,omitempty"`
	ModelFit          string   `json:"model_fit,omitempty"`
	Risks             []string `json:"risks,omitempty"`
	OpenUncertainties []string `json:"open_uncertainties,omitempty"`
	ReviewedAt        string   `json:"reviewed_at"`
}

// IssueOpsPhaseLedgerEntry records that a phase was entered and (optionally)
// completed, plus which artifacts satisfied it. It is an index over existing
// source-of-truth fields, not their replacement. The owning map's key is the
// authoritative phase identity; Phase is a self-describing copy that must equal
// its key.
type IssueOpsDomainReviewRequest struct {
	Terminology       []string
	ModelFit          string
	Risks             []string
	OpenUncertainties []string
}

type IssueOpsPhaseLedgerEntry struct {
	Phase       IssueOpsPhase `json:"phase"`
	EnteredAt   string        `json:"entered_at,omitempty"`
	CompletedAt string        `json:"completed_at,omitempty"`
	Artifacts   []string      `json:"artifacts,omitempty"`
	Missing     []string      `json:"missing,omitempty"`
	Notes       []string      `json:"notes,omitempty"`
}

// IssueOpsPhaseLedger indexes phase completion. Iterate in IssueOpsPhases order
// (never Go map order) when rendering or comparing for determinism.
type IssueOpsPhaseLedger map[IssueOpsPhase]IssueOpsPhaseLedgerEntry

// IssueOpsRegressEvent is the audit trail of one Brooks regression (stop →
// reflect → regress). Its count backs the regress cap: repeated stop/regress
// rounds on one cycle stop consuming tokens and escalate to a human decision.
type IssueOpsRegressEvent struct {
	Reason    string        `json:"reason"`
	FromPhase IssueOpsPhase `json:"from_phase"`
	At        string        `json:"at"`
}

type IssueOpsDelegationContract struct {
	ParentCycleID      string   `json:"parent_cycle_id"`
	TaskScope          string   `json:"task_scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ParentPlanPath     string   `json:"parent_plan_path,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	DelegatedAt        string   `json:"delegated_at"`
}

type IssueOpsChildCycleRef struct {
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

type IssueOpsChildStartRequest struct {
	ParentID           string
	Branch             string
	Title              string
	TaskScope          string
	AcceptanceCriteria []string
	ParentPlanPath     string
	ChildIssueURL      string
}

type IssueOpsChildStartResult struct {
	OK               bool                  `json:"ok"`
	ParentID         string                `json:"parent_id"`
	Child            IssueOpsRecord        `json:"child"`
	ParentRef        IssueOpsChildCycleRef `json:"parent_ref"`
	Guidance         string                `json:"guidance,omitempty"`
	ChildLinkWarning string                `json:"child_link_warning,omitempty"`
}

type IssueOpsChildStatusEntry struct {
	CycleID            string        `json:"cycle_id"`
	Branch             string        `json:"branch,omitempty"`
	Title              string        `json:"title,omitempty"`
	ChildIssueURL      string        `json:"child_issue_url,omitempty"`
	Phase              IssueOpsPhase `json:"phase,omitempty"`
	LastActiveAt       string        `json:"last_active_at,omitempty"`
	WorktreePath       string        `json:"worktree_path,omitempty"`
	ValidationVerdict  string        `json:"validation_verdict,omitempty"`
	ValidationReason   string        `json:"validation_reason,omitempty"`
	ValidationEvidence []string      `json:"validation_evidence,omitempty"`
	ValidatedAt        string        `json:"validated_at,omitempty"`
	ParentClosedState  string        `json:"parent_closed_state,omitempty"`
	Indexed            bool          `json:"indexed"`
	Scanned            bool          `json:"scanned"`
	Orphaned           bool          `json:"orphaned,omitempty"`
}

type IssueOpsChildStatusResult struct {
	OK             bool                       `json:"ok"`
	ParentID       string                     `json:"parent_id"`
	Children       []IssueOpsChildStatusEntry `json:"children"`
	Repaired       bool                       `json:"repaired,omitempty"`
	RepairAppended []string                   `json:"repair_appended,omitempty"`
	Orphaned       []string                   `json:"orphaned,omitempty"`
}

type IssueOpsChildValidationResult struct {
	OK        bool                  `json:"ok"`
	ParentID  string                `json:"parent_id"`
	ChildID   string                `json:"child_id"`
	ParentRef IssueOpsChildCycleRef `json:"parent_ref"`
}

const IssueOpsCurrentSchemaVersion = IssueOpsSchemaVersion

type IssueOpsRecord struct {
	OK                      bool                                `json:"ok"`
	Invalid                 bool                                `json:"-"`
	InvalidReason           string                              `json:"-"`
	SchemaVersion           int                                 `json:"schema_version"`
	ID                      string                              `json:"id"`
	Repo                    string                              `json:"repo"`
	Branch                  string                              `json:"branch,omitempty"`
	Phase                   IssueOpsPhase                       `json:"phase"`
	Intent                  *IssueOpsIntentContract             `json:"intent,omitempty"`
	DesignReview            *IssueOpsDesignReview               `json:"design_review,omitempty"`
	DomainReview            *IssueOpsDomainReview               `json:"domain_review,omitempty"`
	IssueURL                string                              `json:"issue_url,omitempty"`
	PlanPath                string                              `json:"plan_path,omitempty"`
	WorktreePath            string                              `json:"worktree_path,omitempty"`
	IssueLinks              []IssueOpsIssueLink                 `json:"issue_links,omitempty"`
	BranchPrepare           *IssueOpsBranchPrepare              `json:"branch_prepare,omitempty"`
	RemoteArtifact          *IssueOpsRemoteArtifactVerification `json:"remote_artifact,omitempty"`
	Decisions               []IssueOpsDecision                  `json:"decisions,omitempty"`
	PlanPrep                *IssueOpsPlanPrep                   `json:"plan_prep,omitempty"`
	CompatibilityReview     *IssueOpsCompatibilityReview        `json:"compatibility_review,omitempty"`
	DevilsAdvocateReview    *IssueOpsDevilsAdvocateReview       `json:"devils_advocate_review,omitempty"`
	Feedback                []IssueOpsFeedbackItem              `json:"feedback,omitempty"`
	RegressEvents           []IssueOpsRegressEvent              `json:"regress_events,omitempty"`
	Delegation              *IssueOpsDelegationContract         `json:"delegation,omitempty"`
	ChildCycles             []IssueOpsChildCycleRef             `json:"child_cycles,omitempty"`
	Execution               *Execution                          `json:"execution,omitempty"`
	RoutingTrace            []SkillRoutingEntry                 `json:"routing_trace,omitempty"`
	AISlopCleanAt           string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   []string                            `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification []string                            `json:"ai_slop_clean_verification,omitempty"`
	PhaseLedger             IssueOpsPhaseLedger                 `json:"phase_ledger,omitempty"`
	CreatedAt               string                              `json:"created_at"`
	UpdatedAt               string                              `json:"updated_at"`
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
