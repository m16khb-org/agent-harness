package projectdoc

import projectdoccontract "agent-harness/internal/contract/projectdoc"

type ProjectSignals struct {
	Files               []string                          `json:"files"`
	Languages           []string                          `json:"languages"`
	PackageManagers     []string                          `json:"package_managers"`
	Profile             projectdoccontract.ProjectProfile `json:"profile"`
	TestCommands        []EvidenceCommand                 `json:"test_commands"`
	BuildCommands       []EvidenceCommand                 `json:"build_commands"`
	LintCommands        []EvidenceCommand                 `json:"lint_commands"`
	ExistingAgentDocs   []string                          `json:"existing_agent_docs"`
	GitHubWorkflows     []string                          `json:"github_workflows"`
	DetectedConventions []string                          `json:"detected_conventions"`
}

type EvidenceCommand struct {
	Command    string   `json:"command"`
	Evidence   []string `json:"evidence"`
	Confidence string   `json:"confidence"`
}
