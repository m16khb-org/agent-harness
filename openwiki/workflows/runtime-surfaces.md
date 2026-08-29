---
type: "Reference"
title: "Runtime Surfaces: CLI, MCP, Daemon, Worker, Hooks"
openwiki_generated: true
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T17:13:20.810Z
sources:
  - id: openwiki-source-d62612aa242cb096115ea93c
    resource: repo://.agent-harness/adr/decisions/2026-07-24-canonical-command-ah-shorthand.md
  - id: openwiki-source-ea052c1253074c3997aaf47f
    resource: repo://.agent-harness/adr/decisions/2026-08-10-default-hooks-thin-static-context.md
  - id: openwiki-source-9aa1149ffd536597c5fedda8
    resource: repo://.agent-harness/adr/decisions/2026-08-27-session-start-owns-compaction-context.md
  - id: openwiki-source-6205862704f1d06a5fb1ae3d
    resource: repo://cmd/harness/channelcli/channel.go
  - id: openwiki-source-6bdc7639e06c311468b3d34d
    resource: repo://cmd/harness/contractgolden/contract_golden_test.go
  - id: openwiki-source-f977714c44dfead8efbf8241
    resource: repo://cmd/harness/daemoncli/daemon_admission_test.go
  - id: openwiki-source-41dd8f7aa0e11c5301e2034b
    resource: repo://cmd/harness/daemoncli/daemon_admission.go
  - id: openwiki-source-83e546ff1a037f9857fcc730
    resource: repo://cmd/harness/daemoncli/daemon_identity.go
  - id: openwiki-source-3996975706c6c5f6ad4b92be
    resource: repo://cmd/harness/daemoncli/daemon_proxy_session_test.go
  - id: openwiki-source-02ee90a3186d5816fec35c79
    resource: repo://cmd/harness/daemoncli/daemon_proxy_test.go
  - id: openwiki-source-7547bacc40580875dcf1f46b
    resource: repo://cmd/harness/daemoncli/daemon_proxy.go
  - id: openwiki-source-5cd11c683a4ffe738e0da4fe
    resource: repo://cmd/harness/daemoncli/daemon_server.go
  - id: openwiki-source-83d371ff1b7137e207ccd5cd
    resource: repo://cmd/harness/daemoncli/daemon_start.go
  - id: openwiki-source-b0f7d17a9c351d728fc4e6f3
    resource: repo://cmd/harness/daemoncli/daemon_status.go
  - id: openwiki-source-1a7f8717157eb40c8f71ae59
    resource: repo://cmd/harness/daemoncli/daemon.go
  - id: openwiki-source-64e183d44f294c43fc789a1e
    resource: repo://cmd/harness/daemoncli/daemonlock/lock.go
  - id: openwiki-source-2269686a6a981ef0eceb3752
    resource: repo://cmd/harness/daemoncli/daemonpaths/instance.go
  - id: openwiki-source-5dac37a40f9db429ffba74cc
    resource: repo://cmd/harness/daemoncli/daemonpaths/paths.go
  - id: openwiki-source-679efdb78dbe316092265219
    resource: repo://cmd/harness/harnessapp/daemon_facade.go
  - id: openwiki-source-b4b461407e4468b621b5439c
    resource: repo://cmd/harness/harnessapp/mcp_command_facade.go
  - id: openwiki-source-a4dd3a647d29033677a0b06b
    resource: repo://cmd/harness/harnessapp/mcp_concurrency_test.go
  - id: openwiki-source-fc97803c25e6aaf9664c2899
    resource: repo://cmd/harness/harnessapp/mcp_facade.go
  - id: openwiki-source-9e37307cec8ebb29091064ff
    resource: repo://cmd/harness/harnessapp/root_command_facade.go
  - id: openwiki-source-e0ece59f5f9ea275969a37f3
    resource: repo://cmd/harness/harnessapp/worker_wiring.go
  - id: openwiki-source-8cee7d9c4c409e2f75b6c931
    resource: repo://cmd/harness/hookcli/hook_catalog_test.go
  - id: openwiki-source-8bcf74991ee3252a16bc3334
    resource: repo://cmd/harness/hookcli/hook.go
  - id: openwiki-source-092c7f3dd75fc49176e014f3
    resource: repo://cmd/harness/hookcli/hookcatalog/catalog.go
  - id: openwiki-source-d61ee1af1f93c99dbfb117ab
    resource: repo://cmd/harness/hookcli/hookenv/env.go
  - id: openwiki-source-99d2318ebf9ee6fbc7cd2f4b
    resource: repo://cmd/harness/hookcli/hookinput/hook_input.go
  - id: openwiki-source-cc1a832b00448ec51a87fc31
    resource: repo://cmd/harness/installcli/install_native_path.go
  - id: openwiki-source-4d6d4997a2e667fb6e6a7c29
    resource: repo://cmd/harness/main.go
  - id: openwiki-source-8e4a7fe437c124f43960f8c1
    resource: repo://cmd/harness/mcpcli/mcp_daemon_logfilter.go
  - id: openwiki-source-4f77575528e43db1ba98e102
    resource: repo://cmd/harness/mcpcli/mcp_sdk_server.go
  - id: openwiki-source-d7459c384ef9ebb7eba8f20c
    resource: repo://cmd/harness/mcpcli/mcp_tool_assistant_worker.go
  - id: openwiki-source-561a7de88618fbffc10df36d
    resource: repo://cmd/harness/mcpcli/mcp_tools.go
  - id: openwiki-source-8a70bdb6f9b911cb36256c6a
    resource: repo://cmd/harness/mcpcli/mcp_transport.go
  - id: openwiki-source-31d671ca79f515c15697d5af
    resource: repo://cmd/harness/rootcmd/root_command_test.go
  - id: openwiki-source-e1dc18775504626258612f4b
    resource: repo://cmd/harness/rootcmd/root_command.go
  - id: openwiki-source-41df5b9da259ad094d84bd77
    resource: repo://cmd/harness/testdata/mcp_tools.golden.json
  - id: openwiki-source-de0df019b7c74489768ef0ac
    resource: repo://cmd/harness/workercli/worker_dependencies.go
  - id: openwiki-source-b81a8f09d37eab6ad13bb25a
    resource: repo://cmd/harness/workercli/worker_test.go
  - id: openwiki-source-8abf398006dffb0f4876451c
    resource: repo://cmd/harness/workercli/worker.go
  - id: openwiki-source-4cf76bbc7d773373aa3947c6
    resource: repo://internal/adapter/hookprompt/catalog.go
  - id: openwiki-source-e58bc86ddc3e59bc18be4c49
    resource: repo://internal/adapter/policy/policy_run.go
  - id: openwiki-source-33c460260b93fb7c68c37061
    resource: repo://internal/adapter/worker/read_only.go
  - id: openwiki-source-e25ce8207d3653343a14b667
    resource: repo://internal/adapter/worker/store.go
  - id: openwiki-source-6ff6e4ad125ca0ca70428aa6
    resource: repo://internal/adapter/worker/worker_lock.go
  - id: openwiki-source-0e7055cdc28cfe7d87c16720
    resource: repo://internal/adapter/worker/worker.go
  - id: openwiki-source-11199c8c97ece0cbe1a61e4b
    resource: repo://internal/contract/policy/command_types.go
  - id: openwiki-source-f43a2646d3dfff930a7d4ea4
    resource: repo://internal/domain/cli/usage.go
  - id: openwiki-source-6e9fcf38cfb244d792963c41
    resource: repo://internal/domain/hook/output.go
  - id: openwiki-source-56597a0730aa3ea748051cf7
    resource: repo://internal/domain/mcp/catalog.go
