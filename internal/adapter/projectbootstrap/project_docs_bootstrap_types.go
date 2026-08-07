package projectbootstrap

import (
	"agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
)

type ProjectDocsBootstrapRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
	Sync     bool   `json:"sync"`
}

type ProjectDocsBootstrapResult struct {
	OK             bool                                `json:"ok"`
	Kind           string                              `json:"kind"`
	RepoRoot       string                              `json:"repo_root"`
	DocsDir        string                              `json:"docs_dir"`
	Write          bool                                `json:"write"`
	Sync           bool                                `json:"sync"`
	DryRun         bool                                `json:"dry_run"`
	GeneratedAt    string                              `json:"generated_at"`
	Signals        projectdocs.ProjectSignals          `json:"signals"`
	Files          []projectdoc.ProjectDocsPlannedFile `json:"files"`
	LifecycleState lifecycle.ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Warnings       []string                            `json:"warnings,omitempty"`
}
