package main

type SelfAugmentPlanRequest struct {
	Cycles      int     `json:"cycles"`
	TargetScore float64 `json:"target_score"`
}

const (
	selfAugmentationPlanKind   = "self_augmentation_plan"
	selfAugmentationLessonKind = "self_augmentation_lesson"
)

type SelfAugmentPlanResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Cycles              int                         `json:"cycles"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	HarnessRoot         string                      `json:"harness_root"`
	GeneratedAt         string                      `json:"generated_at"`
	GeniusThinkPath     string                      `json:"genius_think_path"`
	UsesGeniusThink     bool                        `json:"uses_genius_think"`
	SelectedFormulas    []string                    `json:"selected_formulas"`
	ResearchInfluences  []SelfAugmentInfluence      `json:"research_influences"`
	Goals               []SelfAugmentGoal           `json:"goals"`
	Candidates          []SelfAugmentCandidate      `json:"candidates"`
	SelectedCandidate   *SelfAugmentCandidate       `json:"selected_candidate,omitempty"`
	ExecutionProtocol   []string                    `json:"execution_protocol"`
	VerificationGate    []string                    `json:"verification_gate"`
	Warnings            []string                    `json:"warnings"`
	RepoSignals         SelfAugmentRepoSignals      `json:"repo_signals"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentInfluence struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Adopted string `json:"adopted"`
}

type SelfAugmentGoal struct {
	Name        string   `json:"name"`
	KoreanName  string   `json:"korean_name"`
	Score       float64  `json:"score"`
	TargetScore float64  `json:"target_score"`
	Passed      bool     `json:"passed"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
}

type SelfAugmentCandidate struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	Impact               float64  `json:"impact"`
	Feasibility          float64  `json:"feasibility"`
	Novelty              float64  `json:"novelty"`
	Risk                 float64  `json:"risk"`
	WhyNow               []string `json:"why_now"`
	ExpectedGain         []string `json:"expected_gain"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
}

type SelfAugmentRepoSignals struct {
	DocsIndexed                 int      `json:"docs_indexed"`
	Skills                      []string `json:"skills"`
	HasGeniusThink              bool     `json:"has_genius_think"`
	HasSelfAugmentSkill         bool     `json:"has_self_augment_skill"`
	HasSelfVerificationDocs     bool     `json:"has_self_verification_docs"`
	HasSelfVerifyCLI            bool     `json:"has_self_verify_cli"`
	HasSelfAugmentPlanner       bool     `json:"has_self_augment_planner"`
	HasSelfAugmentStateCapture  bool     `json:"has_self_augment_state_capture"`
	HasSelfAugmentLessonCapture bool     `json:"has_self_augment_lesson_capture"`
	HasAdapterContractMatrix    bool     `json:"has_adapter_contract_matrix"`
	HasRiskQATier               bool     `json:"has_risk_qa_tier"`
	HasGoalScoreSummary         bool     `json:"has_goal_score_summary"`
	HasRepoLocalSandbox         bool     `json:"has_repo_local_sandbox"`
	HasPerformanceBaseline      bool     `json:"has_performance_baseline"`
	HasGeniusMermaidLint        bool     `json:"has_genius_mermaid_lint"`
	HasInstallDryRunMode        bool     `json:"has_install_dry_run_mode"`
	HasCLIAdapterSplit          bool     `json:"has_cli_adapter_split"`
	HasMCPAdapterCatalog        bool     `json:"has_mcp_adapter_catalog"`
	HasCompatibilityContract    bool     `json:"has_compatibility_contract"`
	HasCandidateRefill          bool     `json:"has_candidate_refill"`
	HasCommandAuditLog          bool     `json:"has_command_audit_log"`
	HasWorkerMVP                bool     `json:"has_worker_mvp"`
}

type SelfAugmentLessonRequest struct {
	CandidateID string `json:"candidate_id"`
	Lesson      string `json:"lesson"`
	NextAction  string `json:"next_action"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
	StateKey    string `json:"state_key"`
}

type SelfAugmentLessonResult struct {
	OK              bool                        `json:"ok"`
	Kind            string                      `json:"kind"`
	LoopKind        string                      `json:"loop_kind"`
	KoreanName      string                      `json:"korean_name"`
	CandidateID     string                      `json:"candidate_id"`
	Lesson          string                      `json:"lesson"`
	NextAction      string                      `json:"next_action"`
	Source          string                      `json:"source"`
	Severity        string                      `json:"severity"`
	HarnessRoot     string                      `json:"harness_root"`
	GeneratedAt     string                      `json:"generated_at"`
	StateCheckpoint *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
}

type SelfAugmentLessonStateSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	LoopKind      string `json:"loop_kind"`
	KoreanName    string `json:"korean_name"`
	OK            bool   `json:"ok"`
	CandidateID   string `json:"candidate_id"`
	Lesson        string `json:"lesson"`
	NextAction    string `json:"next_action"`
	Source        string `json:"source"`
	Severity      string `json:"severity"`
	HarnessRoot   string `json:"harness_root"`
	GeneratedAt   string `json:"generated_at"`
}

type SelfAugmentPlanStateSnapshot struct {
	SchemaVersion         int                    `json:"schema_version"`
	Kind                  string                 `json:"kind"`
	LoopKind              string                 `json:"loop_kind"`
	KoreanName            string                 `json:"korean_name"`
	OK                    bool                   `json:"ok"`
	Cycles                int                    `json:"cycles"`
	TargetScore           float64                `json:"target_score"`
	HarnessRoot           string                 `json:"harness_root"`
	GeneratedAt           string                 `json:"generated_at"`
	SelectedCandidate     *SelfAugmentCandidate  `json:"selected_candidate,omitempty"`
	CandidateCount        int                    `json:"candidate_count"`
	OpenCandidateIDs      []string               `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string               `json:"satisfied_candidate_ids"`
	Goals                 []SelfAugmentGoal      `json:"goals"`
	SelectedFormulas      []string               `json:"selected_formulas"`
	ResearchInfluences    []SelfAugmentInfluence `json:"research_influences"`
}
