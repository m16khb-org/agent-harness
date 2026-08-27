package lifecycle

import projectdoccontract "agent-harness/internal/contract/projectdoc"

type ProjectFingerprint struct {
	RepoRoot      string `json:"repo_root"`
	GitDir        string `json:"git_dir,omitempty"`
	GitOriginHash string `json:"git_origin_hash,omitempty"`
}
type ProjectLifecycleProfile struct {
	SchemaVersion int                                `json:"schema_version"`
	RepoID        string                             `json:"repo_id"`
	Fingerprint   ProjectFingerprint                 `json:"fingerprint"`
	Metadata      *projectdoccontract.ProjectProfile `json:"metadata,omitempty"`
	CreatedAt     string                             `json:"created_at"`
	UpdatedAt     string                             `json:"updated_at"`
}
type ProjectLifecycleStatePlan struct {
	OK              bool                     `json:"ok"`
	SchemaVersion   int                      `json:"schema_version"`
	RepoRoot        string                   `json:"repo_root"`
	RepoID          string                   `json:"repo_id"`
	StateRoot       string                   `json:"state_root"`
	ProjectStateDir string                   `json:"project_state_dir"`
	ProjectJSONPath string                   `json:"project_json_path"`
	QueuePath       string                   `json:"queue_path"`
	CompactPath     string                   `json:"compact_path"`
	Fingerprint     ProjectFingerprint       `json:"fingerprint"`
	Exists          bool                     `json:"exists"`
	NamespaceValid  bool                     `json:"namespace_valid"`
	Profile         *ProjectLifecycleProfile `json:"profile,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}
