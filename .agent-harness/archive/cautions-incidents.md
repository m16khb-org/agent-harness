---
name: cautions-incidents
description: Dated caution incident notes moved out of the active CAUTIONS.md reading path.
---

# Cautions Incident Archive

This file preserves dated caution entries that should remain searchable without making `.agent-harness/CAUTIONS.md` longer than the active evergreen guidance.

## 2026-05-31 — Codex PreCompact hook stdout schema

- Kind: `caution`
- Source: cli
- Summary: Codex 0.135 PreCompact and PostCompact hook output rejects hookSpecificOutput/additionalContext; compact hook stdout must stay in compact-control shape such as {}, suppressOutput-only, or systemMessage. Model-facing additionalContext injection belongs to SessionStart/UserPromptSubmit/PostToolUse-style hooks whose installed Codex schema explicitly allows it.

## 2026-05-31 — PreToolUse false-positive risk

- Kind: `caution`
- Source: codex/claude runtime evidence
- Summary: Codex 0.135.0 and Claude Code 2.1.158 both expose PreToolUse, but the hook runs before every matched tool call and can block or rewrite execution. Keep agent-harness PreToolUse host stdout as `{}` by default and expose only raw `--json` diagnostics until a deterministic policy has host-schema tests and false-positive coverage.
- Do not record lifecycle upkeep from PreToolUse; the tool may not succeed. Use PostToolUse only for observed successful mutating changes, not read-only searches whose output happens to mention lifecycle-relevant paths.
- Follow-up: when an agent-harness hook process exits non-zero, record a redacted JSONL failure event in user state with hook subcommand, host, cwd/repo, tool name, argv, relevant command/query snippet, and error. Codex UI may only show `PreToolUse hook (failed) error: hook exited with code 1`, which is insufficient to distinguish user hooks from plugin hooks or payload-specific failures.

## 2026-05-31 — Do not patch upstream companion plugin caches

- Kind: `caution`
- Source: manual
- Summary: Do not edit installed upstream plugin cache files such as `~/.codex/plugins/cache/claude-mem/...`; fix duplicate or host-specific integration issues in user-owned Codex/Claude settings, wrappers, or upstream itself.
- If an upstream memory provider is installed as a Codex plugin, do not also install the same hooks in `~/.codex/hooks.json`; that double-runs capture hooks and creates duplicated observations/summaries.

## 2026-06-03 — Do not leave self-verify invariant failures as reports only

- Kind: `caution`
- Source: self-verify
- Summary: When `agent-harness self-verify` fails on harness invariants, treat the failure as actionable unless the evidence proves it is physically impossible to fix in the current environment.
- Evidence: `self-verify --progress=jsonl --json` stops at the first failed invariant step, so later coverage gaps are a consequence of early termination, not separate proof that the README or code change is safe.
- Resolution: For forbidden legacy name hits, inspect the exact file:line evidence, generalize personal GitHub owners and absolute local paths to neutral examples, run `rg` for all forbidden needles, then rerun self-verify.

## 2026-06-03 — Separate PR/MR merge from IssueOps worktree cleanup

- Kind: `caution`
- Source: PR #15 merge attempt
- Summary: `gh pr merge --merge --delete-branch` can merge the GitHub PR remotely and then fail during local branch cleanup when the base branch, such as `main`, is already checked out in another linked worktree. In IssueOps worktree flows, run provider merge first without local cleanup flags, then verify merged state, remote source branch state, and worktree cleanliness before deleting the remote source branch, removing the feature worktree, and deleting the local branch.
- Do not rely on provider CLI merge flags that also perform local branch/worktree cleanup from a feature worktree. For GitHub, avoid `gh pr merge "$PR_NUMBER" --merge --delete-branch`; use `gh pr merge "$PR_NUMBER" --merge`, then explicit post-merge cleanup. For GitLab, verify whether the installed `glab`/API flag is remote-only before using it; otherwise merge first and clean up remote/local state in separate commands.

## 2026-06-03 — Do not stop at cosmetic symptom fixes

- Kind: `caution`
- Source: user directive
- Summary: When fixing a failure, degraded workflow, confusing output, or repeated user pain point, do not stop after patching the visible symptom. Trace the reproduction, call path, state transition, policy boundary, and documented contract until the root cause is identified and removed.
- Avoid: changing only wording, loosening tests, adding one-off guards, hiding errors, or documenting around broken behavior unless evidence shows the root cause is already fixed or cannot be fixed in the current scope.
- Verify: add or run a targeted regression test, command smoke, grep evidence, or log/state check that proves the same class of failure no longer reaches the user-facing path.
