# Gemini Code Assist Review Guide

Use this guide when reviewing `m16khb/agent-harness` pull requests. Focus on concrete correctness, safety, and maintainability risks. Avoid generic style comments unless they hide a real defect or contract drift.

## Architecture Boundaries

- Prefer host-neutral behavior in `internal/core` or `internal/port` when the logic must be shared by Codex and Claude Code.
- Keep Codex and Claude adapters thin. Host adapters should translate installation, hook, or MCP surfaces; they must not duplicate command policy, lifecycle state, or redaction rules.
- Do not recommend new abstractions unless there is a real variation point, external boundary, or repeated responsibility. Flag speculative abstractions that make a small change larger without reducing risk.

## Hook And MCP Contracts

- Hook stdout schema compatibility matters. Review changes to `cmd/harness/hook_*`, `configs/codex/**`, and `configs/claude/**` for Codex/Claude differences before suggesting a shared output shape.
- PreToolUse hooks are on the critical path. They should be cheap, deterministic, and no-op by default unless an explicit policy gate is enabled.
- MCP and CLI JSON fields should stay aligned with the same core DTOs. If a response contract changes, require matching golden/schema updates.

## State, Logs, And Secrets

- User state, queue, audit, worker, and hook failure records must be redacted before writing. Secret-like tokens, env assignments, local config contents, and private paths should not appear in logs, prompts, issue bodies, PR bodies, or test output.
- Runtime state belongs under the harness user state directory or ignored runtime paths, not tracked source files.
- Treat `.env`, `.mcp.json`, `dbhub.toml`, credentials, and local-only config as sensitive unless the PR proves they are safe and required.

## IssueOps And Worktrees

- Issue-driven work should create or link the remote issue before implementation when repo, credentials, and ownership are clear.
- Implementation should happen in an isolated sibling worktree following `../<repo>.worktrees/<branch-slug-with-slashes-replaced>`.
- Worker or reviewer prompts should verify `pwd`, branch, `HEAD`, and expected worktree path before inspecting or editing.
- Do not ask to nest worktrees inside the source checkout just to make an editor show them.

## Testing And Verification

- For Go behavior changes, expect focused tests first, then `go test ./... -count=1` when the blast radius touches shared CLI/MCP/core behavior.
- For configuration-only changes, verify parser validity and exact file scope.
- For generated golden files, check that the source behavior changed intentionally and that the golden update is not hiding unrelated drift.
- For generated or mechanical artifacts, review the source contract and generation command first. Do not comment on generated output unless it is missing, stale, or inconsistent with the source behavior.
- For comments and docs, flag text that restates obvious code without explaining intent, assumptions, user-visible contract, or verification evidence.

## Review Scope And Severity

- Treat MEDIUM comments as actionable defects, missing verification, security/privacy risk, or contract drift. Avoid low-value nits that do not change behavior or reviewability.
- Prefer one precise inline comment over several broad comments when one root cause explains multiple changed lines.
- Do not comment on line length, formatting, import order, or style-only details when existing formatters, linters, or `git diff --check` are the right enforcement mechanism.
- When a PR changes only templates, prompts, plans, or configuration, focus on parser validity, host compatibility, ignored/generated file scope, secret exposure, and whether the documented verification command actually succeeds.

## Review Tone

- Write review summaries and inline review comments primarily in Korean. Keep commands, code identifiers, file paths, API names, and upstream project names in their original English form.
- Lead with defects, regressions, missing verification, and contract risks.
- Keep comments specific to changed files and cite the relevant behavior or invariant.
- Avoid broad refactors, preference-only formatting changes, and suggestions outside the pull request's stated goal.
