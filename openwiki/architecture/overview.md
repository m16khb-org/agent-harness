---
type: architecture-overview
title: Architecture Overview
description: One host-neutral Go core with thin Codex/Claude Code/Omo adapters, exposed through CLI one-shot, MCP stdio proxy, daemon, and worker execution surfaces under five fixed boundary commitments.
tags: [architecture, hexagonal, go-core, adapters, mcp, daemon, worker, hybrid]
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T17:13:20.810Z
sources:
  - id: openwiki-source-283ed105c07bd52f678e1f2b
    resource: repo://.agent-harness/ADR.md
  - id: openwiki-source-b661742f8e2b001be7622a60
    resource: repo://.agent-harness/adr/decisions/2026-05-25-go-language-selection.md
  - id: openwiki-source-cbfffe1db69c0bf2d44b8260
    resource: repo://.agent-harness/adr/decisions/2026-05-25-plugin-vs-external-worker.md
  - id: openwiki-source-ea052c1253074c3997aaf47f
    resource: repo://.agent-harness/adr/decisions/2026-08-10-default-hooks-thin-static-context.md
  - id: openwiki-source-a22a17c09030869babcf857f
    resource: repo://.agent-harness/adr/decisions/2026-08-12-omo-native-first-party-host.md
  - id: openwiki-source-42b90bfa150819efc9065f4f
    resource: repo://.agent-harness/ARCHITECTURE.md
  - id: openwiki-source-8d31c78479f6d54f47812b54
    resource: repo://.agent-harness/architecture/hexagonal-core.md
  - id: openwiki-source-225198edf50314e777287d8b
    resource: repo://.agent-harness/architecture/host-integration.md
  - id: openwiki-source-9be88b82096f247b8b24dc5f
    resource: repo://.agent-harness/architecture/issueops.md
  - id: openwiki-source-e31e8beb2f56c36939086f18
    resource: repo://.agent-harness/architecture/runtime.md
  - id: openwiki-source-8037e2358a2c4f9b2c722a11
    resource: repo://AGENTS.md
  - id: openwiki-source-6bdc7639e06c311468b3d34d
    resource: repo://cmd/harness/contractgolden/contract_golden_test.go
  - id: openwiki-source-41dd8f7aa0e11c5301e2034b
    resource: repo://cmd/harness/daemoncli/daemon_admission.go
  - id: openwiki-source-83e546ff1a037f9857fcc730
    resource: repo://cmd/harness/daemoncli/daemon_identity.go
  - id: openwiki-source-7547bacc40580875dcf1f46b
    resource: repo://cmd/harness/daemoncli/daemon_proxy.go
  - id: openwiki-source-5cd11c683a4ffe738e0da4fe
    resource: repo://cmd/harness/daemoncli/daemon_server.go
  - id: openwiki-source-1a7f8717157eb40c8f71ae59
    resource: repo://cmd/harness/daemoncli/daemon.go
  - id: openwiki-source-5dac37a40f9db429ffba74cc
    resource: repo://cmd/harness/daemoncli/daemonpaths/paths.go
  - id: openwiki-source-679efdb78dbe316092265219
    resource: repo://cmd/harness/harnessapp/daemon_facade.go
  - id: openwiki-source-1eb0bc0c0af2651c30aad2a3
    resource: repo://cmd/harness/harnessapp/install_wiring.go
  - id: openwiki-source-5c58197e97da783a8d01647b
    resource: repo://cmd/harness/harnessapp/response_contract_golden_test.go
  - id: openwiki-source-9e37307cec8ebb29091064ff
    resource: repo://cmd/harness/harnessapp/root_command_facade.go
  - id: openwiki-source-8bcf74991ee3252a16bc3334
    resource: repo://cmd/harness/hookcli/hook.go
  - id: openwiki-source-092c7f3dd75fc49176e014f3
    resource: repo://cmd/harness/hookcli/hookcatalog/catalog.go
  - id: openwiki-source-4d6d4997a2e667fb6e6a7c29
    resource: repo://cmd/harness/main.go
  - id: openwiki-source-4f77575528e43db1ba98e102
    resource: repo://cmd/harness/mcpcli/mcp_sdk_server.go
  - id: openwiki-source-8a70bdb6f9b911cb36256c6a
    resource: repo://cmd/harness/mcpcli/mcp_transport.go
  - id: openwiki-source-bd9ed29c5293259c0fd3216a
    resource: repo://cmd/harness/mcpcli/resources/resources.go
  - id: openwiki-source-e1dc18775504626258612f4b
    resource: repo://cmd/harness/rootcmd/root_command.go
  - id: openwiki-source-41df5b9da259ad094d84bd77
    resource: repo://cmd/harness/testdata/mcp_tools.golden.json
  - id: openwiki-source-7bd911fdd3026b7b031a01e3
    resource: repo://go.mod
  - id: openwiki-source-66db0c9308e3d4b796d76037
    resource: repo://internal/adapter/claude/install.go
  - id: openwiki-source-2d27cd56ee6e55188317c4ea
    resource: repo://internal/adapter/codex/install_hooks.go
  - id: openwiki-source-7d4f2b30ea7da36e9f3f3139
    resource: repo://internal/adapter/codex/install.go
  - id: openwiki-source-b6ebee518991653bf5cb3f24
    resource: repo://internal/adapter/install_contract_matrix_test.go
  - id: openwiki-source-e5a9db04ba86d33ec5e11a29
    resource: repo://internal/adapter/install/install.go
  - id: openwiki-source-f4c56339242c84012d1f5433
    resource: repo://internal/adapter/issueops/execution_namespace_test.go
  - id: openwiki-source-ef114713d53bbd681676b3f1
    resource: repo://internal/adapter/omo/extension.go
  - id: openwiki-source-c29c577ac03714f40253f007
    resource: repo://internal/adapter/omo/install.go
  - id: openwiki-source-1bb7e294c7243e8798131d47
    resource: repo://internal/adapter/outbound/sqlstore/sqlstore.go
  - id: openwiki-source-e58bc86ddc3e59bc18be4c49
    resource: repo://internal/adapter/policy/policy_run.go
  - id: openwiki-source-33c460260b93fb7c68c37061
    resource: repo://internal/adapter/worker/read_only.go
  - id: openwiki-source-e25ce8207d3653343a14b667
    resource: repo://internal/adapter/worker/store.go
  - id: openwiki-source-0e7055cdc28cfe7d87c16720
    resource: repo://internal/adapter/worker/worker.go
  - id: openwiki-source-b78b8f957dae0c4e1dae1fcc
    resource: repo://internal/architecture/dependency_test.go
  - id: openwiki-source-f43a2646d3dfff930a7d4ea4
    resource: repo://internal/domain/cli/usage.go
  - id: openwiki-source-56597a0730aa3ea748051cf7
    resource: repo://internal/domain/mcp/catalog.go
  - id: openwiki-source-a4b853cf3b3dff0668c7171b
    resource: repo://internal/domain/pioneerskill/catalog.go
  - id: openwiki-source-c0418c35a633373a6a133212
    resource: repo://internal/port/install.go
  - id: openwiki-source-4e1998b79639c789b2cdeef3
    resource: repo://README.en.md
