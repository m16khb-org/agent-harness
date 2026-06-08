package inspect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	coredocs "agent-harness/internal/core/docs"
)

type InspectInfo struct {
	OK          bool              `json:"ok"`
	Version     string            `json:"version"`
	HarnessRoot string            `json:"harness_root"`
	TargetRepo  string            `json:"target_repo"`
	Skills      []SkillInfo       `json:"skills"`
	Docs        []string          `json:"docs"`
	Integration IntegrationStatus `json:"integration"`
	GeneratedAt string            `json:"generated_at"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasSkillMD  bool   `json:"has_skill_md"`
	HasOpenAI   bool   `json:"has_openai_yaml"`
	Description string `json:"description,omitempty"`
}

type IntegrationStatus struct {
	CodexSkillPath            string `json:"codex_skill_path"`
	CodexSkillInstalled       bool   `json:"codex_skill_installed"`
	CodexMCPConfigured        bool   `json:"codex_mcp_configured"`
	ClaudeSkillPath           string `json:"claude_skill_path"`
	ClaudeSkillInstalled      bool   `json:"claude_skill_installed"`
	ProjectClaudeSkillPath    string `json:"project_claude_skill_path"`
	ProjectClaudeSkill        bool   `json:"project_claude_skill"`
	ProjectClaudeMCPConfig    bool   `json:"project_claude_mcp_config"`
	ReasonixSkillPath         string `json:"reasonix_skill_path"`
	ReasonixSkillInstalled    bool   `json:"reasonix_skill_installed"`
	ReasonixSettingsInstalled bool   `json:"reasonix_settings_installed"`
	ReasonixMCPConfigured     bool   `json:"reasonix_mcp_configured"`
	ProjectReasonixSkillPath  string `json:"project_reasonix_skill_path"`
	ProjectReasonixSkill      bool   `json:"project_reasonix_skill"`
	ProjectReasonixSettings   bool   `json:"project_reasonix_settings"`
	MCPBinaryPath             string `json:"mcp_binary_path"`
}

func InspectHarness(root, target, home, version, skillName string) InspectInfo {
	codexSkill := filepath.Join(home, ".codex", "skills", skillName)
	claudeSkill := filepath.Join(home, ".claude", "skills", skillName)
	reasonixSkill := filepath.Join(home, ".reasonix", "skills", skillName)
	projectClaudeSkill := filepath.Join(root, ".claude", "skills", skillName)
	projectReasonixSkill := filepath.Join(root, ".reasonix", "skills", skillName)
	mcpBinary := filepath.Join(root, "bin", "agent-harness")
	reasonixConfigDir, _ := os.UserConfigDir()
	if reasonixConfigDir == "" {
		reasonixConfigDir = filepath.Join(home, ".config")
	}
	reasonixConfigPath := filepath.Join(reasonixConfigDir, "reasonix", "config.toml")
	return InspectInfo{
		OK:          true,
		Version:     version,
		HarnessRoot: root,
		TargetRepo:  target,
		Skills:      ListSkills(root, skillName),
		Docs:        coredocs.ListDocs(root),
		Integration: IntegrationStatus{
			CodexSkillPath:            codexSkill,
			CodexSkillInstalled:       Exists(filepath.Join(codexSkill, "SKILL.md")),
			CodexMCPConfigured:        CodexMCPConfigured(filepath.Join(home, ".codex", "config.toml")),
			ClaudeSkillPath:           claudeSkill,
			ClaudeSkillInstalled:      Exists(filepath.Join(claudeSkill, "SKILL.md")),
			ProjectClaudeSkillPath:    projectClaudeSkill,
			ProjectClaudeSkill:        Exists(filepath.Join(projectClaudeSkill, "SKILL.md")),
			ProjectClaudeMCPConfig:    Exists(filepath.Join(root, ".mcp.json")),
			ReasonixSkillPath:         reasonixSkill,
			ReasonixSkillInstalled:    Exists(filepath.Join(reasonixSkill, "SKILL.md")),
			ReasonixSettingsInstalled: Exists(filepath.Join(home, ".reasonix", "settings.json")),
			ReasonixMCPConfigured:     fileContains(reasonixConfigPath, "agent_harness"),
			ProjectReasonixSkillPath:  projectReasonixSkill,
			ProjectReasonixSkill:      Exists(filepath.Join(projectReasonixSkill, "SKILL.md")),
			ProjectReasonixSettings:   Exists(filepath.Join(root, ".reasonix", "settings.json")),
			MCPBinaryPath:             mcpBinary,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}

func ListSkills(root, skillName string) []SkillInfo {
	dir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		s := SkillInfo{
			Name:       e.Name(),
			Path:       p,
			HasSkillMD: Exists(filepath.Join(p, "SKILL.md")),
			HasOpenAI:  Exists(filepath.Join(p, "agents", "openai.yaml")),
		}
		s.Description = readSkillDescription(filepath.Join(p, "SKILL.md"))
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readSkillDescription(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), `"'`)
		}
	}
	return ""
}

func CodexMCPConfigured(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "[mcp_servers.agent_harness]")
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileContains(path, substr string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), substr)
}
