---
name: cautions/lessons/2026-08-21-issueops-whoami-record-flags-and-branch-examples.md
description: Dated lesson — full local issueops lifecycle dogfood found whoami advertised claim-shaped flags only and branch errors cited foreign-project examples; both fixed on 2026-08-21.
---

# 2026-08-21 — issueops lifecycle dogfood: whoami flags were claim-only; branch errors cited foreign issues

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: full local issueops lifecycle dogfooding in a temp clone + temp state (2026-08-21); start → intent → plan-prep → domain-review → decision → link-issue → branch prepare (+base_sha, --link-verified) → design review → worktree → plan stage/link → compatibility review → execution prepare preview/confirm → implement → implementation-review → ai-slop-clean record → feedback add. `pr` phase correctly refused locally (`upstream`, `worktree_clean`, `implementation_changes` need a real remote/commits).
- Summary: two discoverability defects and one usage trap.

## 1. `execution whoami` advertised only claim-shaped flags

`whoami --json` emitted `claim_actor_flags` (receipt flags with
`--session-pid/--session-started-at/--session-executable`). The ~30 record-style
subcommands that dominate the lifecycle take RECORD_ACTOR_FLAGS
(`--host/--session-id/--agent-id/--cwd`) and **reject the receipt flags** with
`flag provided but not defined`. An agent following the whoami guidance hit
immediate rejections. Fix: whoami now also emits `record_actor_flags`
(one copy-pasteable RECORD_ACTOR_FLAGS vector with `--cwd` = current directory).

Usage trap that mimics a harness bug: the flag strings are **shell-quoted**.
Building an argv array by word-splitting (`args=($FLAGS)`) keeps the literal
single quotes inside values, so the receipt compares unequal and execution
commands fail with `native session process receipt is not in the local process
ancestry` even though the PID really is an ancestor. Paste the flags into a
command line (or `eval`) so the shell interprets the quotes; never
program-split them.

## 2. Branch-format error cited an unrelated repository's issues

`ValidateBranch` rejected `dogfood/lifecycle` with examples
`2387-fix-grpc-ai-dmm-tag-replication-lag` /
`2388-fanza-delete-404-stale-registered` — real issue titles from a different
project, confusing out of context. Fix: neutral examples
(`123-fix-login-timeout` / `456-refactor-issueops-gates`) locked by
`TestValidateBranchErrorUsesNeutralExamples`.

## 3. Gates that looked like friction but were correct

- worktree must be under the sibling `<repo>.worktrees` directory; plan file
  must be staged **inside** the linked worktree;
- approved design review requires `--alternative`, `--risk`, `--refactor-plan`,
  and verification text asserting alternatives/risks were reviewed
  (anti-rubber-stamp);
- branch prepare needs `--base-sha` + `--link-verified` before execution
  prepare confirms;
- after execution prepare, mutations require `--cwd` = canonical worktree —
  rerun whoami from the worktree to get the right vector.
