# IssueOps Design

## Problem

Agents need a repeatable workflow that starts from ambiguous user problems and ends with a verified GitHub/GitLab PR or MR. The current harness has strong individual skills and gates, but no single protocol that ties problem intake, issue creation, planning, TDD implementation, feedback, and PR/MR preparation into one observable cycle.

## Success Criteria

- A shared `issueops` skill guides Codex and Claude Code through the same issue-driven workflow.
- The workflow uses `brainstorming` for problem intake, `grill-with-docs` for domain-language pressure testing, `writing-plans` for issue-based plans, and `test-driven-development` plus `subagent-driven-development` for implementation.
- A lightweight IssueOps state surface records the current phase, issue URL, plan path, branch, feedback, and verification evidence without writing runtime state into the target repository.
- UserPromptSubmit routing can suggest the workflow when prompts mention IssueOps, issue-driven implementation, feedback loops, or PR/MR creation, but hooks never create issues, run plans, commit, push, or open PRs automatically.

## Non-Goals

- Do not build a full GitHub/GitLab client in harness core.
- Do not replace existing specialized skills.
- Do not let hooks execute workflow steps.
- Do not store secrets, tokens, or issue bodies containing private credentials in IssueOps state.

## Architecture

The first-class user entrypoint is a shared skill at `skills/issueops/SKILL.md`. It orchestrates existing skills and keeps approval gates explicit.

The harness core owns only durable state and read-only readiness helpers. The CLI/MCP surface records and reports cycle metadata; it does not perform provider-specific issue or PR writes. Agents still use the host's available GitHub/GitLab tools to create issues and PRs after user approval.

UserPromptSubmit hooks remain advisory. They can recommend `issueops` and show the active IssueOps phase, but they do not advance the phase or perform external writes.

## Domain Language

- **IssueOps loop**: one issue-driven workflow instance for one target repository and branch.
- **Phase**: the current gate in the cycle: `problem`, `grill`, `issue`, `plan`, `implement`, `feedback`, or `pr`.
- **Issue contract**: the GitHub/GitLab issue URL plus acceptance criteria and test expectations used as the source for planning.
- **Plan path**: the implementation plan file generated from the issue contract.
- **Feedback item**: user or reviewer input that must be classified as issue change, plan change, test change, or implementation change.

## Workflow

1. Problem intake with `brainstorming`.
2. Domain grill with `grill-with-docs`; update glossary/ADR only when a term or hard-to-reverse decision is resolved.
3. Issue draft/create/update after explicit user approval.
4. Issue-based plan with `writing-plans`.
5. Plan grill with `grill-with-docs`.
6. Implementation with `test-driven-development` and `subagent-driven-development`.
7. Feedback loop; classify feedback before editing code.
8. PR/MR readiness check and draft.

## Verification

- Skill metadata validates with `quick_validate.py`.
- IssueOps CLI JSON can start, inspect, link issue, link plan, add feedback, and report PR readiness against a temporary state directory.
- MCP tools expose the same state contract as CLI response DTOs.
- Hook routing tests prove issue-driven prompts recommend `issueops` without executing it.