generated: { by: "openwiki/0.4.3", at: "2026-08-29T17:13:20.810Z" }
---


# Runtime Surfaces: CLI, MCP, Daemon, Worker, Hooks

agent-harness exposes one host-neutral Go core through five runtime surfaces:
CLI one-shot commands, the `agent-harness mcp` stdio proxy, the user-level
`agent-harness daemon` that backs the proxy, the `agent-harness worker`
(policy-gated read-only jobs), and the two `agent-harness hook` catalog
commands that hosts run at session start and compaction. Every surface is
entered through the same binary and the same composition root, and every
surface's command vocabulary is pinned by goldens so the human, agent, and
contract views of the system cannot drift apart. Related pages:
[Architecture Overview](../architecture/overview.md),
[Response Contract Surface](../concepts/contract-surface.md),
[Safety & Command Policy](../operations/safety-and-policy.md),
[State, SQLite Store, and Locking](../concepts/state-and-sqlstore.md),
[IssueOps Cycle](issueops-cycle.md).

## The dispatch chain

Every invocation starts at the same three-hop chain:

```mermaid
flowchart TD
    MAIN["cmd/harness/main.go"] --> RR["harnessapp.RunRootCommand"]
    RR --> WIRE["wireDependencies sync.Once"]
    RR --> RUN["rootcmd.Command.Run"]
    RUN -->|"name in Runners"| SUB["leaf CLI facade"]
    RUN -->|"unknown name"| USAGE["usage text + exit 2"]
    WIRE --> BASIC["wireBasicCLIDeps"]
    WIRE --> HOST["wireHostCLIDeps"]
    WIRE --> POL["wirePolicyCLIDeps"]
    WIRE --> MCFG["configureMCPCLI"]
    RUN --> MCP["mcp"]
    RUN --> DMN["daemon"]
    RUN --> WRK["worker"]
    RUN --> HK["hook"]
    MCP --> PROXY["proxy or direct stdio"]
    PROXY --> DSOCK["daemon --internal over Unix socket"]
    WRK --> POLICY["command policy read-only run"]
    HK --> CATALOG["static project-doc catalog"]
```

