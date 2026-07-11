---
name: OPERATIONS.md
description: Install, sync, runtime, and operational procedures.
---

# Operations Map

`agent-harness` gives Codex and Claude Code the same Go binary, MCP schema, command policy, state store, shared skills, lifecycle hooks, and project-doc workflow.

Use this file as the quick map. Read the focused operation file that matches the task:

| Task | Read |
|------|------|
| Install, bootstrap, and refresh | `.agent-harness/operations/install.md` |
| Release checklist and clean-machine install reproducibility | `.agent-harness/operations/release-reproducibility.md` |
| Release dogfood transcripts and observed release UX gaps | `.agent-harness/operations/release-dogfood-notes.md` |
| Codex/Claude native skills, MCP registration, lifecycle hooks | `.agent-harness/operations/hosts.md` |
| Direct CLI, daemon-backed MCP, command policy, guard, worker commands | `.agent-harness/operations/cli-and-mcp.md` |
| Web-fetch deterministic benchmark and opt-in live parity | `.agent-harness/operations/web-fetch-live-parity.md` |
| self-verify, self-augment, API documentation gates, smoke checks | `.agent-harness/operations/verification.md` |
| project bootstrap, project-doc routing, MCP document update rules | `.agent-harness/operations/project-docs.md` |

## Core Surfaces

1. Native skills: `atomic-commit-push`, `issueops`, `self-augment`, `project-bootstrap`, `self-verify`, `stability-audit`, plus the named specialist skills in `skills/`.
2. MCP stdio proxy: `agent-harness mcp` starts or connects to the shared user-level `agent-harness daemon`.
3. CLI: `agent-harness inspect/preflight/status/verify-work/doctor/docs/project/policy/guard/state/issueops/workpool/loop/daemon/worker/self-verify/self-augment/api-doc/hook`.
4. Loop contracts: `agent-harness loop start/record-attempt/status/stop` records verify-until-done state and strict readiness gates without executing verification commands.

## Daily Commands

```bash
agent-harness bootstrap --dry-run --json
agent-harness bootstrap
agent-harness project bootstrap --repo /path/to/repo --dry-run --json
agent-harness project bootstrap --repo /path/to/repo --sync --json
agent-harness doctor --repo . --json
agent-harness status --json
agent-harness docs --json
```

## State Store Maintenance

The sqlite-backed state stores accumulate WAL frames and sidecar files that need periodic checkpointing. Two surfaces handle this:

**Automatic (session-start hook):** `MaybeMaintainStateStores` runs WAL truncate + permission repair at most once per 24h via a `.last-store-maintain` sentinel in the state root. No user action needed.

**Manual CLI:**
```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json

# Clean up orphan session bindings (cycle done or absent) and stale cycles
agent-harness issueops cleanup stale --repo /path/to/repo --apply --prune-done 720h
```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows. Stale binding cleanup is destructive and gated behind `--apply`; without it, stale bindings are reported but not deleted.

## Release Smoke

```bash
scripts/release-repro-smoke.sh
```

Use `.agent-harness/operations/release-reproducibility.md` before deciding Homebrew, tarball, or other release packaging.

## Invariants

- Default install writes only user-level host configuration. Target repos get files only through explicit project bootstrap or project-local opt-in.
- Host adapters are thin wrappers around the same CLI/core behavior. They must not duplicate policy, schema, or state semantics.
- Hooks provide routing, lifecycle state, and bounded reminders only. They must not create issues/PRs, run tests, edit shared docs, or perform long network/file reads.
- IssueOps implementation must pass durable `compatibility_review` and `execution_decision` gates. Hooks do not decide backward compatibility, side effects, auto-proceed, human gates, or sub-agent usage; `issueops compatibility review` / MCP `issueops_record_compatibility_review` records the compatibility and side-effect judgement, then `issueops execution decide` / MCP `issueops_record_execution_decision` records the main-agent execution judgement before `implement`.
- Native install/update paths are standalone. External tools are neither installed nor required by `agent-harness`; use their own setup paths when a separate workflow needs them.
- Worker functionality remains policy-gated and state-first until write/network/background execution has explicit audit, timeout, cancellation, and redaction coverage.

## Optional Orca supervised execution

Orca is user-installed and optional. Preview with `agent-harness issueops worktree prepare --id ID --orchestrator auto --json`; add `--confirm` only after reviewing the resolved mode and path. `auto` preserves the existing inline flow when the read-only probe fails before mutation, while `--orchestrator orca` requires readiness and `--orchestrator inline` bypasses Orca.

For a resolved Orca path, follow `skills/issueops/references/orca-handoff.md`: coordinator `worktree prepare`/`handoff start`, fresh worker `handoff claim`/`issueops heartbeat`/`handoff finish`, coordinator `handoff accept`. On `recovery_required`, inspect `issueops status --json` and reconcile explicitly; never repeat create or switch to inline after an external mutation may have run.

Plan edits originate only from the exact source coordinator root; do not relay child-plan mutations through a feature or worker terminal. Preparation and dispatch use `issueops handoff start`. Raw terminal control is blocked for workers and non-source sessions. After claim, the source coordinator may send only single-line literal-safe `# agent-harness guidance:` to a uniquely matching persisted worker handle with exact `orca terminal send --terminal <handle> --text <payload> --enter --json`; ASCII C0 and DEL bytes are forbidden. The target hook is not a substitute for this source-side authorization.

Operational release evidence includes fake-runner recovery matrices, installed Codex/Claude/GJC ownership-block smokes, and one disposable live Orca cycle with per-resource cleanup receipts. Native installation and `self-verify` do not require Orca availability.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.
