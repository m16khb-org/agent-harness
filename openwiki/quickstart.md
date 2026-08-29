---
type: quickstart
title: Quickstart
description: Entry point for engineers and coding agents — what agent-harness is, the five-minute build/install/verify commands, the agent-harness vs ah command identity, and a task-routed map into the rest of the wiki.
tags: [quickstart, onboarding, install, build, verification, cli, navigation]
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T17:13:20.810Z
sources:
  - id: openwiki-source-42b90bfa150819efc9065f4f
    resource: repo://.agent-harness/ARCHITECTURE.md
  - id: openwiki-source-d451a1fffcdc3985a6ac0105
    resource: repo://.agent-harness/OPERATIONS.md
  - id: openwiki-source-01a6ad22f88010223759f8c6
    resource: repo://.agent-harness/TESTING.md
  - id: openwiki-source-164e2da859b5277df81c7d94
    resource: repo://.github/workflows/ci.yml
  - id: openwiki-source-8037e2358a2c4f9b2c722a11
    resource: repo://AGENTS.md
  - id: openwiki-source-9e37307cec8ebb29091064ff
    resource: repo://cmd/harness/harnessapp/root_command_facade.go
  - id: openwiki-source-cc1a832b00448ec51a87fc31
    resource: repo://cmd/harness/installcli/install_native_path.go
  - id: openwiki-source-4d6d4997a2e667fb6e6a7c29
    resource: repo://cmd/harness/main.go
  - id: openwiki-source-e1dc18775504626258612f4b
    resource: repo://cmd/harness/rootcmd/root_command.go
  - id: openwiki-source-7bd911fdd3026b7b031a01e3
    resource: repo://go.mod
  - id: openwiki-source-03ffc32a0ca502ab67c54b25
    resource: repo://install.sh
  - id: openwiki-source-23775c3de52f3ab95a13cb8b
    resource: repo://README.md
generated: { by: "openwiki/0.4.3", at: "2026-08-29T17:13:20.810Z" }
---

# Quickstart

agent-harness is **one host-neutral Go core behind thin host adapters**. Codex,
Claude Code, and Omo native never embed harness behavior — they install
user-level skills, MCP registration, and lifecycle hooks that call the same
`agent-harness` binary, and a human shell reaches the same core through the
CLI. Five boundary commitments hold everywhere: core behavior lives in Go (not
in plugins or hooks), CLI JSON / MCP / daemon responses keep identical
semantics, adapters never bypass policy or workspace boundaries, hooks provide
only static project-doc context, and the worker stays policy-gated and
read-only. [Architecture Overview](architecture/overview.md) owns the topology,
enforcement, and extension points; this page gets you from a fresh clone to a
verified install, then routes you into the wiki by task.

## Requirements

- Git
- Go 1.26.3 (pinned in `go.mod`)
- Optionally, the host you plan to use: Codex, Claude Code, or Omo. Install,
  readiness, and self-verification are standalone — no host is required to
  build or verify, and external tools are never installed on your behalf.

## Five minutes: build, install, verify

### 1. Build the binary

```bash
go build -o bin/agent-harness ./cmd/harness
```

`cmd/harness` is the composition root. `main.go` delegates to
`harnessapp.RunRootCommand`, which wires concrete adapters into the leaf CLI
packages exactly once (`wireDependencies` under a `sync.Once`, ordered and
visible rather than `init()` side effects) and then dispatches through
`rootcmd.Command.Run`. If the build works, the whole layer graph compiled.

### 2. Preview and apply the install

```bash
./install.sh --dry-run --json   # review the complete change plan
./install.sh                    # apply it
```

`install.sh` builds `bin/agent-harness` (skip with `--skip-build`) and execs
`agent-harness install`; with no arguments in a real terminal it adds
`--interactive`. The dry run prints the full plan before anything changes. A
default install touches **only user-level host configuration** — command shims
under `~/.local/bin`, host skill links, MCP registration, and a `SessionStart`
hook. It writes nothing into a target repository unless you explicitly opt in
via `project bootstrap` or `--project-local` (which creates `.claude/skills/*`,
`.mcp.json`, `.omo/skills/*`, `.omo/mcp.json` — but never repo-local hook
registration). `install` also supports `--path-mode=auto|manual|skip`.

### 3. Verify the binary after install

```bash
./bin/agent-harness inspect --json
./bin/agent-harness doctor --repo . --json
```

`inspect` is the detailed installation and native-integration projection;
`doctor` diagnoses installation, state, hooks, MCP, daemon, and project docs
together. Day-to-day summaries use `ah status --json` and `ah docs --json`.

