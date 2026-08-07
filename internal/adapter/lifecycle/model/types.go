package model

import (
	"agent-harness/internal/adapter/projectdocs"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

const ProjectLifecycleSchemaVersion = 1
const ProjectLifecycleProfileFile = "project.json"
const DocUpkeepQueueFile = "doc-upkeep-queue.jsonl"
const CompactCapsuleFile = "compact-capsule.json"
const StopNextActionRelayFile = "stop-next-action-relay.json"

type ProjectFingerprint struct {
	RepoRoot      string `json:"repo_root"`
	GitDir        string `json:"git_dir,omitempty"`
	GitOriginHash string `json:"git_origin_hash,omitempty"`
}

type ProjectLifecycleProfile struct {
	SchemaVersion int                         `json:"schema_version"`
	RepoID        string                      `json:"repo_id"`
	Fingerprint   ProjectFingerprint          `json:"fingerprint"`
	Metadata      *projectdocs.ProjectProfile `json:"metadata,omitempty"`
	CreatedAt     string                      `json:"created_at"`
	UpdatedAt     string                      `json:"updated_at"`
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

type DocUpkeepAppendResult struct {
	OK              bool                             `json:"ok"`
	RepoRoot        string                           `json:"repo_root"`
	RepoID          string                           `json:"repo_id"`
	ProjectStateDir string                           `json:"project_state_dir"`
	Path            string                           `json:"path"`
	Event           lifecyclecontract.DocUpkeepEvent `json:"event"`
}

type LifecycleStopReminderResult struct {
	OK                bool   `json:"ok"`
	ShouldInject      bool   `json:"should_inject"`
	AdditionalContext string `json:"additional_context,omitempty"`
	PendingCount      int    `json:"pending_count"`
}

type LifecycleCompactCapsule struct {
	SchemaVersion     int                                `json:"schema_version"`
	RepoRoot          string                             `json:"repo_root"`
	RepoID            string                             `json:"repo_id"`
	CreatedAt         string                             `json:"created_at"`
	RequiredDocs      []string                           `json:"required_docs,omitempty"`
	PendingDocUpkeep  []lifecyclecontract.DocUpkeepEvent `json:"pending_doc_upkeep,omitempty"`
	AdditionalSummary string                             `json:"additional_summary,omitempty"`
	// Nonce uniquely identifies one capsule write so the PostCompact consume can
	// compare-and-swap on (CreatedAt, Nonce) and never delete a newer capsule
	// that happens to share a coarse-clock CreatedAt. Empty on legacy capsules,
	// which safely degrades the CAS to CreatedAt-only (its prior behavior).
	Nonce string `json:"nonce,omitempty"`
}

type LifecycleCompactResult struct {
	OK                bool     `json:"ok"`
	Recorded          bool     `json:"recorded,omitempty"`
	ShouldInject      bool     `json:"should_inject,omitempty"`
	AdditionalContext string   `json:"additional_context,omitempty"`
	PendingCount      int      `json:"pending_count,omitempty"`
	CompactPath       string   `json:"compact_path,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}