generated: { by: "openwiki/0.4.3", at: "2026-08-29T17:13:20.810Z" }
---

# Architecture Overview

agent-harness is a **hybrid**: a single host-neutral Go core (`cmd/harness` +
`internal/...`) surrounded by thin adapters for each AI coding host. The three
first-party hosts — Codex, Claude Code, and Omo native — never embed harness
behavior; they install user-level skills, MCP registration, and lifecycle
hooks that call the same `agent-harness` binary. A human shell reaches the
same core through the CLI. The core exposes four execution surfaces: CLI
one-shot, `agent-harness mcp` stdio proxy, `agent-harness daemon` (the MCP
backend), and `agent-harness worker` (constrained local jobs).

Related pages: [Dependency Ratchet](dependency-ratchet.md),
[Source Map](source-map.md), [Contract Surface](../concepts/contract-surface.md),
[Hosts](../integrations/hosts.md), [IssueOps Cycle](../workflows/issueops-cycle.md),
<!-- openwiki: broken internal link [../workflows/execution-lease.md] file "../workflows/execution-lease.md" does not exist. Fix the href or restore the target, then delete this comment. -->
[Execution Lease](../workflows/execution-lease.md),
[Runtime Surfaces](../workflows/runtime-surfaces.md).

## Decision lineage

