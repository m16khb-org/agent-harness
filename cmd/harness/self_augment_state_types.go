package main

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