### 4. Check the public contract and docs index

```bash
agent-harness contract check --json
agent-harness docs --json
```

The response-contract schema pins **29 top-level CLI commands and 51 MCP
tools**; `contract schema --json` prints it, and `contract check --json`
verifies the running binary against it. Golden tests enforce the same surface
in CI, so a human, an agent, and a contract check cannot see different command
vocabularies. `docs --json` is itself a golden-pinned response — the project
docs index must stay load-bearing.

### 5. Run the verification battery

```bash
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
go test ./... -count=1
```

The self-verify gate is deterministic — pinned seed, LLM eval off, no model
required. `go test ./... -count=1` covers unit, fixture, golden, and contract
regressions. Steps 1 and 3–5 together are exactly the **minimum completion
gate** from `.agent-harness/TESTING.md`, which applies to every change
including docs-only edits; when a multi-step verification fails, earlier passes
do not count — rerun the whole battery from the first gate in a single run.
See [Testing & Verification Gates](testing/verification-gates.md).

### Daily refresh

```bash
git pull --ff-only
ah update
ah inspect --json
```

`ah update` rebuilds the current checkout and refreshes user-level
integrations; it never runs `git pull` itself.

## `agent-harness` vs `ah`

`agent-harness` is the canonical command; `ah` is the short symlink the
installer manages at `~/.local/bin/ah`. Installation **fails rather than
overwriting** an existing `ah` file or a foreign symlink — resolving that
conflict is a manual step. If `ah` is not found after install, open a new
shell or refresh your shell's command cache and confirm `~/.local/bin` is on
`PATH`. Both forms point at the checkout's `bin/agent-harness`, so `ah update`
works from any directory. Procedures and recovery live in the
[Operations Runbook](operations/runbook.md).

## Exit codes to script against

| Exit | Meaning |
| --- | --- |
| 0 | success (a `--help` request also exits 0) |
| 1 | generic subcommand failure, including a `channel` wait timeout |
| 2 | unknown command, missing args, usage error — and `gates` usage errors |
| 3 | command-policy denial (`policy`) or guard block (`guard`) |

## Where to go next

Route by task. Each linked page owns its topic; this page deliberately does
not duplicate them.

| Task | Start here |
| --- | --- |
| Understand the hybrid core/adapter structure, surfaces, and state semantics | [Architecture Overview](architecture/overview.md) |
| Place a change in the right package (and learn its forbidden imports) | [Source Map](architecture/source-map.md), [Dependency Ratchet](architecture/dependency-ratchet.md) |
| Decode a recurring term (execution lease, generation fence, phase, gate ledger) | [Domain Glossary](concepts/domain-glossary.md) |
| Understand the versioned CLI/MCP response contract and goldens | [Response Contract Surface](concepts/contract-surface.md) |
| Understand the state root, SQLite layout, and lock spans | [State, SQLite Store, and Locking](concepts/state-and-sqlstore.md) |
<!-- openwiki: broken internal link [workflows/execution-lease.md] file "workflows/execution-lease.md" does not exist. Fix the href or restore the target, then delete this comment. -->
| Start or advance an IssueOps cycle | [IssueOps Cycle Workflow](workflows/issueops-cycle.md), then [Execution Lease](workflows/execution-lease.md) for the generation-fenced execution state machine |
| Follow an invocation through CLI, MCP proxy, daemon, worker, and hooks | [Runtime Surfaces](workflows/runtime-surfaces.md) |
| Bootstrap and keep project docs fresh in a target repo | [Project Docs Workflow](workflows/project-docs.md) |
| Install into, or debug wiring for, Codex / Claude Code / Omo | [Host Integrations](integrations/hosts.md) |
| Reach GitHub/GitLab providers or the optional Orca execution adapter | [Providers & Orca Boundary](integrations/providers-and-orca.md) |
| Operate day to day (install, update, daemon recovery, state maintenance, release) | [Operations Runbook](operations/runbook.md) |
| Understand command policy, workspace fences, redaction, and the read-only executor | [Safety & Command Policy](operations/safety-and-policy.md) |
| Verify a change before declaring done | [Testing & Verification Gates](testing/verification-gates.md) |

For the repository's own agent-facing rules, the canonical internal owners are
root `AGENTS.md` (working contract, essential commands, invariants) and the
`.agent-harness/` family — `ARCHITECTURE.md`, `OPERATIONS.md`, and
`TESTING.md` are the indexes the wiki pages above mirror and route into. When
docs and code disagree, re-check the running binary (`contract check`,
`inspect --json`) and trust the code.
