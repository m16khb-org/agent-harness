package projectdocs

type ProjectDocsRecordRequest struct {
	RepoRoot     string   `json:"repo_root"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Context      string   `json:"context,omitempty"`
	Resolution   string   `json:"resolution,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Consequences string   `json:"consequences,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type ProjectDocsRecordResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RecordKind    string   `json:"record_kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	GeneratedAt   string   `json:"generated_at"`
	BytesAppended int      `json:"bytes_appended"`
	SHA256        string   `json:"sha256"`
	Warnings      []string `json:"warnings,omitempty"`
}

type ProjectDocsReadResult struct {
	OK          bool     `json:"ok"`
	Kind        string   `json:"kind"`
	RepoRoot    string   `json:"repo_root"`
	RelPath     string   `json:"rel_path"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Content     string   `json:"content,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	Warnings    []string `json:"warnings,omitempty"`
}