The structure is fixed by three accepted ADRs (full arguments live in the
records, not here):

- **External core + thin adapters** ([ADR 2026-05-25](../../.agent-harness/adr/decisions/2026-05-25-plugin-vs-external-worker.md)):
  a Codex plugin-only or Claude hook-only core cannot be shared across hosts,
  so the core is an external binary that both call; plugins/hooks are install
  UX and command wrappers only. CLI one-shot + MCP stdio came first; the
  worker/daemon followed when shared state actually needed them.
- **Go** ([ADR 2026-05-25](../../.agent-harness/adr/decisions/2026-05-25-go-language-selection.md)):
  single-binary distribution, fast iteration, and CLI/daemon productivity beat
  Rust's stricter safety for a personal harness.
- **Omo native as the third first-party host** ([ADR 2026-08-12](../../.agent-harness/adr/decisions/2026-08-12-omo-native-first-party-host.md)):
  install/update/readback evidence covers all three hosts in one deterministic
  order, instead of letting Omo drift to partial auto-discovery. Omo mutations
  bind to `PI_SESSION_ID` plus a live process receipt; the persistent `senpi`
  runtime receipt is accepted as the Omo process identity only for `host=omo`,
  and a session ID alone never authorizes a lease.

## Topology

<!-- openwiki: mermaid parse failed and this diagram was converted to a text fence so it does not break rendering. Fix the diagram source and restore the mermaid fence. Parser error: Heuristic: an unescaped angle bracket inside a label breaks rendering; rephrase the label. -->
```text
flowchart TD
    subgraph HOSTS["Hosts"]
        CODEX["Codex"]
        CLAUDE["Claude Code"]
        OMO["Omo native"]
        HUMAN["Human shell"]
    end

    subgraph ADAPTERS["Thin host adapters"]
        INSTALL["native install adapters<br/>codex claude omo"]
        SKILLS["shared skills symlinks"]
        HOOKS["SessionStart context hooks<br/>project-doc catalog only"]
    end

    subgraph SURFACES["Execution surfaces"]
        CLI["CLI one-shot<br/>agent-harness COMMAND"]
        PROXY["mcp stdio proxy"]
        DAEMON["daemon<br/>user-level Unix socket"]
    end

    subgraph CORE["Host-neutral Go core"]
        LAYERS["contract domain application port<br/>CLI usage and MCP catalog"]
    end

    subgraph PLANES["Core-owned planes"]
        POLICY["command policy<br/>catalog audit read-only run"]
        STATE["SQLite user state<br/>harness.db plus lock spans"]
        ISSUEOPS["IssueOps v1<br/>generation-fenced Execution"]
        WORKER["worker plane<br/>policy-gated read-only jobs"]
    end

    CODEX --> INSTALL
    CLAUDE --> INSTALL
    OMO --> INSTALL
    INSTALL --> SKILLS
    INSTALL --> HOOKS
    HOOKS --> CLI
    HUMAN --> CLI
    CODEX --> PROXY
    CLAUDE --> PROXY
    OMO --> PROXY
    PROXY --> DAEMON
    CLI --> LAYERS
    PROXY --> LAYERS
    DAEMON --> LAYERS
    LAYERS --> POLICY
    LAYERS --> STATE
    LAYERS --> ISSUEOPS
    LAYERS --> WORKER
    WORKER --> POLICY
```

