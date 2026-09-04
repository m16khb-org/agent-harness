# 2026-06-09 — Expose IssueOps gate contracts through MCP and skills

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: codex
- Summary: IssueOps readiness gates must be visible in CLI help, MCP schema descriptions, and SKILL.md so agents do not infer hidden flags or state fields.
- Context: A design review approval failure exposed design_review_evidence only as an internal missing key, causing repeated attempts to find nonexistent evidence flags, approval subcommands, decision records, and force transitions.
- Decision: Treat every IssueOps readiness gate key and conditional approval requirement as a public agent contract. New or changed gates must update CLI guidance, MCP schema descriptions, shared skill instructions, and focused contract tests in the same change.
- Consequences: Future IssueOps changes that hide required gate inputs from MCP or SKILL.md should fail contract tests before reaching installed binaries.
