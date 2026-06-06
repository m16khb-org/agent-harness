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