*Hosts reach the core through installed adapters or a shell; the MCP proxy
dials the daemon's Unix socket; the daemon, CLI, and proxy all call the same
core, which owns policy, state, IssueOps, and the worker plane.*

## The five boundary commitments

These are deliberate, stated in the README and enforced (or fail-closed) in
code:

1. **Core behavior belongs in Go** — never in a host plugin, skill, or hook.
2. **Identical semantics across CLI JSON, MCP responses, and daemon
   responses** — same DTOs, same meaning, pinned by goldens.
3. **Adapters never bypass policy** — host adapters cannot get around
   authentication, command policy, or workspace boundaries; there is no
   privileged adapter path.
4. **Hooks are context-only** — default installs register only
   `SessionStart`/compaction-context hooks that render the static project-doc
   catalog; they never block a tool call and never create issues/PRs, edit
   files, or run tests ([ADR 2026-08-10](../../.agent-harness/adr/decisions/2026-08-10-default-hooks-thin-static-context.md)).
5. **The worker is policy-gated read-only** — lifecycle job records plus
   read-only evidence commands; it is not a general writable shell runner.

## Execution surfaces

### CLI one-shot

`cmd/harness/main.go` delegates to `harnessapp.RunRootCommand`, which wires
dependencies once and hands `os.Args` to `rootcmd.Command`. The router owns
only dispatch, `help`/`version`, and exit codes (e.g. policy denial → 3); the
canonical command vocabulary and usage text live in
`internal/domain/cli.Usage`, so the human surface cannot drift from the
contract checks. Top-level commands: `inspect`, `preflight`, `status`,
`doctor`, `docs`, `policy`, `guard`, `quality`, `verify-work`, `trace`,
`contract`, `state`, `issueops`, `api-doc`, `hook`, `project`, `install`,
`update`, `bootstrap`, `worker`, `loop`, `gates`, `channel`, `web-fetch`,
`self-verify`, `self-augment`, `mcp`, `daemon`, `version` (29 total; `ah` is
the installer-managed short form).

### MCP stdio proxy

`agent-harness mcp` is the agent-facing surface. By default it **ensures the
daemon is running, dials the daemon's Unix socket, and pumps JSON-RPC frames
between stdio and the socket** (`cmd/harness/daemoncli/daemon_proxy.go`).
Setting `HARNESS_MCP_DIRECT=1` instead serves MCP directly over stdio for
one-shot, daemon-free use. The server itself is the official
`modelcontextprotocol/go-sdk`; its advertised tool list and dispatch routing
both derive from the single ordered `catalogSections()` table in
`internal/domain/mcp` (51 advertised tools, deterministic order pinned by a
golden).

A few aliases route without being advertised: `self_augment_history`,
`self_augment_compare`, and `self_augment_promote` mirror the
`self_verify_history/compare/promote` tools under self-augment-prefixed state
keys, and `catalogSections` marks their section `advertised: false` so the
tools/list stays free of duplicate-looking names.

The proxy is deliberately failure-explicit:

- It tracks in-flight request IDs per session. If the daemon generation
  changes mid-flight (restart, update), pending requests are answered with
  JSON-RPC error `-32002` (`daemon_generation_changed`, `outcome: unknown`,
  `automatic_retry: false`, `reconcile_required: true`) — never silently
  retried, because the outcome is genuinely unknown.
- Reconnection is bounded (`daemonProxyReconnectAttempts` with capped
  backoff); frames up to 4 MiB are supported.

### Daemon

