// Package inspect는 inspect capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package inspect

type InspectInfo struct {
	OK           bool              `json:"ok"`
	Version      string            `json:"version"`
	IssueOpsRoot string            `json:"issueops_root"`
	TargetRepo   string            `json:"target_repo"`
	Skills       []SkillInfo       `json:"skills"`
	Docs         []string          `json:"docs"`
	Integration  IntegrationStatus `json:"integration"`
	GeneratedAt  string            `json:"generated_at"`
}

type IntegrationStatus struct {
	CodexSkillPath         string `json:"codex_skill_path"`
	CodexSkillInstalled    bool   `json:"codex_skill_installed"`
	CodexMCPConfigured     bool   `json:"codex_mcp_configured"`
	ClaudeSkillPath        string `json:"claude_skill_path"`
	ClaudeSkillInstalled   bool   `json:"claude_skill_installed"`
	ProjectClaudeSkillPath string `json:"project_claude_skill_path"`
	ProjectClaudeSkill     bool   `json:"project_claude_skill"`
	ProjectClaudeMCPConfig bool   `json:"project_claude_mcp_config"`
	MCPBinaryPath          string `json:"mcp_binary_path"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasSkillMD  bool   `json:"has_skill_md"`
	HasOpenAI   bool   `json:"has_openai_yaml"`
	Description string `json:"description,omitempty"`
}
