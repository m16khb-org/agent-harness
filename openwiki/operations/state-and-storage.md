---
type: Operations
title: State and Storage
description: SQLite-backed state store with namespace isolation, dual-database locking via BEGIN IMMEDIATE, schema versioning, install/bootstrap flow, and host adapter pattern for Codex and Claude Code.
tags: [state, sqlite, install, bootstrap, host-adapters]
---

# State and Storage

agent-harness persists all durable state in SQLite databases under `~/.local/state/agent-harness/`. State is separated from the repository source, isolated by namespace, and serialized via cross-process locks.

## SQLite Store Architecture

Each state root directory owns two SQLite databases:

| Database | Purpose |
|----------|---------|
| `harness.db` | Stores all records as `(bucket, id, data-JSON)` rows |
| `harness.lock.db` | Exists solely to carry cross-process span locks via `BEGIN IMMEDIATE` |

Source: [`internal/core/sqlstore/sqlstore.go`](/internal/core/sqlstore/sqlstore.go).

### Locking Model

Locking is dual-layer:

- **In-process**: per-directory token gate (`spanGate chan struct{}`) ensures one active span at a time within a process.
- **Cross-process**: the span holds a `BEGIN IMMEDIATE` transaction on `harness.lock.db`, which auto-releases on process death (crash-safe, no deadlocks).

`StateUpdate` performs read-modify-write inside a single span. `WithKeyLock` exposes the same span to external callers. No Git, provider, or Orca process call runs while a cycle lock is held.

Source: [`internal/core/state/state_lock.go`](/internal/core/state/state_lock.go).

### Handle Caching

`sqlstore.Open()` caches one `*DB` handle per absolute directory path, so all callers in a process share the same in-process mutex.

## State Namespaces

| Namespace | Database | Bucket | Purpose |
|-----------|----------|--------|---------|
| General state | `harness.db` | `state` | Agent checkpoints (`state write/read/list`) |
| IssueOps v1 | `issueops_v1/harness.db` | `issueops_v1` | Issue-driven workflow records and execution |
| Loop | `loop/harness.db` | `loop` | Verify-until-done loop contracts |
| Project lifecycle | `projects/<repo-id>/` | file-based | Per-repo project profiles and doc-upkeep queues |

### Key Constraints

