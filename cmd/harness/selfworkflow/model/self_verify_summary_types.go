package model

import "agent-harness/cmd/harness/commandstep"

type SelfAugmentResult struct {
	OK                  bool                        `json:"ok"`
	LoopKind            string                      `json:"loop_kind"`
	KoreanName          string                      `json:"korean_name"`
	Iterations          int                         `json:"iterations"`
	BaseSeed            int64                       `json:"base_seed"`
	TargetScore         float64                     `json:"target_score"`
	TerminationEligible bool                        `json:"termination_eligible"`
	ElapsedMS           int64                       `json:"elapsed_ms"`
	HarnessRoot         string                      `json:"harness_root"`
	InspiredBy          string                      `json:"inspired_by"`
	LoopContract        []string                    `json:"loop_contract"`
	Summary             SelfAugmentSummary          `json:"summary"`
	StateCheckpoint     *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	LLMEval             *SelfVerifyLLMEvalResult    `json:"llm_eval,omitempty"`
	Runs                []SelfAugmentIteration      `json:"runs"`
}

type SelfVerifyLLMEvalResult struct {
	OK                     bool     `json:"ok"`
	Mode                   string   `json:"mode"`
	ExecutionClass         string   `json:"execution_class"`
	ReadOnly               bool     `json:"read_only"`
	Score                  float64  `json:"score"`
	Summary                string   `json:"summary,omitempty"`
	Blockers               []string `json:"blockers,omitempty"`
	Risks                  []string `json:"risks,omitempty"`
	RecommendedNextActions []string `json:"recommended_next_actions,omitempty"`
	EvidencePacketBytes    int      `json:"evidence_packet_bytes"`
	Prompt                 string   `json:"prompt,omitempty"`
	Error                  string   `json:"error,omitempty"`
}

type SelfAugmentSummary struct {
	TotalRuns           int                              `json:"total_runs"`
	TotalSteps          int                              `json:"total_steps"`
	PassedSteps         int                              `json:"passed_steps"`
	FailedSteps         int                              `json:"failed_steps"`
	TargetScore         float64                          `json:"target_score"`
	Contract            SelfVerificationContract         `json:"contract"`
	MinimumGoalScore    float64                          `json:"minimum_goal_score"`
	TerminationEligible bool                             `json:"termination_eligible"`
	GoalScores          []SelfVerificationGoalScore      `json:"goal_scores"`
	Coverage            []SelfVerificationCoverage       `json:"coverage"`
	CoverageGaps        []string                         `json:"coverage_gaps"`
	RerunCommands       []string                         `json:"rerun_commands,omitempty"`
	FailureClass        string                           `json:"failure_class,omitempty"`
	FailureClassReason  string                           `json:"failure_class_reason,omitempty"`
	FailureClusters     []SelfVerificationFailureCluster `json:"failure_clusters,omitempty"`
	FailedIteration     int                              `json:"failed_iteration,omitempty"`
	FailedSeed          int64                            `json:"failed_seed,omitempty"`
	FailedStep          string                           `json:"failed_step,omitempty"`
	StepLabels          []string                         `json:"step_labels"`
	SlowestSteps        []SelfAugmentSlowStep            `json:"slowest_steps"`
	StepDurationStats   []SelfAugmentStepDurationStat    `json:"step_duration_stats"`
}

type SelfVerificationContract struct {
	Name           string   `json:"name"`
	Version        int      `json:"version"`
	Hash           string   `json:"hash"`
	RequiredFields []string `json:"required_fields"`
	GoalNames      []string `json:"goal_names"`
	CoverageClaims []string `json:"coverage_claims"`
}

type SelfVerificationGoalScore struct {
	Name           string   `json:"name"`
	KoreanName     string   `json:"korean_name"`
	Score          float64  `json:"score"`
	TargetScore    float64  `json:"target_score"`
	Passed         bool     `json:"passed"`
	EvidenceLabels []string `json:"evidence_labels"`
	PassedChecks   int      `json:"passed_checks"`
	TotalChecks    int      `json:"total_checks"`
}

type SelfVerificationCoverage struct {
	Claim          string   `json:"claim"`
	EvidenceLabels []string `json:"evidence_labels"`
	Covered        bool     `json:"covered"`
	MissingLabels  []string `json:"missing_labels"`
}

type SelfVerificationFailureCluster struct {
	Step  string  `json:"step"`
	Seeds []int64 `json:"seeds"`
	Count int     `json:"count"`
}

type SelfAugmentSlowStep struct {
	Iteration  int    `json:"iteration"`
	Seed       int64  `json:"seed"`
	Label      string `json:"label"`
	DurationMS int64  `json:"duration_ms"`
}

type SelfAugmentStepDurationStat struct {
	Label             string  `json:"label"`
	Count             int     `json:"count"`
	MinDurationMS     int64   `json:"min_duration_ms"`
	MaxDurationMS     int64   `json:"max_duration_ms"`
	AverageDurationMS float64 `json:"average_duration_ms"`
	P95DurationMS     int64   `json:"p95_duration_ms"`
}

type SelfVerificationGoalDefinition struct {
	Name       string
	KoreanName string
	Labels     []string
}

type SelfVerificationCoverageDefinition struct {
	Claim  string
	Labels []string
}

type StepResult = commandstep.StepResult
