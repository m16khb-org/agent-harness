package selfworkflow

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
	Error                  string   `json:"error,omitempty"`
}