*One `main.go`, one composition root, one runner map; the MCP, daemon, worker,
and hook commands are just rows in the same map with their own subcommand
dispatch.*

1. `cmd/harness/main.go` does nothing but call `harnessapp.RunRootCommand`
   with `os.Args[1:]` and exit with the returned code.
2. `RunRootCommand` calls `wireDependencies()` and then hands off to
   `rootcmd.Command.Run`. `wireDependencies` is guarded by `sync.Once` and
   calls an explicit, ordered list of `configure*` functions
   (`wireBasicCLIDeps`, `wireHostCLIDeps`, `wirePolicyCLIDeps`,
   `configureMCPCLI`) — the composition root wires by call, never by package
   `init()` side effects, so wiring is ordered, visible, and not
   import-order sensitive. A concurrency test pins that 80 parallel root
   commands do not rewire MCP dependencies out from under each other.
3. `rootcmd.Command` owns only dispatch, `help`/`version`, usage printing, and
   exit codes. Its `Runners` map in
   `cmd/harness/harnessapp/root_command_facade.go` holds 28 entries
   (`inspect`, `preflight`, `status`, `doctor`, `docs`, `policy`,
   `verify-work`, `trace`, `guard`, `quality`, `self-verify`,
   `self-augment`, `contract`, `state`, `issueops`, `api-doc`, `hook`,
   `project`, `install`, `update`, `bootstrap`, `worker`, `loop`, `gates`,
   `channel`, `web-fetch`, `daemon`, `mcp`); `version` is answered by
   `rootcmd` itself.

**The canonical command vocabulary lives in `internal/domain/cli`, not in the
router.** `cli.Commands()` returns the deterministic top-level catalog and
`cli.Usage(version)` renders the human usage text (with the `issueops` block
projected from the single `issueOpsUsageCatalog` — never hand-duplicated). The
`contractgolden` golden tests pin `usage.golden.txt`,
`mcp_tools.golden.json`, and `mcp_resources.golden.json`, so a new command or
tool must be added to the domain catalog *and* have its golden refreshed, and
the usage a user sees is byte-identical to the surface the contract checks
audit. The MCP tool vocabulary has the same discipline one level down:
`internal/domain/mcp/catalogSections()` is the single ordered source from which
both the advertised `tools/list` (51 tools, golden-pinned order) and the
name→handler `DispatchMap()` derive.

**`ah` is the installer-managed shorthand.** Native install creates
`~/.local/bin/agent-harness` plus a collision-safe `~/.local/bin/ah` symlink in
every PATH mode; `agent-harness` stays the canonical name in CLI output, MCP
configuration, and host adapters, while `ah update` works from any directory
([ADR 2026-07-24](../../.agent-harness/adr/decisions/2026-07-24-canonical-command-ah-shorthand.md)).

## Exit-code contract

`rootcmd.Command.Run` and the composition root's `ErrorExitCode` hook define
the contract every caller (scripts, CI, host hooks) can rely on:

| Code | Meaning |
| --- | --- |
| `0` | success; also `help`/`--help`/`-h`, `version`, and any subcommand answering `--help` by returning `flag.ErrHelp` (no `flag: help requested` noise is printed) |
| `1` | general failure — the default for a runner error |
| `2` | usage problems: no arguments, unknown top-level subcommand (usage printed to stderr), and `gates` usage errors (`gatescli.UsageError`) |
| `3` | denial by a core gate: `policy` when `policycontract.IsPolicyDenied(err)` matches, `guard` when `guard.IsGuardBlocked(err)` matches |

