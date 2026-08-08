package projectbootstrap

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoccontract "agent-harness/internal/contract/projectdoc"
)

type (
	ProjectSignals         = projectdoccontract.ProjectSignals
	ProjectDocsPlannedFile = projectdoccontract.ProjectDocsPlannedFile
)

type ProjectDocsBootstrapRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
	Sync     bool   `json:"sync"`
}

type ProjectDocsBootstrapResult struct {
	OK             bool                                        `json:"ok"`
	Kind           string                                      `json:"kind"`
	RepoRoot       string                                      `json:"repo_root"`
	DocsDir        string                                      `json:"docs_dir"`
	Write          bool                                        `json:"write"`
	Sync           bool                                        `json:"sync"`
	DryRun         bool                                        `json:"dry_run"`
	GeneratedAt    string                                      `json:"generated_at"`
	Signals        ProjectSignals                              `json:"signals"`
	Files          []ProjectDocsPlannedFile                    `json:"files"`
	LifecycleState lifecyclecontract.ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Warnings       []string                                    `json:"warnings,omitempty"`
}
