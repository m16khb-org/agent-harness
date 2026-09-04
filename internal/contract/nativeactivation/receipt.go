package nativeactivation

const SchemaVersion = 1

type Request struct {
	StateRoot    string `json:"state_root"`
	IssueOpsRoot string `json:"issueops_root"`
	TargetBinary string `json:"target_binary"`
	TransitionID string `json:"transition_id,omitempty"`
}

type Evidence struct {
	Host           string `json:"host"`
	Surface        string `json:"surface"`
	Path           string `json:"path"`
	SemanticSHA256 string `json:"semantic_sha256"`
	SHA256         string `json:"sha256"`
	Mode           uint32 `json:"mode"`
	Size           int64  `json:"size"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
}

type Receipt struct {
	SchemaVersion int        `json:"schema_version"`
	StateRoot     string     `json:"state_root"`
	IssueOpsRoot  string     `json:"issueops_root"`
	TargetBinary  string     `json:"target_binary"`
	BinarySHA256  string     `json:"binary_sha256"`
	TransitionID  string     `json:"transition_id"`
	CatalogSHA256 string     `json:"catalog_sha256"`
	Evidence      []Evidence `json:"evidence"`
	SealedAt      string     `json:"sealed_at"`
}

type Result struct {
	OK           bool     `json:"ok"`
	StateRoot    string   `json:"state_root"`
	IssueOpsRoot string   `json:"issueops_root"`
	TargetBinary string   `json:"target_binary"`
	BinarySHA256 string   `json:"binary_sha256,omitempty"`
	TransitionID string   `json:"transition_id"`
	Pending      bool     `json:"pending"`
	Sealed       bool     `json:"sealed"`
	Aborted      bool     `json:"aborted,omitempty"`
	UpdatedAt    string   `json:"updated_at"`
	Receipt      *Receipt `json:"receipt,omitempty"`
}
