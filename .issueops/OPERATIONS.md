---
name: OPERATIONS.md
description: Install, sync, runtime, and operational procedures.
---

# Operations Map

`issueops` gives Codex, Claude Code, and Omo native the same Go binary, MCP schema,
command policy, state store, shared skills, lifecycle hooks, and project-doc
workflow.

This file is the canonical index for the operations family. Read the focused
guide that matches the task.

## Guides in this family

| Task | Read |
|------|------|
| Install, bootstrap, refresh, daily commands, release smoke | [install-and-update.md](operations/guides/install-and-update.md) |
| CLI discovery, command policy, guard, state maintenance, contract conformance, quick smoke | [cli-and-state.md](operations/guides/cli-and-state.md) |
| Native skills, host hook rules, hook kill-switch | [skills-and-hosts.md](operations/guides/skills-and-hosts.md) |
| Health gate, diagnosis, one-time reconciliation | [troubleshooting.md](operations/guides/troubleshooting.md) |
| IssueOps provider publication, branch linkage, issue snapshots | [issueops-providers.md](operations/guides/issueops-providers.md) |
| IssueOps execution lifecycle, recovery, sync-base, owner sequence, ten-stage operation | [issueops-execution.md](operations/guides/issueops-execution.md) |

## Detailed references owned under `operations/`

These sibling references own their topics normatively. Link to them directly
instead of duplicating their content here or in a guide.

| Topic | Owner |
|-------|-------|
| First-run install, `io update`, command shims, MCP refresh | [operations/install.md](operations/install.md) |
| Direct CLI, policy, guard, state, loop, daemon, MCP cleanup, worker, audit | [operations/cli-and-mcp.md](operations/cli-and-mcp.md) |
| Codex/Claude/Omo native skills, MCP registration, lifecycle hooks | [operations/hosts.md](operations/hosts.md) |
| self-verify, self-augment, api-doc gate, general smoke | [operations/verification.md](operations/verification.md) |
| Project bootstrap, project-doc routing, MCP document updates | [operations/project-docs.md](operations/project-docs.md) |
| Release checklist, clean-machine smoke, build matrix, rollback | [operations/release-reproducibility.md](operations/release-reproducibility.md) |
| Release dogfood transcripts | [operations/release-dogfood-notes.md](operations/release-dogfood-notes.md) |
| Web-fetch deterministic benchmark and opt-in live parity | [operations/web-fetch-live-parity.md](operations/web-fetch-live-parity.md) |

## Core Surfaces

1. Native skills: `io-update`, `atomic-commit-push`, `issueops`, `self-augment`,
   `project-bootstrap`, `self-verify`, `stability-audit`, plus the named
   specialist skills in `skills/`. The IssueOps stage skills are
   `issueops-create-issue`, `issueops-prepare`, `issueops-plan`,
   `issueops-implement`, `issueops-clean`, `issueops-docs`, `issueops-verify`,
   `issueops-create-pr`, `issueops-complete`, `issueops-cleanup`, and
   `issueops-abandon`; the shared ones are `issueops-review`, `gates-ledger`,
   and `issueops-remote-write`. `issueops next` decides which stage
   a cycle is in and which command advances it.
2. MCP stdio proxy: `issueops mcp` starts or connects to the shared
   user-level `issueops daemon`.
3. CLI: 29 top-level commands (`install/update/bootstrap/version`,
   `inspect/preflight/status/doctor/docs`,
   `policy/guard/quality/verify-work/trace/contract/api-doc`, `project/hook`,
   `state/daemon/mcp/worker`, `issueops/loop/gates/channel`,
   `self-verify/self-augment/web-fetch`); `issueops --help` is the
   canonical list.
4. Loop contracts: `issueops loop start/record-attempt/status/stop` records
   verify-until-done state and strict readiness gates without executing
   verification commands.
5. Task gate ledgers: `issueops gates init/check/status/report/abandon`
   discovers per-issue `.issueops/issues/<n>/gates.md` first, then generic
   `.issueops/gates/*.md` and compatible `GATES.md`/`gates/*.md` ledgers.
   Generic `gates init` still defaults to `.issueops/gates/<scope-slug>.md`;
   IssueOps owns `.issueops/issues/<provider-issue-number>/gates.md` and
   judges only its own or anonymous ledgers for strict PR readiness.
   CHECK commands run through the command policy engine (never a raw shell),
   and unmet gates add `gates_incomplete:<file>`. See
   [operations/cli-and-mcp.md](operations/cli-and-mcp.md).
6. Cross-session channels: `issueops channel send/recv` gives Codex,
   Claude Code, and Omo sessions a durable shared mailbox over issueops state —
   the transport for front/server-style multi-session coordination.
   `recv --wait --since <id>` is the blocking consumer; MCP exposes
   `channel_send`/`channel_recv` with the same contract.

## Invariants

- Default install writes only user-level host configuration. Target repos get
  files only through explicit project bootstrap or project-local opt-in.
- Host adapters are thin wrappers around the same CLI/core behavior. They must
  not duplicate policy, schema, or state semantics.
- Default hooks provide only static project-doc context. They must not create
  issues/PRs, run tests, edit shared docs, read IssueOps state, perform
  maintenance, or perform long network/file reads.
- IssueOps implementation must pass durable design, compatibility,
  devil's-advocate, and execution v1 gates. Hooks do not decide compatibility,
  side effects, sub-agent usage, or lease ownership.
  `issueops execution prepare/status/claim/release/replace/reconcile/complete`
  and MCP `issueops_execution` are the single execution contract.
- Native install/update, readiness, and self-verification remain standalone.
  After native activation, the declarative `configs/upstream.json` catalog may
  provision missing Claude plugins and Git skills; it is dry-run visible,
  Claude-scoped, and non-fatal. Other companion tools keep their own setup paths.
- Worker functionality remains policy-gated and state-first until
  write/network/background execution has explicit audit, timeout, cancellation,
  and redaction coverage.

## Updating this family

1. Add focused detail to the guide that owns the responsibility.
2. Keep this index to navigation, the universal summary, and invariants only.
3. Every guide links back to `OPERATIONS.md`; cross-family rules link to their
   canonical owner, not a duplicate summary.
4. Run the documentation check before committing:

```bash
uv run --directory skills/project-docs-optimize python -m scripts.check \
  --root "$PWD" --mode check --json
```
