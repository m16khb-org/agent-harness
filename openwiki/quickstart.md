---
type: Reference
title: agent-harness Quickstart
description: Entry point for the agent-harness code wiki. Overview of the project, core architecture, key commands, and links to all major documentation sections.
tags: [quickstart, overview, navigation]
---

# agent-harness Quickstart

**agent-harness** is a personal agent harness that lets multiple AI coding agents — Codex, Claude Code, and human shells — share the same Go core, CLI/MCP contract, command policy, user-state store, and skill sources. The goal is not to run agents more, but to ensure that any agent working in any host leaves the same decisions, respects the same safety boundaries, and judges completion with the same evidence.

The project is written in Go 1.26.3, ships as a single binary (`agent-harness`), and uses SQLite for durable state.

## What This Wiki Covers

| Section | What it explains |
|---------|-----------------|
| [Architecture Overview](architecture/overview.md) | Hybrid harness design, layer boundaries, execution modes, package structure |
| [Source Map](architecture/source-map.md) | Navigable map of the repository tree with key directories and files |
| [IssueOps Workflow](workflows/issueops.md) | The durable issue-driven work cycle: 9-phase state machine, readiness gates, phase ledger |
| [Execution Model](workflows/execution-model.md) | Direct vs Orca execution, write-lease state machine, generation fence, completion gate |
| [CLI and MCP Surface](operations/cli-and-mcp.md) | CLI command tree, MCP server/proxy/daemon architecture, tool-to-usecase mapping |
| [State and Storage](operations/state-and-storage.md) | SQLite namespaces, locking, schema versions, install/bootstrap, host adapters |
| [Policy, Guard, and Testing](operations/policy-guard-testing.md) | Command policy catalog, guard anti-pattern rules, testing conventions, golden contracts |

## Quick Commands

```bash
# First-time install (builds binary, symlinks skills, registers MCP/hooks)
./install.sh

# Diagnose install and state health
./bin/agent-harness inspect --json
./bin/agent-harness doctor --repo . --json

# Harness quality gate (deterministic, no external dependencies)
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json

# Update from current checkout
ah update

# CLI and MCP contract check
agent-harness --help
agent-harness contract schema --json
agent-harness contract check --json
```

## Core Concepts

The harness maintains a strict separation between **core logic** (Go, host-neutral) and **host adapters** (thin wrappers for Codex and Claude Code). The codebase follows a hexagonal architecture: domain logic lives in `internal/domain/`, application services in `internal/application/`, boundary adapters in `internal/adapter/`, and port interfaces in `internal/port/`. All three execution surfaces — CLI one-shot, daemon-backed MCP stdio proxy, and local worker — call the same [core use cases](architecture/overview.md), so the same input produces the same result regardless of host.

[IssueOps](workflows/issueops.md) is the central workflow engine: it moves task context out of conversation and into a durable issue→plan→worktree→feedback→verification cycle that survives across sessions and hosts. Every phase transition passes a fail-closed readiness gate.

The [command policy](operations/policy-guard-testing.md) is a policy-check-only system (not a real shell runner) that classifies commands into read-only, write, network, and shell tiers before execution. The [guard](operations/policy-guard-testing.md) catches code-level anti-patterns like non-deterministic tests, ambiguous test names, and secrets in staged files.

[State](operations/state-and-storage.md) lives in SQLite under `~/.local/state/agent-harness/`, separated by namespace (general state, IssueOps v1, loop, project lifecycle). All writes are serialized via `BEGIN IMMEDIATE` transactions on a lock database. The install system detects [build-generation skew](operations/state-and-storage.md#install-generation-skew-detection) between installed hooks and the running CLI.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| External Go core + thin host adapters | A Codex-plugin-only or Claude-hook-only approach cannot be shared. An external binary serves both. |
| Hexagonal package architecture | Domain, application, adapter, and port layers enforce dependency direction — adapters depend on ports, never the reverse. |
| Single binary, three surfaces | CLI, MCP proxy, and daemon all call the same core — identical semantics, testable. |
| Policy check, not shell runner | The most dangerous capability (command execution) starts as a policy gate, not a real executor. |
| IssueOps as single workflow authority | IssueOps holds durable authority over execution; Orca and other adapters are optional. |
| SQLite user-state, repo-separated | State lives outside the repo, isolated by repo fingerprint hash. |

Detailed rationale is in [`.agent-harness/ADR.md`](/.agent-harness/ADR.md) and [`.agent-harness/CONSTITUTION.md`](/.agent-harness/CONSTITUTION.md).

## Pioneer Skills

The `skills/` directory is the single source of truth for 20 skills shared across hosts. Each is named after a computing pioneer whose insight defines the skill's methodology. Key examples:

- **Execution hub**: `turing` (evidence-bound execution), `issueops` (workflow orchestration)
- **Planning**: `von-neumann` (strategic planning), `brooks` (devil's-advocate critic)
- **Investigation**: `berners-lee` (web research), `hopper` (systematic debugging)
- **Quality**: `shannon` (signal-to-noise measurement), `karpathy` (prompt engineering)
- **Git/ops**: `torvalds` (git operations), `atomic-commit-push`, `self-verify`, `stability-audit`

Skills are language/tech agnostic and installed via symlinks from host skill directories to the repo `skills/` source.

## Verification

```bash
# Full Go test suite
go test ./... -count=1
go test -race ./... -count=1

# Build
go build -o bin/agent-harness ./cmd/harness

# Contract golden
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1

# Architecture ratchet (errors.AsType enforcement, dependency direction)
go test ./internal/architecture -count=1
```

Change-type-specific verification is detailed in [Policy, Guard, and Testing](operations/policy-guard-testing.md) and [`.agent-harness/TESTING.md`](/.agent-harness/TESTING.md).

## Backlog

| Area | Source anchor | Reason deferred |
|------|--------------|----------------|
| GitHub/GitLab provider deep-dive | `internal/adapter/provider/`, `internal/port/provider.go` | Covered at summary level in [Execution Model](workflows/execution-model.md); full provider contract page deferred |
| Orca adapter 4-stage pipeline detail | `internal/adapter/orca/execution.go` | Boundary and mode selection covered in [Execution Model](workflows/execution-model.md); full Orca operational runbook deferred |
| Web-fetch and remote artifact gates | `internal/adapter/`, `internal/domain/webfetch/`, `scripts/remote-artifact-gate-smoke.sh` | Referenced in operations; full page deferred |
| Draft-wiki promotion workflow | `internal/adapter/worker/`, `.agent-harness/draft-wiki/` | Summarized in [State and Storage](operations/state-and-storage.md); full page deferred |
| Operational health and doctor internals | `internal/domain/operationalhealth/`, `internal/adapter/operationalhealth/` | Referenced in CLI/MCP; full health classifier page deferred |
