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

type ProjectSignals struct {
	Files               []string          `json:"files"`
	Languages           []string          `json:"languages"`
	PackageManagers     []string          `json:"package_managers"`
	Profile             ProjectProfile    `json:"profile"`
	TestCommands        []EvidenceCommand `json:"test_commands"`
	BuildCommands       []EvidenceCommand `json:"build_commands"`
	LintCommands        []EvidenceCommand `json:"lint_commands"`
	ExistingAgentDocs   []string          `json:"existing_agent_docs"`
	GitHubWorkflows     []string          `json:"github_workflows"`
	DetectedConventions []string          `json:"detected_conventions"`
}

type EvidenceCommand struct {
	Command    string   `json:"command"`
	Evidence   []string `json:"evidence"`
	Confidence string   `json:"confidence"`
}

type ProjectProfile struct {
	VCS             ProjectVCSProfile `json:"vcs"`
	Languages       []string          `json:"languages"`
	PackageManagers []string          `json:"package_managers,omitempty"`
	ProjectTypes    []string          `json:"project_types,omitempty"`
	Frameworks      []string          `json:"frameworks,omitempty"`
	Monorepo        bool              `json:"monorepo"`
	Evidence        []string          `json:"evidence,omitempty"`
}

type ProjectVCSProfile struct {
	Provider   string `json:"provider"`
	Hosting    string `json:"hosting"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemoteName string `json:"remote_name,omitempty"`
}

type ProjectDocsRouteResult struct {
	OK          bool                   `json:"ok"`
	Kind        string                 `json:"kind"`
	RepoRoot    string                 `json:"repo_root"`
	Task        string                 `json:"task"`
	GeneratedAt string                 `json:"generated_at"`
	Docs        []ProjectDocRouteEntry `json:"docs"`
	Warnings    []string               `json:"warnings,omitempty"`
}

type ProjectDocRouteEntry struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	Exists  bool   `json:"exists"`
}

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

type routeDoc struct{ rel, reason string }
