package model

import "agent-harness/internal/core/projectdocs"

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

type DocUpkeepEvent struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	TargetDocs []string `json:"target_docs"`
	Summary    string   `json:"summary"`
	Evidence   []string `json:"evidence,omitempty"`
	Source     string   `json:"source,omitempty"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
}

type DocUpkeepAppendResult struct {
	OK              bool           `json:"ok"`
	RepoRoot        string         `json:"repo_root"`
	RepoID          string         `json:"repo_id"`
	ProjectStateDir string         `json:"project_state_dir"`
	Path            string         `json:"path"`
	Event           DocUpkeepEvent `json:"event"`
}

type HookToolUseLifecycleRequest struct {
	Repo                 string   `json:"repo,omitempty"`
	Tool                 string   `json:"tool,omitempty"`
	Paths                []string `json:"paths,omitempty"`
	Command              string   `json:"command,omitempty"`
	Source               string   `json:"source,omitempty"`
	EnforceSearchRouting bool     `json:"enforce_search_routing,omitempty"`
	EnforceWorktree      bool     `json:"enforce_worktree,omitempty"`
	EnforceKoreanRemote  bool     `json:"enforce_korean_remote,omitempty"`
	EnforceVCSLinking    bool     `json:"enforce_vcs_linking,omitempty"`
	EnforceGitOpsKubectl bool     `json:"enforce_gitops_kubectl,omitempty"`
	EnforceStagedChecks  bool     `json:"enforce_staged_checks,omitempty"`
	ExpectedWorktree     string   `json:"expected_worktree,omitempty"`
	SourceCheckout       string   `json:"source_checkout,omitempty"`
	ProjectPath          string   `json:"project_path,omitempty"`
}

type HookToolUseLifecycleResult struct {
	OK       bool           `json:"ok"`
	Recorded bool           `json:"recorded"`
	Event    DocUpkeepEvent `json:"event,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}

type HookPreToolUseDecisionResult struct {
	OK       bool     `json:"ok"`
	Decision string   `json:"decision"`
	Reason   string   `json:"reason,omitempty"`
	Tool     string   `json:"tool,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Command  string   `json:"command,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type LifecycleStopReminderResult struct {
	OK                bool   `json:"ok"`
	ShouldInject      bool   `json:"should_inject"`
	AdditionalContext string `json:"additional_context,omitempty"`
	PendingCount      int    `json:"pending_count"`
}

type StopNextActionRelayCandidate struct {
	Index       int    `json:"index"`
	Recommended bool   `json:"recommended,omitempty"`
	Text        string `json:"text"`
}

type StopNextActionRelayRecord struct {
	SchemaVersion    int                            `json:"schema_version"`
	Fingerprint      string                         `json:"fingerprint"`
	RecommendedIndex int                            `json:"recommended_index,omitempty"`
	RecommendedText  string                         `json:"recommended_text,omitempty"`
	Candidates       []StopNextActionRelayCandidate `json:"candidates,omitempty"`
	UpdatedAt        string                         `json:"updated_at"`
}

type StopNextActionRelayResult struct {
	OK          bool     `json:"ok"`
	ShouldRelay bool     `json:"should_relay"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Path        string   `json:"path,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type LifecycleCompactCapsule struct {
	SchemaVersion     int              `json:"schema_version"`
	RepoRoot          string           `json:"repo_root"`
	RepoID            string           `json:"repo_id"`
	CreatedAt         string           `json:"created_at"`
	RequiredDocs      []string         `json:"required_docs,omitempty"`
	PendingDocUpkeep  []DocUpkeepEvent `json:"pending_doc_upkeep,omitempty"`
	AdditionalSummary string           `json:"additional_summary,omitempty"`
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
