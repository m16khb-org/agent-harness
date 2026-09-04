# Research: Orca contract for an IssueOps worktree-session handoff

## TL;DR

IssueOps can safely use Orca as an **optional external orchestrator**, provided that issueops keeps its own durable execution record as the source of truth. Orca can create an isolated checkout, launch a fresh known agent, inject a task, persist task/message state, and expose worktree/terminal identities. It does not provide a documented automatic coordinator-loop resume contract, terminal handles are runtime-scoped, and its hook status is ephemeral. Therefore the integration must capability-probe before mutation, persist every acquired domain identity, refresh live terminal handles after restart, and fall back to the existing inline IssueOps path only before any Orca artifact has been created.

Snapshot: 2026-07-11. Public source was checked at official Orca commit [`fa4e0813`](https://github.com/stablyai/orca/commit/fa4e08136200b4ee98648c6b3a25dc2531cdba24); the latest official release observed was [v1.4.134](https://github.com/stablyai/orca/releases/tag/v1.4.134). The installed relay build was inspected separately because its version and output contract differ from the public release naming.

## Method

- Public research used only official Orca documentation, official skills, and the `stablyai/orca` source repository.
- Local verification used non-mutating `status`, `help`, `list`, `show`, and stale-handle probes plus the installed relay source. No worktree, terminal, task, message, gate, or coordinator run was created.
- Claims below distinguish documented contracts, current implementation details, installed behavior, and unresolved mutation-only behavior.

## Findings

### 1. Worktree creation matches the requested session boundary

`orca worktree create` creates a new checkout. `--agent <id>` launches a known agent in its first terminal, and `--prompt` sends initial work to that agent. Parent lineage can be inferred or selected explicitly, while `--base-branch` independently controls the Git base. Plain creation remains backgrounded unless activation is requested. These semantics directly support a coordinator that prepares an issue/branch/worktree and a fresh worker session that performs implementation.

Sources:

- [Orca CLI reference](https://www.onorca.dev/docs/cli/reference), retrieved 2026-07-11.
- [Official worktree command specification](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/cli/specs/core.ts#L84-L130), retrieved 2026-07-11.
- [Official worktree create handler](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/cli/handlers/worktree.ts#L104-L258), retrieved 2026-07-11.

The official create result exposes `result.worktree`; it does not promise a terminal handle. The documented worker flow reacquires the terminal using `terminal list --worktree id:<worktree-id>`. Domain identifiers must therefore be read from `result`, not the top-level RPC correlation `id`.

Sources:

- [Create result type](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/shared/runtime-types.ts#L654-L668), retrieved 2026-07-11.
- [Terminal list result type](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/shared/runtime-types.ts#L368-L435), retrieved 2026-07-11.
- [Official orchestration worker flow](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/skills/orchestration/SKILL.md#L185-L196), retrieved 2026-07-11.

### 2. Direct handoff and supervised orchestration are deliberately different

Orca's official skills distinguish two workflows:

1. A full ownership handoff creates the worktree/terminal, delivers the prompt, and stops.
2. Supervised work that needs monitoring, task lifecycle, dependencies, or a result join uses `task-create`, `dispatch --inject`, messages, and task status.

IssueOps needs the second form here because the coordinator must persist ownership, prevent duplicate execution, recover after interruption, validate the worker result, and retain PR/cleanup authority. The integration should describe this explicitly as a **supervised execution handoff**, not use orchestration commands merely because they exist.

Sources:

- [Official Orca CLI skill: ownership-handoff boundary](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/skills/orca-cli/SKILL.md#L42-L64), retrieved 2026-07-11.
- [Official orchestration skill: supervision boundary](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/skills/orchestration/SKILL.md#L54-L66), retrieved 2026-07-11.

### 3. Task and dispatch identities provide stale-worker protection

`task-create` accepts a task specification, title/display name, dependencies, and parent task. `dispatch` accepts only a ready task and creates a fresh dispatch context. `--inject` requires a recognized running agent and supplies the task plus lifecycle preamble.

Completion authority is the active `taskId + dispatchId + assignee handle` tuple. A worker completion message carries both IDs, so a stale worker cannot complete a newer retry. `dispatch-show` is the official inspection/recovery surface for the latest dispatch context. These identities complement, but do not replace, an IssueOps attempt token and context hash.

Sources:

- [Orca orchestration documentation](https://www.onorca.dev/docs/cli/orchestration), retrieved 2026-07-11.
- [Task creation implementation](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/main/runtime/orchestration/db.ts#L430-L467), retrieved 2026-07-11.
- [Dispatch RPC](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/main/runtime/rpc/methods/orchestration.ts#L427-L548), retrieved 2026-07-11.
- [Worker completion contract](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/skills/orchestration/SKILL.md#L217-L223), retrieved 2026-07-11.

### 4. Runtime-scoped handles must not become durable ownership keys

Official guidance says terminal handles are runtime-scoped and must be reacquired with `terminal list` after restart or `terminal_handle_stale`. Local verification found a stale `ORCA_TERMINAL_HANDLE` and a fresh list handle for the same `tabId`, `leafId`, and PTY. Terminal-control calls rejected the stale handle, while orchestration mailbox reads still accepted it as a recipient identity.

The durable handoff therefore needs separate fields for:

- Orca runtime ID;
- Orca worktree ID and worktree instance ID;
- mailbox/coordinator recipient handle used by persisted messages;
- current live terminal handle used for injection/control;
- stable-enough matching data such as worktree path, tab ID, leaf ID, PTY ID, and pane key.

The current live handle must be refreshed from the worktree before control operations. The old mailbox handle must be preserved for historical message recovery.

Sources:

- [Official CLI reference](https://www.onorca.dev/docs/cli/reference), retrieved 2026-07-11.
- [Official handle recovery guidance](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/skills/orca-cli/SKILL.md#L173-L180), retrieved 2026-07-11.
- Installed relay identity forwarding: `/Users/m16khb/.orca-remote/relay-0.1.0+b634e93f6a7c/relay.js:288-300`.

### 5. Readiness is capability-based, not version-string based

The installed wrapper exposes no reliable `--version`: it prints help, while `orca version` fails. Public source also shows that `status --json` can return `ok:true` while the runtime is unreachable. A valid readiness gate must check all of the following before any mutation:

1. an `orca` executable exists;
2. stdout is a valid JSON envelope;
3. `result.runtime.reachable` is true and the runtime state is `ready`;
4. the graph is ready where the installed contract exposes it;
5. required command capabilities are present;
6. the target repository/worktree can be resolved.

The installed wrapper uses exit 42 for a relay/app version mismatch and exit 1 for transport and ordinary command errors. JSON must be decoded from stdout only because handshake logs appear on stderr.

Sources:

- [Public status implementation](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/cli/runtime/status.ts#L7-L79), retrieved 2026-07-11.
- Installed wrapper: `/Users/m16khb/.orca-relay/bin/orca:1-10`.
- Installed relay handshake/forwarding: `/Users/m16khb/.orca-remote/relay-0.1.0+b634e93f6a7c/relay.js:7-15,298-302`.

This makes `auto` mode safe: a pre-mutation probe failure can select the existing inline workflow. Explicit `orca` mode should report the failed capability rather than silently change execution mode.

### 6. Orca persistence is useful, but IssueOps remains the source of truth

Orca persists orchestration tasks/messages in its own database, and ordinary app quit/update/crash can warm-reattach running PTYs. Host reboot, hard power loss, or daemon crash restores worktrees/layout but not agent processes. No public contract was found for automatically resuming an active `orca orchestration run` after runtime loss; current source exposes `run` and `run-stop`, but no resume/status verb, and the active coordinator object is memory-resident.

Sources:

- [Session restore documentation](https://www.onorca.dev/docs/model/session-restore), retrieved 2026-07-11.
- [Orchestration command specification](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/cli/specs/orchestration.ts#L112-L154), retrieved 2026-07-11.
- [Orchestration database location](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/main/runtime/orca-runtime.ts#L2724-L2733), retrieved 2026-07-11.
- [Coordinator-run implementation](https://github.com/stablyai/orca/blob/fa4e08136200b4ee98648c6b3a25dc2531cdba24/src/main/runtime/rpc/methods/orchestration-gates.ts#L7-L100), retrieved 2026-07-11.

Installed Orca agent-status hooks are also ephemeral: they key status by pane, keep it in memory, and clear it on pane/relay exit. IssueOps must not derive durable claim/completion state from those hooks.

Local evidence: `/Users/m16khb/.orca-remote/relay-0.1.0+b634e93f6a7c/relay.js:288-293`.

### 7. Fallback must stop at the first external mutation

The installed app's successful create/dispatch result shape, collision behavior, and retry deduplication could not be verified without mutation, and the remote app source was not installed locally. Public source establishes the general envelopes, but a compatible adapter still needs tolerant parsing and deterministic reconciliation selectors.

Safe boundary:

- If capability probing fails before create/task/terminal mutation, `auto` mode records a diagnostic and continues through the existing inline IssueOps workflow.
- Once any Orca worktree, terminal, task, dispatch, or message identity may exist, the harness must not start an inline worker. It records a recoverable failure and reconciles by deterministic worktree metadata, task title/display name, dispatch context, and refreshed terminal identity.

This boundary prevents duplicate agents from editing the same branch after a timeout where the external operation may actually have succeeded.

### 8. Host session identity, not a terminal handle, is the ownership fence

All three supported hosts expose a stable session identifier at their native hook boundary:

- Codex 0.144.1 hook inputs carry `session_id` (`ThreadId`) and `cwd`; turn-scoped events also carry `turn_id`.
- Claude Code 2.1.206 common hook inputs carry `session_id` and `cwd`.
- GJC 0.7.8 supplies `ctx.sessionManager.getSessionId()` and `ctx.cwd` to every first-party HookAPI handler.

Current issueops adapters discard that identity. The common parser reads only repository/cwd-like fields, IssueOps “session” bindings are actually keyed by repo or repo+cycle, and the installed GJC shim discards both the event and HookContext while spawning the harness with stdin ignored. GJC's `context` event also runs before each LLM call rather than exactly once per user prompt, so `before_agent_start` is the safer UserPromptSubmit analogue.

Sources:

- [Codex 0.144.1 hook schema](https://github.com/openai/codex/blob/rust-v0.144.1/codex-rs/hooks/src/schema.rs), retrieved 2026-07-11.
- [Claude Code hook reference](https://code.claude.com/docs/en/hooks), retrieved 2026-07-11.
- Installed GJC HookAPI: `/Users/m16khb/.bun/install/global/node_modules/@gajae-code/coding-agent/src/extensibility/hooks/types.ts:166-175,431-436`.
- Installed GJC session manager: `/Users/m16khb/.bun/install/global/node_modules/@gajae-code/coding-agent/src/session/session-manager.ts:343-383,3048-3064,3422-3440`.
- Current harness parser: `cmd/issueops/hookcli/hookinput/hook_input.go:8-28`.
- Current IssueOps binding: `internal/core/issueops/session/session.go:30-37,48-59,173-202`.
- Current GJC shim: `gjc-plugin/hook.ts:31-74`.

The ownership key should therefore be composite:

```text
owner      = host + host_session_id + optional agent_id
scope      = cycle_id + canonical_repo + expected_worktree
handoff    = ownership_epoch + attempt_token + context_sha256
orca loc   = workspace_id + worktree_id + worktree_instance_id + tab_id
orca route = refreshed live_terminal_handle
role       = coordinator | worker
```

The new worker claims only when its native session ID, canonical cwd/worktree, injected attempt token, and Orca worktree locator all agree. A cleared/restarted host session must claim again even if the terminal/worktree is unchanged. When Orca is absent, the Orca locator is empty and legacy repo/worktree rules remain available for inline mode.

## Implications for issueops

- Add a thin optional Orca process adapter; do not install, update, or emulate Orca.
- Keep the core DTO host-neutral and model a coordinator/worker execution lease independently of Orca output envelopes.
- Extend hook input and IssueOps binding state with native host session identity; keep legacy repo bindings as a backward-compatible inline fallback.
- Repair the GJC shim to forward event/context payloads and honor blocking tool-hook results before relying on cross-host ownership enforcement.
- Use a bounded, redacted, versioned context packet with a SHA-256 over stable IssueOps intent/design/plan/acceptance data. Do not copy raw transcripts or local secret-bearing configuration.
- Persist an attempt token before mutation; reconcile partial success instead of replaying blindly.
- Gate implementation entry on a valid worker claim only for resolved Orca mode. A missing handoff field on legacy records remains inline-compatible.
- Hooks only read durable state to enforce coordinator-versus-worker mutation ownership and show recovery reminders. They do not spawn agents, wait on Orca, run tests, or advance phases.
- Use Orca task/dispatch messaging only for this explicitly supervised handoff. Keep existing IssueOps child contracts for other delegation shapes.
- Verify the unknown create/dispatch schemas with an isolated disposable-repository criterion test before relying on them in production code.

## Live-spike resolution

The required mutation spike was completed after the initial research. It confirmed the create/terminal/task/dispatch projections and changed the recovery design materially:

- repeated worktree names create suffixed worktrees/branches instead of failing;
- repeated task titles create distinct tasks;
- terminal custom titles and create/list tab identities are not stable recovery keys;
- `dispatch-show` reliably recovers a dispatch once task ID is known;
- `orca worktree rm --force` removed the temporary worktrees, directories, branches, and terminals;
- there is no per-task delete, so disposable tasks were terminally completed rather than globally reset.

The resulting V1 rule is: invoke each create mutation once, never retry an ambiguous response automatically, reconcile only one exact ownership-epoch marker (or one terminal PTY delta), and leave zero/multiple candidates fail-closed. Full evidence and cleanup receipts are in `.issueops/research/orca-live-handoff-spike-2026-07-11.md`.

## Access boundary and unresolved items

All web sources were public official sources. No login, paywall, or unofficial claim was used. Local inspection did not expose hook tokens or secret contents.

The following require a controlled mutation test and remain design-time unknowns:

- exact installed `worktree create --json` result fields;
- name-to-branch/path transformation and branch-collision behavior;
- partial-create rollback and retry deduplication;
- exact installed dispatch success result and injection failure transition;
- whether an agent process accepting its prompt is part of create success;
- full installed task/message/gate item schemas.

These are verification tasks, not fields to guess into a static DTO.
