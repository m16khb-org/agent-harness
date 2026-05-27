package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	CodexSkillPath         string `json:"codex_skill_path"`
	CodexSkillInstalled    bool   `json:"codex_skill_installed"`
	CodexMCPConfigured     bool   `json:"codex_mcp_configured"`
	ClaudeSkillPath        string `json:"claude_skill_path"`
	ClaudeSkillInstalled   bool   `json:"claude_skill_installed"`
	ClaudeUserHook         bool   `json:"claude_user_session_start_hook"`
	ProjectClaudeSkillPath string `json:"project_claude_skill_path"`
	ProjectClaudeSkill     bool   `json:"project_claude_skill"`
	ProjectClaudeMCPConfig bool   `json:"project_claude_mcp_config"`
	ProjectClaudeHook      bool   `json:"project_claude_session_start_hook"`
	MCPBinaryPath          string `json:"mcp_binary_path"`
}

func InspectHarness(root, target, home, version, skillName string) InspectInfo {
	codexSkill := filepath.Join(home, ".codex", "skills", skillName)
	claudeSkill := filepath.Join(home, ".claude", "skills", skillName)
	projectClaudeSkill := filepath.Join(root, ".claude", "skills", skillName)
	mcpBinary := filepath.Join(root, "bin", "harness")
	return InspectInfo{
		OK:          true,
		Version:     version,
		HarnessRoot: root,
		TargetRepo:  target,
		Skills:      ListSkills(root, skillName),
		Docs:        ListDocs(root),
		Integration: IntegrationStatus{
			CodexSkillPath:         codexSkill,
			CodexSkillInstalled:    exists(filepath.Join(codexSkill, "SKILL.md")),
			CodexMCPConfigured:     CodexMCPConfigured(filepath.Join(home, ".codex", "config.toml")),
			ClaudeSkillPath:        claudeSkill,
			ClaudeSkillInstalled:   exists(filepath.Join(claudeSkill, "SKILL.md")),
			ProjectClaudeSkillPath: projectClaudeSkill,
			ProjectClaudeSkill:     exists(filepath.Join(projectClaudeSkill, "SKILL.md")),
			ClaudeUserHook:         ClaudeSessionStartHookConfigured(filepath.Join(home, ".claude", "settings.json")),
			ProjectClaudeMCPConfig: exists(filepath.Join(root, ".mcp.json")),
			ProjectClaudeHook:      ClaudeSessionStartHookConfigured(filepath.Join(root, ".claude", "settings.json")),
			MCPBinaryPath:          mcpBinary,
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
			HasSkillMD: exists(filepath.Join(p, "SKILL.md")),
			HasOpenAI:  exists(filepath.Join(p, "agents", "openai.yaml")),
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

func ClaudeSessionStartHookConfigured(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, `"SessionStart"`) &&
		strings.Contains(text, "startup|resume|clear|compact") &&
		strings.Contains(text, "session-start-llm-wiki.sh")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
