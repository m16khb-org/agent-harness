package projectdoc

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

type ProjectDocsPlannedFile struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Action  string `json:"action"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	Reason  string `json:"reason"`
}
