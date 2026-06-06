package core

type ProjectDocsBootstrapRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
	Sync     bool   `json:"sync"`
}

type ProjectDocsBootstrapResult struct {
	OK             bool                      `json:"ok"`
	Kind           string                    `json:"kind"`
	RepoRoot       string                    `json:"repo_root"`
	DocsDir        string                    `json:"docs_dir"`
	Write          bool                      `json:"write"`
	Sync           bool                      `json:"sync"`
	DryRun         bool                      `json:"dry_run"`
	GeneratedAt    string                    `json:"generated_at"`
	Signals        ProjectSignals            `json:"signals"`
	Files          []ProjectDocsPlannedFile  `json:"files"`
	LifecycleState ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Warnings       []string                  `json:"warnings,omitempty"`
}

type ProjectDocsPlannedFile struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Action  string `json:"action"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	Reason  string `json:"reason"`
}