- Keys match `[A-Za-z0-9._-]`, max 128 characters. `/`, `\`, `..` are forbidden.
- Each `StateRecord` carries `SchemaVersion`, `Key`, `Content`, `UpdatedAt`, `Bytes`.
- `StateDoctor` validates integrity (byte-count, key-match, schema-version bounds) without modifying files.

Source: [`internal/core/state/state_io.go`](/internal/core/state/state_io.go), [`internal/core/state/state_types.go`](/internal/core/state/state_types.go).

### Schema Versioning

Current schema version is `1`. `StateMigrate` finds records at version 0 and upgrades them. Migration is dry-run by default; `--confirm` applies the rewrite.

IssueOps uses a different strategy: the schema version is baked into the bucket name (`issueops_v1`), so new versions get a physically separate namespace. Legacy authority fields are explicitly forbidden in v1 — their presence triggers a fail-closed error.

## State Store Maintenance

SQLite WAL frames and sidecar files accumulate and need periodic checkpointing:

- **Automatic**: `MaybeMaintainStateStores` runs WAL truncate + permission repair at most once per 24h via a `.last-store-maintain` sentinel.
- **Manual**: `agent-harness state maintain --json` checkpoints WAL and repairs sidecar permissions (read-only, does not delete rows).

IssueOps v1 lease recovery is **not** part of store maintenance and never happens from a time threshold.

## Install / Bootstrap / Update Flow

### InstallNative Orchestration

[`internal/core/install/install.go`](/internal/core/install/install.go) is the host-neutral install engine:

1. Validates shared inputs (root, home, codexHome, binPath)
2. Auto-discovers skills by scanning `<root>/skills/*/SKILL.md`
3. Iterates over all registered `HostInstaller` implementations
4. Aggregates results into `NativeInstallResult`

### What Gets Installed

- **Binary**: `<root>/bin/agent-harness`
- **Skill symlinks**: Host skill directories → repository `skills/` directories
- **MCP config**: Registers `agent-harness mcp` as a stdio MCP server
- **Lifecycle hooks**: Registers pre/post-tool-use hooks pointing at the binary

### Bootstrap and Update

Both `runUpdate` and `runBootstrap` delegate to [`scripts/install-native.sh`](/scripts/install-native.sh). Flags: `--project-local`, `--dry-run`, `--json`, `--skip-build`.

`ah update` builds from the current checkout and refreshes user-level integration. It does **not** run `git pull` — that is the user's responsibility.

## Host Adapter Pattern

Both Codex and Claude adapters implement the `port.HostInstaller` interface:

```go
type HostInstaller interface {
    Name() string
    Install(NativeInstallRequest) (HostInstallResult, error)
}
```

### Codex Adapter

| Surface | User-level path |
|---------|----------------|
| MCP config (TOML) | `~/.codex/config.toml` |
| Hooks (JSON) | `~/.codex/hooks.json` |
| Skill symlinks | `~/.codex/skills/<name>` → `<root>/skills/<name>` |

Codex writes a `[mcp_servers.agent_harness]` block into `config.toml`, idempotently removing prior harness sections. Hooks are merged into existing `hooks.json` with backup-on-first-write. No project-local mode for Codex.

Source: [`internal/adapter/codex/`](/internal/adapter/codex/).

### Claude Adapter

| Surface | User-level path | Project-local path |
|---------|----------------|-------------------|
| Settings (hooks) | `~/.claude/settings.json` | — |
| MCP config | `~/.claude.json` | `.mcp.json` |
| Skill symlinks | `~/.claude/skills/<name>` | `.claude/skills/<name>` |

Claude supports `--project-local` mode, which adds repo-relative skill symlinks and a `.mcp.json` with `HARNESS_ROOT: "."`.

Source: [`internal/adapter/claude/`](/internal/adapter/claude/).

Both adapters are **idempotent** — they remove their own prior sections before re-inserting, preserving user-configured content. Both support dry-run via a shared `installutil.Plan` builder.

## Project Lifecycle State

Project lifecycle state is isolated per-repo via `fingerprint.ForRoot()` → `fingerprint.RepoID()` → `~/.local/state/agent-harness/projects/<repoID>/`. This ensures repos on the same machine never mix state.

| File | Content |
|------|---------|
| `project.json` | `ProjectLifecycleProfile` (schema version, repo fingerprint, metadata) |
| `doc-upkeep-queue.jsonl` | Append-only doc maintenance events |

Namespace consistency is validated on every read (fingerprint equality + schema version match).

Source: [`internal/core/lifecycle/lifecycle_project_state_store.go`](/internal/core/lifecycle/lifecycle_project_state_store.go).

## Draft Wiki Staging

Long-term wiki candidates are staged in `.agent-harness/draft-wiki/{draft,approved,rejected}/` within the target repo. The main agent explicitly queues candidates via `agent-harness project draft-wiki queue --stdin`. The worker processes the queue using `agy -p` argv (no shell string) and writes drafts to `draft/`. Approved drafts are promoted via `promote --confirm` to `exported/`.

Hooks do not auto-judge draft-wiki value or append to the queue. Only `suggest` and `worker draft-wiki` call `agy -p`.

## Default Locations

| Kind | Location | Git-tracked |
|------|----------|-------------|
| Project knowledge | `.agent-harness/`, `AGENTS.md`, `CLAUDE.md` | Yes |
| User global config | `~/.config/agent-harness/config.yaml` | No |
| User global state | `~/.local/state/agent-harness/` | No |
| Workspace local cache | `.harness/` | `.gitignore` |
| Secrets | OS keychain or env reference | Never stored |
