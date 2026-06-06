package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func renderProjectDocs(root string, signals ProjectSignals) map[string]string {
	out := map[string]string{}
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "ARCHITECTURE.md"))] = renderArchitecture(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CAUTIONS.md"))] = renderCautions(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "COMMIT_POLICY.md"))] = renderCommitPolicy()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONSTITUTION.md"))] = renderConstitution()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "CONVENTIONS.md"))] = renderConventions(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "TECH_STACK.md"))] = renderTechStack(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "TESTING.md"))] = renderTesting(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPEN_API_SPEC.md"))] = renderOpenAPISpec()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "ADR.md"))] = renderADR()
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPERATIONS.md"))] = renderOperations(signals)
	out[filepath.ToSlash(filepath.Join(ProjectDocsDir, "AGENT_WORKFLOW.md"))] = renderAgentWorkflow()
	// Prepend canonical meta frontmatter so created/synced docs declare what
	// category of information they hold. Same doc name => same metadata.
	for rel, content := range out {
		out[rel] = ensureDocMetaFrontmatter(filepath.Base(rel), content)
	}
	return out
}

func renderAgentsWithBlock(root, _ string) string {
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
	return strings.TrimRight(behavioralGuidelines, "\n") + "\n\n---\n\n" + strings.TrimLeft(text, "\n")
}

func renderOpenAPISpec() string {
	return `# OpenAPI Spec Guidance

## Purpose

This project-specific API documentation prompt is for agents and MCP routing when endpoint, controller, handler, DTO, schema, or OpenAPI files change.

## Gate order

1. Static gate: ` + "`agent-harness api-doc static-check --json`" + `
2. Agent gate: ` + "`agent-harness api-doc review --json`" + `
3. Combined gate: ` + "`agent-harness api-doc check --json`" + `

Default scope is staged API candidate files. Scan all legacy debt only when ` + "`--all`" + ` is explicitly supplied.

## Static omissions to block

- missing route operation summary/description
- description does not follow the repo's sectioned Markdown format
- missing path/query/header/body parameter documentation
- missing 400 response when validation surface exists
- missing 401 response for private/auth endpoints
- OpenAPI decorator or optional-validation mismatch on required/optional DTO fields

## Agent review prompt

Static checks catch decorator/comment-level omissions. Agent review reads directly related business logic to detect public API contract drift.

The agent must inspect service/usecase/domain/error-mapping code called by changed endpoints. If these errors can occur, they must appear in OpenAPI responses.

- entity/resource not found → 404
- auth/session/token failure → 401
- permission/ownership/tier/role failure → 403
- validation/body/query/header problem → 400
- duplicate/state conflict/idempotency conflict → 409

Documentation must not contradict real behavior. For example, if docs say the endpoint only reads cache but it changes payment state, or docs omit 404 while a service can throw NotFound, that is a blocking issue.

## Clean Swagger style

- Operation summary should be short and client-oriented.
- Prefer sectioned Markdown plus bullets for descriptions, such as ` + "`### Purpose`" + `, ` + "`### Request Rules`" + `/` + "`### Processing`" + `, and ` + "`### Auth/Notes`" + `.
- Path/query/header/body parameters should include name, requiredness, format, and example.
- Responses should include client-handled failure statuses with schema/description, not success-only docs.
- Document single-object responses as top-level objects without unnecessary wrapper objects. Exceptions: pagination/list envelopes, explicit metadata contracts, backward compatibility, and standard error envelopes.
- If public/admin/internal docs are separated, filter paths/schemas for the intended audience.
`
}

func renderArchitecture(signals ProjectSignals) string {
	return "# Architecture\n\n## Purpose\n\nThis is an architecture draft generated from project files by agent-harness. Mark weak inferences with Confidence; current code and command output are authoritative.\n\n## Detected structure\n\n" + bulletListWithFallback(signals.Files, "Not enough project signal files were detected.") + "\n## Guidance\n\n- Before large design changes, inspect current entrypoints, package/module boundaries, and data flow.\n- Add new abstractions only after existing patterns and test boundaries are confirmed.\n"
}

func renderCautions(signals ProjectSignals) string {
	items := []string{"Generated docs are drafts; directly verify weak evidence.", "Do not commit secrets, credentials, local state, or generated artifacts."}
	if len(signals.GitHubWorkflows) > 0 {
		items = append(items, "CI workflows exist; compare local verification with CI behavior.")
	}
	return "# Cautions\n\n" + bulletListWithFallback(items, "No cautions recorded.")
}

