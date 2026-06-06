package benchmark

type IssueOpsBenchmarkFixture struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	UserPrompt        string   `json:"user_prompt"`
	RepoContext       string   `json:"repo_context"`
	ExpectedIssue     []string `json:"expected_issue"`
	ExpectedPlan      []string `json:"expected_plan"`
	ExpectedTasks     []string `json:"expected_tasks"`
	ExpectedTDD       []string `json:"expected_tdd"`
	ExpectedSubagents []string `json:"expected_subagents"`
	ExpectedPR        []string `json:"expected_pr"`
	CriticalFailures  []string `json:"critical_failures"`
}

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
}

type IssueOpsDimensionScore struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	Evidence  string  `json:"evidence"`
}

type IssueOpsBenchmarkScore struct {
	OK                    bool                     `json:"ok"`
	FixtureID             string                   `json:"fixture_id"`
	AverageScore          float64                  `json:"average_score"`
	MinimumScore          float64                  `json:"minimum_score"`
	DimensionScores       []IssueOpsDimensionScore `json:"dimension_scores"`
	DeterministicFailures []string                 `json:"deterministic_failures"`
	JudgeFailures         []string                 `json:"judge_failures"`
	CriticalFailures      []string                 `json:"critical_failures"`
	Passed                bool                     `json:"passed"`
}

type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures  []IssueOpsBenchmarkFixture
	Artifacts map[string]IssueOpsBenchmarkArtifact
}

type IssueOpsBenchmarkRunResult struct {
	OK                   bool                     `json:"ok"`
	ID                   string                   `json:"id"`
	FixtureCount         int                      `json:"fixture_count"`
	AverageScore         float64                  `json:"average_score"`
	MinimumScore         float64                  `json:"minimum_score"`
	CriticalFailureCount int                      `json:"critical_failure_count"`
	Scores               []IssueOpsBenchmarkScore `json:"scores"`
}

type IssueOpsBenchmarkCompareResult struct {
	OK                   bool     `json:"ok"`
	Improved             bool     `json:"improved"`
	BaselineID           string   `json:"baseline_id"`
	CandidateID          string   `json:"candidate_id"`
	AverageScoreDelta    float64  `json:"average_score_delta"`
	MinimumScoreDelta    float64  `json:"minimum_score_delta"`
	CriticalFailureDelta int      `json:"critical_failure_delta"`
	Regressions          []string `json:"regressions"`
}

type IssueOpsAutoresearchCandidate struct {
	ID               string   `json:"id"`
	Hypothesis       string   `json:"hypothesis"`
	TargetDimensions []string `json:"target_dimensions"`
	EditSurface      []string `json:"edit_surface"`
	BaselineCommand  string   `json:"baseline_command,omitempty"`
	CandidateCommand string   `json:"candidate_command,omitempty"`
	KeepCriteria     string   `json:"keep_criteria,omitempty"`
	DiscardCriteria  string   `json:"discard_criteria,omitempty"`
}

type IssueOpsAutoresearchGateRequest struct {
	Candidate    IssueOpsAutoresearchCandidate
	BaselineRun  IssueOpsBenchmarkRunResult
	CandidateRun IssueOpsBenchmarkRunResult
	ChangedPaths []string
}

type IssueOpsAutoresearchGateResult struct {
	OK                         bool                           `json:"ok"`
	KeepCandidate              bool                           `json:"keep_candidate"`
	CandidateID                string                         `json:"candidate_id"`
	BenchmarkCompare           IssueOpsBenchmarkCompareResult `json:"benchmark_compare"`
	EditSurfaceViolations      []string                       `json:"edit_surface_violations,omitempty"`
	TargetDimensionRegressions []string                       `json:"target_dimension_regressions,omitempty"`
	DiscardReasons             []string                       `json:"discard_reasons,omitempty"`
}

var issueOpsBenchmarkDimensions = []string{
	"intent_understanding",
	"issue_quality",
	"domain_contract_quality",
	"plan_quality",
	"api_doc_gate_quality",
	"live_evidence_quality",
	"task_decomposition",
	"tdd_quality",
	"subagent_orchestration",
	"review_feedback_accountability",
	"implementation_readiness",
	"pr_mr_quality",
	"phase_control_quality",
	"branch_worktree_gate_quality",
	"isolation_compliance",
	"completion_hygiene_quality",
	"worktree_cleanup_quality",
}

const issueOpsBenchmarkMaxScore = 100.0
