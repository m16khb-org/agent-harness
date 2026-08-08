package inspect

import (
	inspectcontract "agent-harness/internal/contract/inspect"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	coredocs "agent-harness/internal/adapter/docs"
)

func InspectHarness(root, target, home, version, skillName string) inspectcontract.InspectInfo {
	codexSkill := filepath.Join(home, ".codex", "skills", skillName)
	claudeSkill := filepath.Join(home, ".claude", "skills", skillName)
	projectClaudeSkill := filepath.Join(root, ".claude", "skills", skillName)
	mcpBinary := filepath.Join(root, "bin", "agent-harness")
	return inspectcontract.InspectInfo{
		OK:          true,
		Version:     version,
		HarnessRoot: root,
		TargetRepo:  target,
		Skills:      ListSkills(root, skillName),
		Docs:        coredocs.ListDocs(root),
		Integration: inspectcontract.IntegrationStatus{
			CodexSkillPath:         codexSkill,
			CodexSkillInstalled:    Exists(filepath.Join(codexSkill, "SKILL.md")),
			CodexMCPConfigured:     CodexMCPConfigured(filepath.Join(home, ".codex", "config.toml")),
			ClaudeSkillPath:        claudeSkill,
			ClaudeSkillInstalled:   Exists(filepath.Join(claudeSkill, "SKILL.md")),
			ProjectClaudeSkillPath: projectClaudeSkill,
			ProjectClaudeSkill:     Exists(filepath.Join(projectClaudeSkill, "SKILL.md")),
			ProjectClaudeMCPConfig: Exists(filepath.Join(root, ".mcp.json")),
			MCPBinaryPath:          mcpBinary,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}

func ListSkills(root, skillName string) []inspectcontract.SkillInfo {
	dir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []inspectcontract.SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		s := inspectcontract.SkillInfo{
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
