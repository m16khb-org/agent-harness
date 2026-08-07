package projectdocs

type ProjectDocsUpdateRequest struct {
	RepoRoot       string   `json:"repo_root"`
	RelPath        string   `json:"rel_path"`
	Content        string   `json:"content"`
	ExpectedSHA256 string   `json:"expected_sha256,omitempty"`
	Summary        string   `json:"summary"`
	Evidence       []string `json:"evidence,omitempty"`
	Confirm        bool     `json:"confirm"`
}

type ProjectDocsUpdateResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	Action        string   `json:"action"`
	Confirmed     bool     `json:"confirmed"`
	DryRun        bool     `json:"dry_run"`
	GeneratedAt   string   `json:"generated_at"`
	CurrentSHA256 string   `json:"current_sha256,omitempty"`
	NextSHA256    string   `json:"next_sha256"`
	Bytes         int      `json:"bytes"`
	Summary       string   `json:"summary"`
	Evidence      []string `json:"evidence,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}
