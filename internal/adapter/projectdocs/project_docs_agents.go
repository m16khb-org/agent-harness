package projectdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderAgentsWithBlock(root, existing string) string {
	bullets := []string{
		"- Architecture or large design changes: " + ProjectDocsDir + "/ARCHITECTURE.md, " + ProjectDocsDir + "/CONSTITUTION.md",
		"- Testing or verification changes: " + ProjectDocsDir + "/TESTING.md",
		"- Endpoint/DTO/OpenAPI changes: " + ProjectDocsDir + "/OPEN_API_SPEC.md",
		"- Commit or PR work: " + ProjectDocsDir + "/COMMIT_POLICY.md",
		"- Code style or structure changes: " + ProjectDocsDir + "/CONVENTIONS.md",
		"- Dependency or tech-stack changes: " + ProjectDocsDir + "/TECH_STACK.md",
		"- Run, deploy, environment, or local development: " + ProjectDocsDir + "/OPERATIONS.md",
		"- Agent start, verification, and completion workflow: " + ProjectDocsDir + "/AGENT_WORKFLOW.md",
		"- Risky or recurring-failure work: " + ProjectDocsDir + "/CAUTIONS.md",
		"- Structural rationale, alternatives, and decisions: " + ProjectDocsDir + "/ADR.md",
		"- Session start, instruction conflicts, and principle decisions: " + ProjectDocsDir + "/CONSTITUTION.md",
	}
	if designDocExists(root) {
		bullets = append(bullets, "- UI, styling, or design-system changes: "+ProjectDocsDir+"/DESIGN.md (client repositories only)")
	}
	block := strings.TrimSpace(fmt.Sprintf(`%s
## agent-harness project docs

This repository uses agent-harness project docs. Read existing AGENTS.md rules first, then read only the additional documents relevant to the task.

%s
%s`, agentsStartMarker, strings.Join(bullets, "\n"), agentsEndMarker)) + "\n"
	path := filepath.Join(root, "AGENTS.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimRight(behavioralGuidelines, "\n") + "\n\n---\n\n" + block + "\n"
	}
	text := ensureBehavioralGuidelinesAtTop(string(b))
	start := strings.Index(text, agentsStartMarker)
	end := strings.Index(text, agentsEndMarker)
	if start >= 0 && end > start {
		end += len(agentsEndMarker)
		return strings.TrimRight(text[:start], "\n") + "\n\n" + block + strings.TrimLeft(text[end:], "\n")
	}
	return strings.TrimRight(text, "\n") + "\n\n" + block
}

// designDocExists reports whether this repo carries a design-system doc:
// either the agent-facing .agent-harness/DESIGN.md or a curated root
// DESIGN.md that stays authoritative for the design system.
func designDocExists(root string) bool {
	for _, rel := range []string{
		filepath.Join(ProjectDocsDir, "DESIGN.md"),
		"DESIGN.md",
	} {
		if info, err := os.Stat(filepath.Join(root, rel)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func ensureBehavioralGuidelinesAtTop(text string) string {
	trimmed := strings.TrimLeft(text, "\ufeff\n\r\t ")
	if strings.HasPrefix(trimmed, "# AGENTS.md\n\nBehavioral guidelines to reduce common LLM coding mistakes.") {
		return text
	}
	// agent-harness is a library applied to many repositories: when the
	// existing AGENTS.md already opens with its own heading and guidance,
	// that curated content stays authoritative. Do not stack the generic
	// behavioral template on top of repo-authored rules.
	if strings.HasPrefix(trimmed, "# ") {
		return text
	}
	return strings.TrimRight(behavioralGuidelines, "\n") + "\n\n---\n\n" + strings.TrimLeft(text, "\n")
}