`agent-harness daemon start|status|stop` manages a user-level backend at
`HARNESS_DAEMON_DIR`, or `<state root>/daemon`, or
`~/.local/state/agent-harness/daemon`. Key mechanics
(`cmd/harness/daemoncli/`):

- **Identity**: the server writes an instance record (PID, process start
  time, executable path, random nonce, executable SHA-256, protocol version,
  daemon generation) before serving; `status` verifies it with a
  NUL-prefixed private probe that cannot collide with valid JSON-RPC traffic,
  so liveness checks never fabricate MCP frames.
- **Admission**: a slot channel caps concurrent MCP connections (default 256;
  `HARNESS_DAEMON_MAX_CONNECTIONS`, hard cap 4096). Over-limit connections get
  a typed `-32001` admission error, and a single overflow slot is reserved so
  a classifier/status client can still be answered when the pool is full.
- **Idle reaping**: each connection's read deadline is refreshed on every
  read (`idleConn`, default 30 min, `HARNESS_MCP_IDLE_TIMEOUT`), so abandoned
  clients release their slot instead of blocking forever.
- **Immutable composition**: the daemon builds its MCP dependency set once
  before the listener starts and hands an immutable snapshot to every stream;
  wiring changes take effect only in a new daemon process.
- The socket is `chmod 0600`; stale lock/pid/socket files are cleaned up on
  start; shutdown waits up to 30 s for active connections.

### Worker

`agent-harness worker enqueue|run|status|list|cancel|cleanup-stuck` records
job lifecycle state and, for `run --read-only`, executes an argv command that
has passed command policy with **write, network, and shell forced off**
(`internal/adapter/worker`, `internal/adapter/policy/policy_run.go`). The MCP
tool `worker_run_read_only` reaches the same path. Job state transitions are
serialized under per-job locks so `cancel` cannot race `run`. There is no
long-resident job daemon — the daemon serves only the MCP proxy.

## Core structure and composition root

The core is layered hexagonally: `internal/contract` (versioned DTOs shared by
CLI/MCP/state), `internal/domain` (pure rules with no filesystem/process/DB
I/O), `internal/application` (capability use cases), `internal/port`
(capability interfaces speaking contract vocabulary), and
`internal/adapter/*` (concrete inbound/outbound implementations, including
the three host installers). `cmd/harness/harnessapp` is the **only**
composition root: it injects concrete adapters into leaf CLI/MCP/daemon/hook
packages through explicit `wireDependencies()` (not `init()` side effects).
The zero-legacy-baseline import-graph ratchet in `internal/architecture`
enforces these directions on every `go test` run — see
[Dependency Ratchet](dependency-ratchet.md) for the rules.

## Host adapters

Host installation is one host-neutral engine, `install.InstallNative`, that
normalizes inputs and skill lists and calls `port.HostInstaller`
implementations; the composition root passes exactly the three first-party
installers (Codex, Claude Code, Omo). Each adapter writes only its own
user-level surfaces:

| Host | User-level surfaces written by default |
| --- | --- |
| Codex | `~/.codex/skills/*` symlinks, `~/.codex/config.toml` MCP, `~/.codex/hooks.json` SessionStart hook |
| Claude Code | `~/.claude/skills/*` symlinks, user-scope MCP, `~/.claude/settings.json` SessionStart hook |
| Omo native | `~/.omo/agent/skills/*` symlinks, `~/.omo/mcp.json`, `~/.omo/extensions/agent-harness.js` (`session_start`/`session_compact`) |

Default install writes **nothing** into a target repository; repo-local files
(`.claude/skills/`, `.mcp.json`, `.omo/skills/`, `.omo/mcp.json`) are created
only with explicit `--project-local`. Activation commits only after strict
readback with filesystem identity evidence, and `install`/`update` cover all
three hosts in one deterministic order. Skills are a single host-neutral
source tree (`skills/`: 33 skills — the 12-name pioneer catalog fixed in
`internal/domain/pioneerskill` plus 21 operational skills; hosts get symlinks,
never copies).

