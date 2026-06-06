package main

const (
	selfVerificationSummaryKind     = "self_verification_summary"
	legacySelfAugmentSummaryKind    = "self_augment_summary"
	selfVerificationKoreanName      = "자기 검증 루프"
	selfAugmentationKoreanName      = "자가 증강 루프"
	defaultLoopTargetScoreExclusive = 95.0
)

type SelfAugmentStateCheckpoint struct {
	OK       bool   `json:"ok"`
	Key      string `json:"key"`
	StateDir string `json:"state_dir,omitempty"`
	Path     string `json:"path,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Error    string `json:"error,omitempty"`
}

type SelfAugmentStateSnapshot struct {
	SchemaVersion int                `json:"schema_version"`
	Kind          string             `json:"kind"`
	LoopKind      string             `json:"loop_kind,omitempty"`
	KoreanName    string             `json:"korean_name,omitempty"`
	OK            bool               `json:"ok"`
	Iterations    int                `json:"iterations"`
	BaseSeed      int64              `json:"base_seed"`
	TargetScore   float64            `json:"target_score,omitempty"`
	ElapsedMS     int64              `json:"elapsed_ms"`
	HarnessRoot   string             `json:"harness_root"`
	GeneratedAt   string             `json:"generated_at"`
	Summary       SelfAugmentSummary `json:"summary"`
}

type SelfAugmentCompareResult struct {
	OK                           bool                              `json:"ok"`
	StateDir                     string                            `json:"state_dir"`
	BaselineKey                  string                            `json:"baseline_key"`
	CandidateKey                 string                            `json:"candidate_key"`
	MaxElapsedRegressionPct      float64                           `json:"max_elapsed_regression_pct"`
	Regressed                    bool                              `json:"regressed"`
	ElapsedDeltaMS               int64                             `json:"elapsed_delta_ms"`
	ElapsedDeltaPct              float64                           `json:"elapsed_delta_pct"`
	FailedStepsDelta             int                               `json:"failed_steps_delta"`
	TotalStepsDelta              int                               `json:"total_steps_delta"`
	BaselineMinimumGoalScore     float64                           `json:"baseline_minimum_goal_score"`
	CandidateMinimumGoalScore    float64                           `json:"candidate_minimum_goal_score"`
	MissingStepLabels            []string                          `json:"missing_step_labels"`
	AddedStepLabels              []string                          `json:"added_step_labels"`
	Regressions                  []string                          `json:"regressions"`
	Warnings                     []string                          `json:"warnings"`
	BaselineSummary              SelfAugmentSummary                `json:"baseline_summary"`
	CandidateSummary             SelfAugmentSummary                `json:"candidate_summary"`
	BaselineSnapshotGeneratedAt  string                            `json:"baseline_snapshot_generated_at"`
	CandidateSnapshotGeneratedAt string                            `json:"candidate_snapshot_generated_at"`
	BaselineSlowestSteps         []SelfAugmentSlowStep             `json:"baseline_slowest_steps"`
	CandidateSlowestSteps        []SelfAugmentSlowStep             `json:"candidate_slowest_steps"`
	SlowStepRegressions          []SelfAugmentSlowStepRegression   `json:"slow_step_regressions"`
	BaselineStepDurationStats    []SelfAugmentStepDurationStat     `json:"baseline_step_duration_stats"`
	CandidateStepDurationStats   []SelfAugmentStepDurationStat     `json:"candidate_step_duration_stats"`
	StepBudgetRegressions        []SelfAugmentStepBudgetRegression `json:"step_budget_regressions"`
}

type SelfAugmentSlowStepRegression struct {
	Label               string  `json:"label"`
	BaselineDurationMS  int64   `json:"baseline_duration_ms"`
	CandidateDurationMS int64   `json:"candidate_duration_ms"`
	DeltaMS             int64   `json:"delta_ms"`
	DeltaPct            float64 `json:"delta_pct"`
}

type SelfAugmentStepBudgetRegression struct {
	Label               string  `json:"label"`
	Metric              string  `json:"metric"`
	BaselineDurationMS  int64   `json:"baseline_duration_ms"`
	CandidateDurationMS int64   `json:"candidate_duration_ms"`
	DeltaMS             int64   `json:"delta_ms"`
	DeltaPct            float64 `json:"delta_pct"`
	BaselineCount       int     `json:"baseline_count"`
	CandidateCount      int     `json:"candidate_count"`
}

type SelfAugmentPromoteResult struct {
	OK                  bool               `json:"ok"`
	StateDir            string             `json:"state_dir"`
	FromKey             string             `json:"from_key"`
	BaselineKey         string             `json:"baseline_key"`
	Confirm             bool               `json:"confirm"`
	DryRun              bool               `json:"dry_run"`
	Promoted            bool               `json:"promoted"`
	Path                string             `json:"path,omitempty"`
	Bytes               int                `json:"bytes,omitempty"`
	SnapshotGeneratedAt string             `json:"snapshot_generated_at"`
	Summary             SelfAugmentSummary `json:"summary"`
}

type SelfAugmentHistoryResult struct {
	OK           bool                         `json:"ok"`
	StateDir     string                       `json:"state_dir"`
	Prefix       string                       `json:"prefix"`
	Limit        int                          `json:"limit"`
	TotalMatches int                          `json:"total_matches"`
	Returned     int                          `json:"returned"`
	Retention    *SelfAugmentHistoryRetention `json:"retention,omitempty"`
	Entries      []SelfAugmentHistoryEntry    `json:"entries"`
	Skipped      []SelfAugmentHistorySkipped  `json:"skipped"`
	Warnings     []string                     `json:"warnings"`
}

type SelfAugmentHistoryRetention struct {
	Enabled        bool     `json:"enabled"`
	Limit          int      `json:"limit"`
	TotalMatches   int      `json:"total_matches"`
	RetainedKeys   []string `json:"retained_keys"`
	CandidateKeys  []string `json:"candidate_keys"`
	DeletedKeys    []string `json:"deleted_keys"`
	PruneRequested bool     `json:"prune_requested"`
	Confirm        bool     `json:"confirm"`
	DryRun         bool     `json:"dry_run"`
	Recommendation string   `json:"recommendation"`
}

type selfAugmentHistoryRetentionOptions struct {
	Limit          int
	PruneRequested bool
	Confirm        bool
}

type SelfAugmentHistoryEntry struct {
	Key          string                `json:"key"`
	UpdatedAt    string                `json:"updated_at"`
	Bytes        int                   `json:"bytes"`
	GeneratedAt  string                `json:"generated_at"`
	OK           bool                  `json:"ok"`
	Iterations   int                   `json:"iterations"`
	BaseSeed     int64                 `json:"base_seed"`
	ElapsedMS    int64                 `json:"elapsed_ms"`
	TotalRuns    int                   `json:"total_runs"`
	TotalSteps   int                   `json:"total_steps"`
	FailedSteps  int                   `json:"failed_steps"`
	StepLabels   []string              `json:"step_labels"`
	SlowestSteps []SelfAugmentSlowStep `json:"slowest_steps"`
}

type SelfAugmentHistorySkipped struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type SelfAugmentIteration struct {
	Iteration int          `json:"iteration"`
	Seed      int64        `json:"seed"`
	Steps     []StepResult `json:"steps"`
}

type StepResult struct {
	Label           string `json:"label"`
	Command         string `json:"command,omitempty"`
	OK              bool   `json:"ok"`
	DurationMS      int64  `json:"duration_ms"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutBytes     int    `json:"stdout_bytes,omitempty"`
	StderrBytes     int    `json:"stderr_bytes,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	Error           string `json:"error,omitempty"`
}