Two details are load-bearing. First, the mapping is centralized in
`rootSubcommandErrorExitCode` — leaf packages return typed errors
(`PolicyDeniedError`, `GuardBlockedError`, `UsageError`,
`channelcli.TimedOutError`, which is deliberately mapped to `1` because a
timed-out wait is a normal outcome, not a usage fault) and only the
composition root decides what they mean for the process exit status. Second,
a `policy run`/`worker run` denial surfaces the same code `3` that the policy
evaluation records in its own result payload (`exit_code: 3` with
`executed: false`), so the JSON body, the typed error, and the process status
agree.

## MCP stdio proxy and the user-level daemon

`agent-harness mcp` is the agent-facing surface. With no arguments it starts
the MCP runtime; `mcp cleanup [--dry-run|--apply]` terminates stale proxy
processes. Two serving modes exist:

- **Default (proxy):** ensure the daemon is running, dial its Unix socket, and
  pump newline-delimited JSON-RPC frames between stdin/stdout and the socket
  (`cmd/harness/daemoncli/daemon_proxy.go`). The proxy understands enough of
  the protocol to survive daemon restarts; it does not interpret tool calls.
- **`HARNESS_MCP_DIRECT=1`:** serve MCP directly over stdio with no daemon,
  using the same go-sdk server and the same dependency wiring — a one-shot,
  daemon-free mode for isolated use.

Both modes end in the same server: `mcpcli.initSDKServer*` builds an
`mcp.Server` from the official `modelcontextprotocol/go-sdk`, registers every
advertised tool and every resource, and validates each call's arguments
against the catalog `inputSchema` before dispatch (schema violations are
JSON-RPC `-32602`, not tool results). Handlers resolve through
`mcpadapter.DispatchMap()` into eight stable handler groups
(`project`, `policy_state`, `issueops`, `loop`, `gates`, `channel`,
`assistant_worker`, `self_loop`); `issueops_execution` alone gets a
context-aware handler carrying the `MCPDependencies` snapshot. That
dependency struct is fixed when a server is constructed — there is no
request-time package-global dependency cache — so concurrent MCP servers never
see each other's handlers, and a wiring change takes effect only in a newly
started server/daemon process. Tool-level failures (not-found, validation,
lock errors) are returned as `isError` text results mirroring the CLI's
`{ok:false}` bodies; only genuine JSON-RPC violations become protocol errors.

```mermaid
sequenceDiagram
    participant Host
    participant Proxy as mcp proxy session
    participant D1 as daemon generation A
    participant D2 as daemon generation B
    Host->>Proxy: initialize + notifications/initialized
    Proxy->>D1: forward
    D1-->>Proxy: initialize result
    Proxy-->>Host: initialize result
    Host->>Proxy: tools/call id 2
    Proxy->>D1: forward
    D1 --x Proxy: connection lost on restart
    Proxy-->>Host: id 2 error -32002 daemon_generation_changed
    Proxy->>D2: reconnect and replay initialize with fresh replay id
    D2-->>Proxy: initialize result, contract verified equal
    Proxy->>D2: replay notifications/initialized
    Proxy-->>Host: notifications/tools/list_changed and resources/list_changed
    Host->>Proxy: tools/call id 3
    Proxy->>D2: forward
    D2-->>Proxy: result id 3
    Proxy-->>Host: result id 3
```

*The proxy never replays an interrupted request's call: its outcome is unknown,
so it is failed explicitly and the host must reconcile.*

### Proxy session mechanics

The proxy tracks per-session state that makes reconnection safe:

- **Handshake caching.** The `initialize` request, the
  `notifications/initialized` notification, and the negotiated contract
  (canonicalized `protocolVersion` + `capabilities` JSON) are cached. On
  reconnect the proxy replays `initialize` under a fresh
  `agent-harness-reconnect-N` id with the negotiated protocol version and
  requires the new daemon's result to match the cached contract byte-for-byte;
  a mismatch aborts with "handshake contract changed" instead of silently
  serving a semantically different backend.