func renderCommitPolicy() string {
	return "# Commit Policy\n\n" +
		"## Default\n\n" +
		"- Prefer small atomic commits.\n" +
		"- Run verification appropriate to the change scope before committing.\n" +
		"- Use Conventional Commit format unless the project has stricter rules.\n\n" +
		"~~~text\n<type>(<scope>): <summary>\n\nWhy: <why this change exists>\nTested: <commands run>\nNot-tested: <known verification gaps>\n~~~\n\n" +
		"## Safety\n\n" +
		"- Do not stage unrelated changes.\n" +
		"- Manually inspect secret-like paths or credential changes before committing.\n"
}

func renderConstitution() string {
	return `# Constitution

## SessionStart contract

This project-specific constitution should be read at session start. Follow the general LLM coding behavior guidelines at the top of AGENTS.md; this document adds harness structure, security, and verification invariants. Treat it as the baseline principle document for MCP routing.

## Source of truth

1. Latest explicit user/system instructions
2. Current repo AGENTS.md or a nearer nested AGENTS.md
3. .agent-harness/*.md
4. Current files and command output

## Principles

- Host adapters must not bypass core policy.
- Never put raw secrets in docs, logs, test fixtures, or MCP/CLI responses.
- Preserve explicit workspace-root and command-policy boundaries.
- Harness results observed from Codex and Claude Code should match.
`
}

func renderConventions(signals ProjectSignals) string {
	lines := []string{"# Conventions\n\n## Detected conventions\n\n"}
	if len(signals.DetectedConventions) == 0 {
		lines = append(lines, "- Few conventions were auto-detected. Inspect README, config, and existing files first.\n")
	} else {
		lines = append(lines, bulletListWithFallback(signals.DetectedConventions, "No detected conventions."))
	}
	lines = append(lines, "\n## Editing rules\n\n- Follow existing style first.\n- Do not run repo-wide formatting unless explicitly requested.\n- Add new dependencies only after documenting the need and alternatives.\n")
	lines = append(lines, "\n", solidDesignPatternGuidance)
	return strings.Join(lines, "")
}

func renderTechStack(signals ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Tech Stack\n\n## Detected languages\n\n")
	b.WriteString(bulletListWithFallback(signals.Languages, "Could not auto-confirm languages."))
	b.WriteString("\n## Package managers\n\n")
	b.WriteString(bulletListWithFallback(signals.PackageManagers, "Could not auto-confirm package managers."))
	b.WriteString("\n## Evidence files\n\n")
	b.WriteString(bulletListWithFallback(signals.Files, "No evidence files."))
	return b.String()
}

func renderTesting(signals ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Testing\n\n")
	b.WriteString("## Purpose\n\n")
	b.WriteString("This is the reference document agents should read before writing or modifying tests. It records candidate commands and the difference between well-structured and poorly structured tests.\n\n")
	b.WriteString("## When to read\n\n")
	b.WriteString("- When writing tests for a new feature or bug fix\n")
	b.WriteString("- When existing tests fail or are flaky\n")
	b.WriteString("- When proving behavior preservation after refactoring\n")
	b.WriteString("- When deciding which verification to run before completion\n\n")
	b.WriteString("## Well-structured tests\n\n")
	b.WriteString("- Directly verify changed behavior and prefer public contracts/observable behavior over implementation details.\n")
	b.WriteString("- Use assertion messages or fixture names that reveal the failure cause.\n")
	b.WriteString("- Keep tests deterministic, independently runnable, and not overly dependent on order, time, external network, or global state.\n")
	b.WriteString("- Regression tests clearly encode the recurring input/context and expected result.\n")
	b.WriteString("- Reuse existing test style/helpers and explain intent/scope for broad snapshot or golden updates.\n\n")
	b.WriteString("## Poorly-structured tests\n\n")
	b.WriteString("- Locking only internal implementation structure and blocking harmless refactors.\n")
	b.WriteString("- Adding assertions not tied to a real bug or requirement.\n")
	b.WriteString("- Depending on sleeps, real network, local machine state, or test order.\n")
	b.WriteString("- Using huge fixtures or vague snapshots that do not explain failures.\n")
	b.WriteString("- Weakening production behavior just to pass tests.\n\n")
	b.WriteString("## Candidate test commands\n\n")
	b.WriteString(commandList(signals.TestCommands))
	b.WriteString("\n## Candidate build commands\n\n")
	b.WriteString(commandList(signals.BuildCommands))
	b.WriteString("\n## Candidate lint/static checks\n\n")
	b.WriteString(commandList(signals.LintCommands))
	b.WriteString("\n## Rule\n\nVerification commands are candidates only. Before running, check package scripts, CI workflows, and README for more specific instructions. When adding tests, apply the well-structured/poorly-structured criteria above first.\n")
	return b.String()
}
