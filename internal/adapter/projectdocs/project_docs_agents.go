package projectdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderAgentsWithBlock(root, existing string) string {
	block := strings.TrimSpace(fmt.Sprintf(`%s
## agent-harness project docs

This repository uses agent-harness project docs. Read existing AGENTS.md rules first, then read only the additional documents relevant to the task.

- Architecture or large design changes: %[2]s/ARCHITECTURE.md, %[2]s/CONSTITUTION.md
- Testing or verification changes: %[2]s/TESTING.md
- Endpoint/DTO/OpenAPI changes: %[2]s/OPEN_API_SPEC.md
- Commit or PR work: %[2]s/COMMIT_POLICY.md
- Code style or structure changes: %[2]s/CONVENTIONS.md
- Dependency or tech-stack changes: %[2]s/TECH_STACK.md
- Run, deploy, environment, or local development: %[2]s/OPERATIONS.md
- Agent start, verification, and completion workflow: %[2]s/AGENT_WORKFLOW.md
- Risky or recurring-failure work: %[2]s/CAUTIONS.md
- Structural rationale, alternatives, and decisions: %[2]s/ADR.md
- Session start, instruction conflicts, and principle decisions: %[2]s/CONSTITUTION.md
%s`, agentsStartMarker, ProjectDocsDir, agentsEndMarker)) + "\n"
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
