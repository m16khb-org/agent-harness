package projectdocs

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