## State, locks, and failure semantics

- One state root: `~/.local/state/agent-harness/` (override with
  `HARNESS_STATE_DIR`). Data lives in SQLite. Each capability store directory
  under the root owns exactly two files: `harness.db`, holding records as
  `(bucket, id, JSON)` rows (e.g. the `state` and `worker` buckets), and
  `harness.lock.db`, which exists only to carry cross-process lock spans.
- Every read-modify-write span holds a `BEGIN IMMEDIATE` transaction on the
  lock database for its duration, so the lock **dies with the process** — a
  crashed holder cannot deadlock later writers. Nested spans are rejected, and
  no Git/provider/process call runs while a cycle lock is held.
- Project lifecycle state is isolated per repo under `projects/<repo-id>/`;
  loop state in `loop/harness.db`; IssueOps v1 state in
  `issueops_v1/harness.db`.
- Durable schemas are **current-only and fail-closed**: IssueOps records
  accept exactly `schema_version=1` and reject missing/zero/future versions
  and legacy authority keys as a generic `invalid state`, with no conversion
  or promotion command. IssueOps v1 is the single execution authority: one
  record holds one generation-fenced `Execution`, and the trust boundary is
  the exact native actor (host, session/agent ID, process receipt, canonical
  cwd, lifecycle ID, generation) — not branch names or terminal handles.
- Contract stability is mechanical, not aspirational: golden tests pin the
  CLI usage text, MCP tool/resource catalogs, and full CLI+MCP response
  contracts (`cmd/harness/contractgolden`, `cmd/harness/harnessapp` response
  contract suite), and `agent-harness contract schema|check` exposes the
  compatibility surface to the running binary.

## Extension points

- **New host**: implement `port.HostInstaller` and add composition-root
  wiring. Domain/application policy must not be duplicated per host; the
  install contract matrix golden detects surface drift.
- **New capability**: add a contract package, a pure domain classifier, an
  application use case, and inbound/outbound adapters; register the MCP tool
  in one `catalogSections()` entry (adding a tool makes it both advertised
  and routable).
- **New command**: add a runner in the composition root and the canonical
  description in `internal/domain/cli`; the usage golden forces the two to
  agree.

## Operations quick reference

- Health: `agent-harness status`, `doctor` (install, state, hooks, MCP,
  daemon, project docs together), `inspect`, `daemon status`.
- Maintenance: `state maintain` (WAL checkpoint + sidecar permissions),
  `state prune` (dry-run by default, `--confirm` to delete),
  `mcp cleanup` for stale proxy processes.
- Environment: `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`,
  `HARNESS_DAEMON_MAX_CONNECTIONS`, `HARNESS_MCP_IDLE_TIMEOUT`,
  `HARNESS_MCP_DIRECT=1`, `HARNESS_DISABLE_HOOKS=1`.
- Contract checks: `agent-harness contract schema --json`,
  `contract check --json`; golden tests under
  `go test ./cmd/harness/contractgolden` and
  `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden`.

## Canonical sources this page summarizes

The `.agent-harness/ARCHITECTURE.md` family is authoritative; this page only
summarizes. Its four owned modules hold the line-level detail:

- `architecture/hexagonal-core.md` — dependency direction, package
  boundaries, cross-host tool contract, dependency ratchet summary.
- `architecture/runtime.md` — execution modes, daemon/MCP/worker surfaces,
  docs/state/config/log topology, command/policy model.
- `architecture/host-integration.md` — Codex/Claude/Omo integration map,
  shared pioneer skills layer, host-adapter change checklist.
- `architecture/issueops.md` — IssueOps v1 state/schema authority, execution
  threat model, actor model, Orca boundary.

Historical decisions live in `.agent-harness/adr/decisions/`; the
import-graph enforcement itself is documented in
[Dependency Ratchet](dependency-ratchet.md).
