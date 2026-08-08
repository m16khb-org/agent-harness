package projectdocs

import (
	projectdoc "agent-harness/internal/domain/projectdoc"
	"strings"
)

func renderADR() string {
	return `# Architecture Decision Records

## Purpose

Record structural choices, rejected alternatives, and decisions that affect long-term maintenance. This is not an implementation note; preserve why this structure was chosen and which alternatives should not be retried.

## When to read

- Before architecture changes, large refactors, or dependency/framework replacement
- When changing or bypassing existing structure
- When modifying code whose historical rationale is unclear

## When to append

- A new structure or boundary was chosen.
- Alternatives were considered and rejection reasons will reduce future re-analysis.
- Operations, performance, or security constraints shaped the design.

## Entry template

### YYYY-MM-DD: <decision title>

- Context: <problem and constraints>
- Decision: <chosen structure>
- Alternatives rejected:
  - <alternative>: <why rejected>
- Consequences: <tradeoffs and follow-up>
- Evidence: <files, commands, issues, docs>
`
}

func renderOperations(signals projectdoc.ProjectSignals) string {
	var b strings.Builder
	b.WriteString("# Operations\n\n")
	b.WriteString("## Local development\n\n")
	b.WriteString("- Check README, package scripts, Makefile/Taskfile, and CI workflows first.\n")
	if len(signals.BuildCommands) > 0 {
		b.WriteString("- Candidate build commands are listed in TESTING.md and should be verified before use.\n")
	}
	b.WriteString("\n## Environment and secrets\n\n")
	b.WriteString("- Do not put raw .env values, credentials, API keys, or local state in docs or logs.\n")
	b.WriteString("- Document only required environment variable names and purposes; use OS keychain/env references for values.\n")
	b.WriteString("\n## Deploy/release\n\n")
	b.WriteString("- Do not infer deploy procedures automatically. Verify them from CI/CD workflows and operations docs.\n")
	b.WriteString("\n## Project docs bootstrap and upkeep\n\n")
	b.WriteString("- `agent-harness project bootstrap --repo . --json` creates docs and user-state repo metadata; `--sync` refreshes them from current evidence.\n")
	b.WriteString("- After initial setup, agents should read repo evidence and keep `.agent-harness` docs current through MCP `project_docs_route` → `project_docs_read` → `project_docs_update`.\n")
	b.WriteString("- Append resolved false cases and decisions to CAUTIONS/ADR with `project_docs_record` instead of rewriting full documents.\n")
	b.WriteString("\n## UserPromptSubmit hook\n\n")
	b.WriteString("- When the host supports it, connect `agent-harness hook user-prompt` to UserPromptSubmit to inject short agent_harness MCP candidates for each user prompt.\n")
	b.WriteString("- The hook does not execute work; it only performs static keyword routing. It does not use the network or read large files.\n")
	return b.String()
}

func renderAgentWorkflow() string {
	return `# Agent Workflow

## Start

1. Read AGENTS.md first.
2. At session start, treat .agent-harness/CONSTITUTION.md as the baseline principle document.
3. If MCP is available, send the current task to project_docs_route and select only necessary docs.
4. Verify inferred doc claims against current files and command output.

## MCP usage rule

- When the host supports it, agent-harness hook user-prompt injects MCP candidate hints for each user instruction. The hint is a reminder for judgment, not an auto-execution command.
- Use MCP when the task needs current state, repo-specific doc routing, policy decisions, state checkpoints, or durable records that the model should not rely on from memory.
- Do not use MCP for simple reasoning or summarizing already opened files.
- Avoid exposing many tools at once; narrowly use route/read/update/record/check tools that match the task.
- Do not trust tool output blindly; check paths, exists flags, warnings, and verification evidence.

## Work

Use the Simplicity First and Surgical Changes principles from AGENTS.md, plus these project record/safety rules.

- Do not overwrite existing user changes.
- Add dependencies, deploy, or perform destructive actions only with explicit instruction or strong evidence.
- If docs diverge from current code or user consensus, use project_docs_read to verify the current SHA and project_docs_update to change one document at a time.
- When a problem occurred and was resolved, record it with MCP project_docs_record(kind=caution) in .agent-harness/CAUTIONS.md.
- When a structural decision or rejected alternative matters, record it with MCP project_docs_record(kind=adr) in .agent-harness/ADR.md.

## Verify

Use the Goal-Driven Execution principle from AGENTS.md, plus these verification routing rules.

- Before writing or modifying tests, read the good/bad test criteria in .agent-harness/TESTING.md.
- When changing CLI/MCP/API documentation contracts, also run golden/schema/smoke verification.
- Completion reports must include test/build/static-check results and reasons for skipped verification.

## Finish

- If a commit is needed, follow .agent-harness/COMMIT_POLICY.md.
- Record resolved false cases or structural decisions with MCP project_docs_record when useful.
`
}

func bulletListWithFallback(items []string, fallback string) string {
	if len(items) == 0 {
		return "- " + fallback + "\n"
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString("- " + item + "\n")
	}
	return b.String()
}

func commandList(commands []projectdoc.EvidenceCommand) string {
	if len(commands) == 0 {
		return "- No auto-inferred commands. Check README, CI workflows, and package scripts.\n"
	}
	var b strings.Builder
	for _, cmd := range commands {
		b.WriteString("- `" + cmd.Command + "`\n")
		b.WriteString("  - Evidence: " + strings.Join(cmd.Evidence, ", ") + "\n")
		b.WriteString("  - Confidence: " + cmd.Confidence + "\n")
	}
	return b.String()
}
