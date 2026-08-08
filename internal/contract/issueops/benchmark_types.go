package issueops

type IssueOpsBenchmarkArtifact struct {
	ProblemSummary         string `json:"problem_summary,omitempty"`
	IssueDraft             string `json:"issue_draft,omitempty"`
	Plan                   string `json:"plan,omitempty"`
	TaskBreakdown          string `json:"task_breakdown,omitempty"`
	TDDPlan                string `json:"tdd_plan,omitempty"`
	SubagentPrompts        string `json:"subagent_prompts,omitempty"`
	ImplementationNotes    string `json:"implementation_notes,omitempty"`
	PRDraft                string `json:"pr_draft,omitempty"`
	PhaseChoices           string `json:"phase_choices,omitempty"`
	BranchName             string `json:"branch_name,omitempty"`
	WorktreePath           string `json:"worktree_path,omitempty"`
	ImplementationLocation string `json:"implementation_location,omitempty"`
	WorktreeCleanup        string `json:"worktree_cleanup,omitempty"`
	GuidelineRef           string `json:"guideline_ref,omitempty"`
	DomainContractEvidence string `json:"domain_contract_evidence,omitempty"`
	APIDocGateEvidence     string `json:"api_doc_gate_evidence,omitempty"`
	LiveEvidenceMatrix     string `json:"live_evidence_matrix,omitempty"`
	ReviewFeedbackEvidence string `json:"review_feedback_evidence,omitempty"`
	CompletionHygiene      string `json:"completion_hygiene,omitempty"`
	PioneerSkillEvidence   string `json:"pioneer_skill_evidence,omitempty"`
	// RoutingTrace records which skill fired at which phase during the run; the
	// skill_routing_fidelity dimension (A5) scores it against the fixture's
	// ExpectedRouting.
	RoutingTrace []SkillRouting `json:"routing_trace,omitempty"`
}

type IssueOpsBenchmarkFixture struct {
	ID                      string         `json:"id"`
	Title                   string         `json:"title"`
	UserPrompt              string         `json:"user_prompt"`
	RepoContext             string         `json:"repo_context"`
	PioneerSkillTarget      string         `json:"pioneer_skill_target,omitempty"`
	ExpectedRouting         []SkillRouting `json:"expected_routing,omitempty"`
	ExpectedIssue           []string       `json:"expected_issue"`
	ExpectedPlan            []string       `json:"expected_plan"`
	ExpectedTasks           []string       `json:"expected_tasks"`
	ExpectedTDD             []string       `json:"expected_tdd"`
	ExpectedSubagents       []string       `json:"expected_subagents"`
	ExpectedPR              []string       `json:"expected_pr"`
	ExpectedPioneerArtifact []string       `json:"expected_pioneer_artifact,omitempty"`
	CriticalFailures        []string       `json:"critical_failures"`
}

// SkillRouting is one "skill fired at phase" pairing. ExpectedRouting (fixture)
// and RoutingTrace (artifact) use it to score skill_routing_fidelity (A5).
type SkillRouting struct {
	Phase string `json:"phase"`
	Skill string `json:"skill"`
}