- **In-flight tracking.** Pending request ids (semantically canonicalized:
  `"a\u0062"` ≡ `"ab"`, `1.0` ≡ `1`, matching the SDK's numeric coercion) are
  recorded per session, batch-aware. When a connection dies, every pending
  request is answered before reconnecting.
- **`-32002` `daemon_generation_changed`, never auto-retried.** Interrupted
  requests receive `error.code -32002` with data
  `{code: "daemon_generation_changed", outcome: "unknown",
  automatic_retry: false, reconcile_required: true}`. The request may or may
  not have taken effect on the dying daemon, so the proxy refuses to guess:
  it does not replay the call to the new daemon, and the host is expected to
  reconcile (e.g. re-read IssueOps state) rather than blindly retry.
- **Bounded recovery.** Reconnect is capped at `daemonProxyReconnectAttempts`
  (48) tries within a 20-second budget with backoff capped at 250 ms; frames
  up to 4 MiB are supported; a saturated or unreachable daemon fails the
  proxy explicitly instead of producing empty output.
- **Post-reconnect refresh.** After a re-established session the proxy emits
  synthetic `notifications/tools/list_changed` and
  `notifications/resources/list_changed` to the host so stale tool caches are
  invalidated across the generation change.

### Daemon: identity, admission, idle reaping

`agent-harness daemon start|status|stop` manages the backend (the `--internal`
form, used by the launcher, runs the server loop). State lives under
`HARNESS_DAEMON_DIR`, else `<HARNESS_STATE_DIR>/daemon`, else
`~/.local/state/agent-harness/daemon`, holding `agent-harness.sock`, `.pid`
(the instance record), `.lock`, and `.log`.

- **Immutable-before-serving identity.** Before accepting, the server writes
  an instance record — PID, process start time, executable path, a
  crypto-random instance nonce, the executable's SHA-256 build hash, protocol
  version `"1"`, and a random generation token — atomically (temp file +
  rename, mode 0600). The socket is `chmod 0600`; stale lock/pid/socket files
  are cleaned up; startup is serialized by an `O_EXCL` lock file with
  stale-PID/age detection; shutdown waits up to 30 s for active connections.
- **Identity verification, not liveness folklore.** `daemon status` reads the
  instance record, then probes the socket with a **NUL-prefixed private
  request** (`\x00agent-harness-daemon-identity/1\n`) — a NUL first byte
  cannot begin a valid JSON-RPC message, so the probe can never be mistaken
  for MCP traffic and liveness checks never fabricate frames. The daemon
  answers with its record plus the admission snapshot. Status is `ready` only
  when record, probe, liveness, and OS process identity (start time, and
  executable where the platform reports it stably) all agree; anything else
  reports `instance_identity_mismatch`, `socket_unreachable`, or
  `instance_record_unreadable`. `daemon stop` refuses to signal an unverified
  process (SIGTERM, 3 s grace, then identity re-check before `Kill`), so a
  recycled PID can never be killed on the daemon's behalf.
- **Admission slots with a typed overflow error.** A slot channel caps
  concurrent MCP connections — default 256, `HARNESS_DAEMON_MAX_CONNECTIONS`
  (values outside 1..4096 fall back to the default). Each admitted connection
  gets a cancelable session context released on close; `draining` rejects new
  acquisitions. One extra "overflow classifier" slot is reserved so that when
  the pool is full a health-probe client can still be answered. An ordinary
  MCP connection that cannot get a slot receives a typed JSON-RPC error
  **`-32001`** ("daemon connection capacity exhausted") with data
  `{code: "daemon_connection_limit_reached", active_connections,
  max_connections, accepting, draining, retryable: true}`. The proxy checks
  the admission snapshot *before* dialing and refuses with the same
  classifier code when the daemon is not accepting, so saturation is
  observable rather than a hang.
- **Idle reaping.** Every connection is wrapped in an `idleConn` that resets a
  read deadline on each read — default 30 minutes, `HARNESS_MCP_IDLE_TIMEOUT`.
  An abandoned client therefore hits the deadline, the go-sdk `server.Run`
  returns, and the admission slot is freed instead of being pinned forever by
  a dead editor session.
- **Per-stream server.** The daemon serves each connection by building a
  go-sdk MCP server over the bidirectional socket with an immutable
  `MCPDependencies` value assembled by the composition root; go-sdk session
  routine events are downgraded to DEBUG in the daemon log (they were 99.9%
  of a 270k-line log in one measurement). Requests never mutate the
  dependency snapshot, so daemon wiring changes require a daemon restart —
  which is exactly the generation change the proxy is designed to surface.

`daemon start` is idempotent: `ensureDaemonRunning` reuses a ready daemon,
treats identity-mismatch states as blocking errors, and otherwise starts
`daemon --internal` as a detached (`Setsid`) child, then polls the identity
probe until ready (15 s budget).

## The worker: lifecycle records plus a policy-gated read-only run

To be precise about what the worker is: **there is no long-resident job
daemon.** `agent-harness worker` owns durable job *records* and a single
execution command; the shared daemon serves only the MCP proxy, and `worker
run` executes inside the invoking process.

- **Job records** live in the `worker` bucket of a SQLite store under
  `HARNESS_WORKER_DIR`, else `<HARNESS_STATE_DIR>/worker`, else
  `~/.local/state/agent-harness/worker`. `enqueue` creates a `queued` job with
  a deterministic id (`job-<UTC timestamp>-<12 hex sha256 of kind, payload,
  and time>`), a redacted payload (`policy.RedactFreeform`), and the standing
  safety notice that the MVP records lifecycle state only and never executes
  shell commands. `status`, `list` (with a queue histogram and depth),
  `cancel`, and `cleanup-stuck` read and mutate those records.
- **`worker run --read-only` is the only execution path.** The flag is
  mandatory. The job transitions `queued → running` under the store's span
  lock — recording the PID, the exact argv, `NoShell: true`, and the safety
  notice — then calls `policy.RunReadOnlyCommand`, which **forces write,
  network, and shell allowances off** before the same evaluation used
  everywhere else (`internal/adapter/policy/policy_run.go`), executes argv
  directly (no shell interpreter), with a 30 s default timeout, an
  env-allowlist filter, secret redaction, and a 32 KiB output budget per
  stream. Finally it transitions to `succeeded` or `failed` under the lock. A
  denied command is never executed and reports exit code 3 in the result
  payload; the CLI maps it to `PolicyDeniedError` and thus process exit 3; a
  timeout reports exit code 124.
- **Transitions are serialized.** Every read-modify-write span on the worker
  store takes the directory-wide sqlstore span lock, so `cancel` (legal only
  from `queued`) cannot race `run`'s transitions, and a crash mid-write can
  never leave a truncated record. `cleanup-stuck` finds `running` jobs whose
  PID is dead and re-marks them `failed` under the same lock with an explicit
  safety notice — the honest way a crashed runner is reconciled, since
  nothing else is watching.
- **The MCP tool reaches the same path.** `worker_run_read_only` (plus
  `worker_enqueue`, `worker_status`, `worker_list`, `worker_cancel`) is
  dispatched to the same `internal/adapter/worker` functions the CLI uses,
  injected by the composition root into both transports — the MCP tool is not
  a weaker side door.

```mermaid
stateDiagram-v2
    [*] --> queued: worker enqueue
    queued --> running: worker run --read-only under span lock
    queued --> cancelled: worker cancel
    running --> succeeded: allowed argv command exits 0
    running --> failed: denial, nonzero exit, or timeout
    running --> failed: cleanup-stuck finds dead PID
    succeeded --> [*]
    failed --> [*]
    cancelled --> [*]
```

*All transitions happen under the worker store's span lock; `run` is the only
transition that executes a process, and only in the caller's process.*

## Hooks: two catalog commands, nothing else

The current hook contract is deliberately minimal ([ADR
2026-08-10](../../.agent-harness/adr/decisions/2026-08-10-default-hooks-thin-static-context.md),
[ADR 2026-08-27](../../.agent-harness/adr/decisions/2026-08-27-session-start-owns-compaction-context.md)):
only two subcommands exist — `hook session-start` and `hook post-compact` —
and the enforcement-era surface (`user-prompt`, `pre-tool-use`,
`post-tool-use`, `pre-compact`, `stop`, failures/metrics ledgers, guard
chains) is deleted, not merely unregistered.

Both commands do exactly one thing: resolve the target repo (explicit
`--repo`, else the host's stdin JSON `cwd`/`repo`/`workspace` aliases, else
the harness target resolution), render the **static project-doc catalog** via
the `hookprompt` adapter (discover `.agent-harness` docs, format a compact
model-facing string and a readable user view), and print a host-compatible
JSON payload. They touch no durable state, write no telemetry, and carry no
IssueOps authority — a test drives both hooks across isolated worktrees and
asserts the harness state directory stays empty.

- **`session-start`** injects `hookSpecificOutput.additionalContext` for
  *every* SessionStart source (`startup`, `resume`, `clear`, and `compact`),
  because Claude Code and Codex both re-run SessionStart after compaction and
  only SessionStart output can carry model-facing context on those hosts.
  Claude additionally shows the readable catalog via `systemMessage`; Codex
  omits `systemMessage` and gets the readable view in `additionalContext`.
- **`post-compact`** keeps an explicit post-compaction surface for hosts
  without a SessionStart re-run (Omo's `session_compact`, which reads
  `--json`) and for diagnosis. Claude and Codex default installs do not
  register it, and neither host accepts model-facing output there, so the
  host shape carries only a user-facing `systemMessage`.
- **No docs, no output.** With no project docs both hooks print an empty JSON
  object rather than commentary.
- **Kill switch.** `HARNESS_DISABLE_HOOKS=1` turns either hook into a
  completely silent no-op — not even `{}` — so one host-level registration can
  stay installed while working in repositories the harness does not own.

Output formatting is host-polymorphic through `internal/domain/hook`
(`CodexHookOutput` vs `ClaudeHookOutput`), and the composition root injects
both the repo resolver and the catalog builder, keeping the hook packages
free of core behavior.

## Configuration and operations quick reference

| Variable | Surface | Effect |
| --- | --- | --- |
| `HARNESS_STATE_DIR` | daemon paths, worker dir | relocates the state root (default `~/.local/state/agent-harness`) |
| `HARNESS_DAEMON_DIR` | daemon | relocates socket/pid/lock/log |
| `HARNESS_DAEMON_MAX_CONNECTIONS` | daemon admission | concurrent MCP connection cap (default 256, hard range 1..4096) |
| `HARNESS_MCP_IDLE_TIMEOUT` | daemon | idle read deadline for connections (default 30m) |
| `HARNESS_MCP_DIRECT` | `mcp` | `1` serves stdio MCP without the daemon |
| `HARNESS_DISABLE_HOOKS` | `hook` | silent no-op for both catalog hooks |
| `HARNESS_WORKER_DIR` | worker | relocates job store |
| `HARNESS_ROOT` | daemon child, MCP root discovery | pins the harness checkout root |

Operational entry points: `daemon start|status|stop [--json]`,
`agent-harness status` / `doctor` (which include daemon identity and admission
state), and `mcp cleanup` for stale proxies. Health troubleshooting starts
with `daemon status --json`: its `code`
(`ready` / `stopped` / `socket_unreachable` / `instance_identity_mismatch` /
`instance_record_unreadable`) plus `active_connections`, `max_connections`,
`accepting`, and `draining` describe both identity and admission state in one
payload.

## Focused tests that pin this page

- `cmd/harness/main_test.go` — `main` exits with `run`'s code.
- `cmd/harness/rootcmd/root_command_test.go` — usage/version dispatch,
  unknown-subcommand exit 2, custom error exit codes, `flag.ErrHelp` → exit 0.
- `cmd/harness/contractgolden/contract_golden_test.go` — the usage and MCP
  catalog goldens that pin both vocabularies.
- `cmd/harness/daemoncli/daemon_admission_test.go` — slot accounting,
  draining, overflow-classifier probe, and the typed `-32001` rejection.
- `cmd/harness/daemoncli/daemon_proxy_test.go` /
  `daemon_proxy_session_test.go` — stream copying, reconnect-without-replay,
  `-32002` batch failure payload, handshake-contract change rejection,
  saturated-daemon pre-dial refusal.
- `cmd/harness/daemoncli/daemon_identity_test.go` /
  `daemon_ensure_test.go` / `daemon_server_loop_test.go` — probe/record
  agreement, ensure/start idempotence, accept-loop diagnostics.
- `cmd/harness/workercli/worker_test.go` and
  `internal/adapter/worker/*_test.go` — routing, read-only requirement,
  policy denial propagation, locked transitions, stuck-job repair.
- `cmd/harness/hookcli/hook_catalog_test.go` — host output shapes, compact
  source injection, empty-object noop, no-harness-state guarantee, and the
  `HARNESS_DISABLE_HOOKS` silent kill switch.
- `cmd/harness/harnessapp/mcp_concurrency_test.go` — concurrent root commands
  and MCP streams sharing immutable configuration.
