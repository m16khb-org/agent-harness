# Orca-Aware IssueOps Supervised Handoff Design

**Date:** 2026-07-11
**Status:** Design approved for specification; implementation pending review
**Scope:** Optional Orca-backed IssueOps execution handoff with legacy inline compatibility

## 1. Problem

IssueOps currently links an isolated Git worktree but continues execution in whichever host session is already active. The durable record knows the issue, provider-linked branch, worktree, plan, readiness, and heartbeat, but it does not represent a coordinator handing implementation authority to a fresh agent session.

When Orca is available, the desired workflow is different:

1. The coordinator in the source checkout owns problem framing, remote issue creation, provider-linked branch preparation, worktree provisioning, and execution context preparation.
2. Orca starts a fresh host agent in the isolated worktree.
3. The worker explicitly claims the execution attempt, implements and verifies the change, and submits a result.
4. The coordinator validates the result and retains PR and cleanup authority.

The integration must not make Orca a dependency. If Orca or the required launch capability is unavailable before any Orca mutation, IssueOps must retain its current inline behavior.

## 2. Goals

- Make coordinator-to-worker execution ownership explicit, durable, and restart-recoverable.
- Use a fresh Orca worktree session for implementation when capability probing succeeds.
- Preserve existing inline IssueOps semantics when Orca is absent or explicitly disabled.
- Prevent coordinator writes, stale workers, duplicate dispatch, scope drift, and worktree escape.
- Inject a bounded, redacted execution context rather than a raw conversation transcript.
- Keep hooks deterministic and fast: observe, block, and relay guidance; never run the workflow.
- Keep CLI and MCP response meaning host-neutral across Codex, Claude Code, and GJC.
- Use Orca orchestration only where supervision, recovery, and result joining justify it.
- Verify the implementation through binary Turing criteria and cleanup receipts.

## 3. Non-goals

- Installing, upgrading, configuring, or registering Orca.
- Reimplementing Orca worktrees, terminals, task storage, gates, automations, or coordinator loops.
- Building a generic orchestrator plugin registry or distributed scheduler.
- Replacing IssueOps child cycles or workpool with Orca tasks.
- Copying raw transcripts, credentials, hook tokens, or secret-bearing local configuration.
- Automatically creating remote issues or PRs from hooks.
- Running background polling or automatic retries from lifecycle hooks.
- Depending on automatic restart of `orca orchestration run`; Orca exposes no public resume contract.
- Supporting arbitrary user-provided terminal commands in the first release.
- Repairing the separate legacy-JSON workpool reminder defect; record it as a follow-up rather than bundling it into the handoff release.

## 4. Chosen approach

Use one IssueOps cycle with an optional **supervised execution handoff**.

The core mental model is:

> IssueOps owns one durable execution lease; Orca is an optional transport that provisions and routes the worker attached to that lease.

This keeps authority in agent-harness while using Orca for the functions it already provides. Orca task, dispatch, terminal, and message records are reconciliation evidence. They do not replace the IssueOps record.

### 4.1 Rejected alternatives

#### Direct prompt-only handoff

`orca worktree create --agent --prompt` is sufficient when a coordinator delivers ownership and stops. It is insufficient here because IssueOps must detect partial creation, block duplicate execution, join a verified result, and recover after either process restarts.

#### Generic scheduler integration

Mapping IssueOps children, workpool tasks, skills, and every host operation into an Orca DAG would duplicate existing contracts and create two authorities for lifecycle state. This design adds only the execution handoff needed by the requested workflow.

## 5. Activation and compatibility

The coordinator selects one requested mode:

- `auto` — default. Resolve to Orca only when the entire read-only capability probe succeeds.
- `orca` — require Orca. Probe failure is returned as an error; it never silently changes mode.
- `inline` — preserve the current same-session IssueOps execution path.

Requested/resolved mode belongs to the command result. The durable handoff field is created only after Orca passes the probe and the coordinator is about to provision through Orca. A missing handoff field therefore remains exactly the existing inline contract.

### 5.1 Read-only capability probe

Before any external mutation, the adapter verifies:

1. `orca` exists on `PATH`.
2. `orca status --json` produces a valid stdout JSON envelope.
3. `result.runtime.reachable` is true and runtime state is `ready`.
4. Graph state is ready when the installed contract exposes it.
5. The target repo resolves through Orca.
6. Required worktree, terminal, and orchestration commands are supported.
7. A safe host launch/delivery strategy is available.

`orca --version` is not used: the installed relay does not expose a reliable version command. Handshake output on stderr is diagnostic only; stdout alone is decoded as JSON.

### 5.2 Fallback boundary

- In `auto`, a failed read-only probe returns the existing inline worktree guidance plus a redacted fallback code; it does not add handoff state to the record.
- After an Orca mutation is invoked, no error or timeout may switch execution to inline.
- Any ambiguous post-invocation result moves the lease to `recovery_required`.

This is the duplicate-worker boundary. A timeout may mean an external artifact was created even when the client did not receive its ID.

### 5.3 Inline compatibility

For resolved inline mode:

- no worker claim is required;
- existing implementation readiness and phase transitions remain unchanged;
- current repo/worktree binding and guard behavior remains the fallback contract;
- Orca is not consulted by install, update, bootstrap, self-verify, or ordinary hooks.

## 6. Durable model

Add one optional nested field to `IssueOpsRecord`. The additive field keeps schema version 1 backward-compatible; unsupported future root schemas remain fail-safe as they are today.

```go
type IssueOpsExecutionHandoff struct {
    ProtocolVersion int
    State           string
    ClosedDisposition string

    Attempt         int
    OwnershipEpoch  string
    ContextSHA256   string
    ContextVersion  int

    Driver          string // orca
    Agent           string
    DeliveryMode    string // inject | terminal_send

    CoordinatorRoot string
    WorkerRoot      string
    WorkerSession   *IssueOpsHostSessionIdentity
    Orca            *IssueOpsOrcaIdentity
    Result          *IssueOpsExecutionHandoffResult
    Failure         *IssueOpsExecutionHandoffFailure

    PreparedAt      string
    ProvisionedAt   string
    DispatchedAt    string
    ClaimedAt       string
    LastHeartbeatAt string
    CompletedAt     string
    AcceptedAt      string
    UpdatedAt       string
}
```

The exact Go field names may follow existing model conventions, but the JSON contract uses stable snake_case fields.

### 6.1 States

```text
coordinator_preparing
dispatched
claimed
submitted
closed
recovery_required
```

- `coordinator_preparing` is persisted before the first Orca mutation and remains coordinator-owned while the worktree, plan, reviews, tools, and dispatch context are prepared. Optional artifact IDs show how far preparation has progressed.
- `claimed` is the only Orca state that satisfies implementation-entry ownership readiness.
- `submitted` means the worker submitted a successful implementation result; it does not mean the coordinator accepted it.
- `closed` returns lifecycle authority to the coordinator and carries exactly one disposition: `accepted`, `worker_failed`, or `cancelled`.
- `recovery_required` means external state may differ from the record.

No separate `provisioned`, `running`, `completed`, `accepted`, `failed`, or `cancelled` states are needed. External artifact fields, `claimed` plus heartbeat, `submitted`, and the closed disposition carry those facts without mixing transport progress with lease ownership.

### 6.2 Attempt and compare-and-set rules

- `attempt` is monotonically increasing per cycle.
- `ownership_epoch` is a random, non-secret nonce generated before mutation.
- Every mutating handoff operation carries cycle ID, attempt, and epoch. Once dispatch preparation assigns the context hash, claim and all later mutations carry it as well.
- State writes run under the existing per-cycle IssueOps lock.
- External commands never run while the lock is held.
- After every external command, the result is persisted only if attempt, epoch, hash, and prior state still match.
- The epoch is embedded in worktree comments and task specifications as a reconciliation marker, not treated as an Orca idempotency key.
- No create mutation is automatically retried. The live spike proved that repeated worktree names and task titles create duplicates.
- Repeating worktree preparation or `handoff start` for an active attempt returns resume/recovery guidance instead of invoking the corresponding Orca mutation again.
- Ambiguous worktree/task creation is reconciled only when the epoch marker identifies exactly one artifact. Ambiguous terminal creation is reconciled only when the current PTY set minus the persisted baseline contains exactly one item. Zero or multiple candidates remain `recovery_required`.
- A stale worker cannot claim, heartbeat, finish, or fail a newer attempt.

### 6.3 Actor and transition table

| Actor / command | Required source state | Result state | Authority and idempotency |
|---|---|---|---|
| coordinator `worktree prepare --confirm` | no handoff | `coordinator_preparing` | Persist attempt/epoch before one create call. Same command returns existing state. Ambiguous result becomes `recovery_required`. |
| coordinator `handoff start --confirm` | `coordinator_preparing` with exact worktree, plan, tools, reviews | `dispatched` | Persist context hash and terminal baseline before mutation. Create terminal/task/dispatch once each; any ambiguous step becomes `recovery_required`. |
| worker `handoff claim` | `dispatched` | `claimed` | CAS on cycle/attempt/epoch/context plus native host session and worktree. Same owner may repeat; a different owner is rejected. |
| worker `heartbeat` | `claimed` | `claimed` | CAS on the full worker tuple; timestamp-only idempotent update. |
| worker `handoff finish --outcome completed` | `claimed` | `submitted` | Persist one bounded result keyed by worker/HEAD/evidence digest. Identical repeat succeeds; conflicting repeat fails. |
| worker `handoff finish --outcome failed` | `claimed` | `closed/worker_failed` | Same CAS and repeat rules as completed finish. |
| coordinator `handoff accept` | `submitted` | `closed/accepted` | Reverify HEAD/evidence/context under the cycle lock. Identical repeat succeeds. |
| coordinator `recover --action reconcile` | `recovery_required` | `coordinator_preparing` or `dispatched` | Persist only one uniquely matched external identity; never execute the next step or import a worker result. |
| coordinator `recover --action cancel --confirm` | non-closed; claimed requires explicit stale/force evidence | `closed/cancelled` | Mark closed before cleanup. Repeating cleanup is safe and visible. |
| coordinator `recover --action retry --confirm` | `closed/worker_failed`, `closed/cancelled`, or a fully reconciled abandoned attempt | `coordinator_preparing` with attempt+1 | Never reuses the prior epoch/task/dispatch or retries an ambiguous create. |
| any session `issueops resume` | any | unchanged | Read-only; may probe Orca for evidence but writes nothing. |

### 6.4 Host session identity

```go
type IssueOpsHostSessionIdentity struct {
    Host      string // codex | claude | gjc
    SessionID string
    AgentID   string
}
```

The worker identity is authoritative only together with the cycle, canonical worktree, ownership epoch, and context hash.

Coordinator recovery is intentionally not tied forever to one host session. The coordinator role is the source checkout plus explicit coordinator command under the cycle lock. This lets a restarted main session recover the work without impersonating the worker.

### 6.5 Orca identity

```go
type IssueOpsOrcaIdentity struct {
    RuntimeID             string
    WorktreeID            string
    WorktreeInstanceID    string
    WorktreePath          string
    TerminalBaselinePTYIDs []string
    WorkerPTYID           string
    WorkerMailboxHandle   string
    TaskID                string
    DispatchID            string
}
```

Rules:

- top-level Orca RPC correlation IDs are never stored as domain IDs;
- the live terminal handle is not persisted as authority; it is refreshed by matching the worker PTY ID inside the exact worktree;
- the worker handle captured by dispatch is retained only as its mailbox/assignee identity for historical message recovery;
- create-time tab IDs, pane keys, custom titles, and list-time tab/leaf IDs are excluded because the live spike showed they do not form one stable tuple;
- worktree ID, instance ID, canonical path, and branch are cross-checked to prevent stale path reuse;
- the pre-terminal PTY baseline is bounded and used only to reconcile an ambiguous terminal-create response by exact one-item set difference;
- raw Orca envelopes are not persisted; only bounded redacted projections and error codes are stored.

## 7. Context packet

After the coordinator has prepared the plan inside the linked worktree, the handoff renderer creates a deterministic stable projection from:

- cycle ID, issue URL, branch, base ref/SHA, and canonical worktree;
- intent problem statement and acceptance criteria;
- approved design decisions, constraints, alternatives, and non-goals;
- linked plan path and full plan-file SHA-256;
- relevant compatibility and devil's-advocate conclusions;
- required project documents and selected skill contracts;
- worker scope, verification commands, heartbeat cadence, stop conditions, and result format;
- attempt, ownership epoch, and claim/finish command templates.

It excludes:

- transcript history;
- environment dumps;
- credentials, hook tokens, and secret-like values;
- volatile timestamps and handoff state that would make re-rendering unstable.

The canonical JSON projection is hashed with SHA-256 and rendered as bounded Markdown. The packet is capped at 64 KiB. It references the complete plan inside the worktree rather than embedding an unbounded plan body.

Claim and finish re-render the projection. A changed intent, design, plan path/content, branch, or worktree produces `context_stale` and requires a new attempt.

## 8. Provision and dispatch flow

### 8.1 Coordinator worktree preparation

1. Validate IssueOps design, remote issue, and provider-linked branch prerequisites.
2. `issueops worktree prepare` runs the Orca probe when requested mode is `auto` or `orca`.
3. Without confirmation, return a preview only. Existing worktree prepare remains non-mutating by default.
4. For inline resolution, return the current Git worktree command/guidance and do not add handoff state.
5. For confirmed Orca resolution, persist `coordinator_preparing` with a new attempt and epoch before the first external mutation.
6. Create the checkout without launching the implementation agent, using deterministic branch/cycle metadata.
7. Verify the returned path, worktree instance, exact provider-linked branch, and linked issue before linking it to IssueOps.
8. Persist the Orca worktree identity while remaining `coordinator_preparing`.
9. The coordinator writes/links the plan in that worktree, runs worktree-tool preparation, and records compatibility and Brooks reviews through existing IssueOps commands.

This sequence preserves the current `plan_in_worktree` invariant and keeps all planning/preparation in the coordinator session.

### 8.2 Coordinator dispatch start

1. Re-read the `coordinator_preparing` handoff and a pre-dispatch readiness projection containing every implementation-entry prerequisite except the not-yet-possible worker claim.
2. Render the stable context projection now that the plan exists in the linked worktree; persist its version and hash under the same attempt/epoch.
3. Persist the worktree's current PTY IDs, then start a fresh agent terminal in the existing worktree exactly once.
4. Reacquire and verify the live terminal handle through `terminal list`.
5. Create one Orca task whose title/display name contains the cycle ID and attempt marker.
6. Dispatch/deliver the task and persist the task/dispatch tuple while transitioning to `dispatched`.
7. Return immediately with worker status and recovery commands; do not run a background wait loop.

### 8.3 Host launch and delivery

Preferred path:

- use `terminal create --worktree id:<worktree-id> --command <built-in-host-command>` to start a fresh agent in the already prepared checkout;
- reacquire that terminal with `terminal list`;
- use `dispatch --inject` after the task exists.

Compatibility path for a supported harness host that Orca does not recognize as an injectable agent:

- dispatch without injection and deliver the generated preamble/task through `terminal send`.

Only built-in host command mappings are allowed. Arbitrary command input is out of scope. Host launch/delivery support is checked by the pre-worktree capability probe. If it disappears after provisioning, the handoff becomes `recovery_required`; it cannot fall back inline.

### 8.4 Provider-linked branch invariant

The returned checkout must use the exact IssueOps branch prepared through GitHub/GitLab. The adapter may base creation on the verified provider ref, but it must not accept an unrelated Orca-generated branch merely because the worktree exists.

An isolated disposable-repository test must verify the installed Orca name/base/collision behavior before production wiring relies on it.

## 9. Worker claim and ownership

Hooks do not auto-claim.

The new session's SessionStart relay renders the exact claim command using the native hook `session_id`, host, expected cycle, attempt, epoch, and context hash. The worker explicitly executes that command before editing.

Claim validates:

- handoff state is `dispatched`;
- attempt, epoch, and context hash match;
- native host/session identity is non-empty;
- current Git root equals the canonical worker worktree;
- branch and HEAD lineage match the prepared contract;
- Orca worktree locator matches when available;
- no different session already owns the attempt.

On success, claim stores the worker identity and transitions to `claimed`. Implementation readiness requires this state whenever an Orca handoff field is present; an absent handoff remains inline.

## 10. Hook behavior

### 10.1 Common input

Extend common hook parsing with:

- `session_id`;
- optional `agent_id`/agent type;
- exact `cwd` and canonical Git root;
- host adapter identity;
- Orca worktree ID when present, never terminal handles as authority and never secret hook tokens.

### 10.2 PreToolUse ownership guard

For an Orca handoff:

- before `claimed`, implementation mutations in the linked worktree are blocked;
- after `claimed`, a mutation is allowed only when hook host/session equals the worker identity and hook cwd/Git root equals the worker worktree;
- a coordinator running in the source checkout cannot bypass the fence by targeting an absolute path inside the worktree;
- a different or restarted session inside the worktree is blocked until explicit recovery/new claim;
- coordinator plan/worktree preparation before dispatch plus claim, heartbeat, finish, accept, and recovery commands receive narrowly scoped exceptions;
- worktree escape is always blocked.

Inline and legacy records keep current behavior.

### 10.3 SessionStart claim guidance

SessionStart shows the native session ID, role, attempt, and exact claim/resume command. It does not claim, write IssueOps state, poll Orca, or advance a phase. PostToolUse, Stop, compact, and user-prompt behavior are unchanged in V1.

### 10.4 GJC parity

The installed GJC shim currently drops event/context input and ignores stdin. It must:

- accept `(event, ctx)` from the first-party HookAPI;
- send snake_case JSON including session ID, cwd, tool request, and host to agent-harness stdin;
- relay native `session_start` for claim guidance;
- await PreToolUse results and translate block/reason into GJC's hook result;
- keep the adapter thin; all ownership decisions remain in Go core.

## 11. Worker progress and result join

### 11.1 Heartbeat

Extend the existing IssueOps heartbeat request with optional handoff attempt/epoch/context/session data. For Orca mode it validates the claimed worker before updating `last_heartbeat_at`. It may best-effort mirror an Orca heartbeat message, but the IssueOps timestamp is authoritative.

### 11.2 Finish

The worker submits:

- final HEAD;
- changed file list;
- Turing evidence/report path;
- verification command results;
- cleanup receipts for worker-created temporary resources;
- Orca task/dispatch tuple when available;
- terminal result `completed` or `failed`.

The harness validates the worker identity and current context and writes the IssueOps result first. A completed result transitions `claimed -> submitted`; a failed result transitions `claimed -> closed` with disposition `worker_failed`. Repeating the same finish tuple is idempotent. It then best-effort updates the Orca task/sends `worker_done`.

The IssueOps result is the only completion authority. Orca task/message data may diagnose a sync failure but is never imported as a replacement worker result. If the IssueOps write was ambiguous, the worker repeats the same idempotent finish command.

### 11.3 Coordinator accept

Accept is a separate explicit command. It rechecks:

- worker result tuple and HEAD;
- implementation evidence already used by IssueOps;
- required Turing criteria and verification artifacts;
- no uncommitted scope drift outside the expected worktree;
- context is still current.

Acceptance transitions `submitted -> closed` with disposition `accepted`. The coordinator then owns PR, feedback-loop routing, merge verification, and cleanup through existing IssueOps commands.

Feedback requiring new code creates a new attempt/epoch and a fresh claim; it does not silently reopen the old worker lease.

## 12. Recovery and cancellation

Recovery is explicit and idempotent. It never runs in hooks. `issueops resume` remains observational and may report external evidence, but it never persists reconciliation or invokes the next operation.

`recover --action reconcile` may:

- refresh runtime and terminal identity;
- locate the unique ownership-epoch marker on a worktree or task;
- compute the one-item PTY delta from the pre-terminal baseline;
- inspect `dispatch-show`;
- read historical messages with the preserved worker mailbox handle;
- persist exactly one matching worktree, terminal, task, or dispatch identity into the current attempt;
- return the next explicit coordinator command without executing it.

Recovery may not:

- assume absence after a transport timeout without listing/reconciling;
- choose among zero or multiple candidates;
- reuse an external artifact from another attempt;
- import worker completion from Orca as an IssueOps result;
- switch to inline after partial mutation;
- delete a worktree or terminal without explicit confirmation.

`recover --action retry` creates a new attempt only after the prior attempt is terminally closed or every ambiguous artifact is resolved. `recover --action cancel --confirm` transitions to `closed/cancelled` before cleanup. Orca-owned worktrees are removed through `orca worktree rm`; inline worktrees continue through existing Git cleanup. A failed cleanup remains visible and retryable.

## 13. CLI and MCP surface

CLI family:

```text
issueops worktree prepare --orchestrator auto|orca|inline [--confirm]
issueops handoff start [--confirm]
issueops handoff claim
issueops handoff finish
issueops handoff accept
issueops handoff recover --action reconcile|retry|cancel [--confirm]
issueops resume
```

The existing `issueops heartbeat` is extended rather than duplicated.

MCP exposes one `issueops_handoff` tool with an action discriminator over the same request/result DTOs instead of advertising five near-duplicate tools. `issueops_resume` carries the read-only status projection. JSON responses include requested/resolved mode, state, attempt, context hash, redacted fallback/recovery code, and stable external domain IDs. Human text output derives from those DTOs.

Dry-run/preview is required for external mutation commands. Worktree preparation, handoff start, retry, and cancellation mutate only with the existing confirmation convention used by IssueOps remote operations.

## 14. Adapter boundary

Add a concrete `internal/adapter/orca` package around `exec.CommandContext` with an injected command runner for tests.

The adapter owns:

- safe argv construction;
- timeouts and stdout/stderr separation;
- generic RPC envelope decoding;
- narrow projections for status, repo, worktree, terminal, task, dispatch, and message data;
- stable error codes and redacted diagnostics;
- terminal-handle refresh.

Do not add a driver registry or duplicate Orca behavior in core. The IssueOps usecase receives the small concrete dependency seam it needs for deterministic fake-runner tests.

## 15. Skill and project-document integration

### IssueOps

- Document `auto|orca|inline`, coordinator/worker boundaries, claim, recovery, and acceptance.
- Make the worker stop after implementation, Turing verification, and ai-slop-clean evidence.
- Keep PR/merge/cleanup with the coordinator.

### Turing

- Render criterion IDs into the context packet.
- Require the worker report and cleanup receipts at finish.
- Correct stale guidance that says IssueOps has no heartbeat command.

No other skill source changes in V1. Von Neumann/Karpathy already supply plan/prompt discipline, Brooks remains a review gate, and Torvalds cleanup is invoked by IssueOps without duplicating Orca protocol guidance in each skill. Broader skill-specific Orca recipes are evaluated only after the core handoff survives fake-runner tests and the second live E2E run.

Self-verification may report optional Orca diagnostics, but Orca availability is never a gate.

## 16. Turing verification contract

Each criterion produces an observable artifact and cleanup receipt.

| ID | Criterion | Binary evidence |
|---|---|---|
| ORCA-01 | Orca absent/unreachable in `auto` preserves inline behavior | fake probe + absent handoff field + legacy record/readiness/golden tests |
| ORCA-02 | Explicit `orca` fails before mutation when not ready | command trace contains probe only |
| ORCA-03 | Ready Orca provisions and dispatches exactly once | fake trace + persisted worktree/task/dispatch projections |
| ORCA-04 | Post-mutation timeout never falls back or retries automatically | `recovery_required` + one create call + zero inline actions |
| ORCA-05 | Duplicate-marker and terminal-delta ambiguity fail closed | zero/one/multiple candidate fixtures based on live-spike behavior |
| ORCA-06 | Worker claim gates implementation and stale workers fail CAS | readiness before/after claim + attempt/session/context mismatch tests |
| ORCA-07 | Coordinator writes and worktree escape are blocked; claimed worker in-tree writes pass | positive/negative PreToolUse scenarios |
| ORCA-08 | Finish, submit, accept, failure, cancellation, and retry follow the transition table | table-driven state/actor/idempotency tests |
| ORCA-09 | Resume is read-only and only recover persists one unique reconciliation | fake command traces + record before/after comparison |
| ORCA-10 | Legacy records remain readable and inline | schema fixtures with absent handoff field |
| ORCA-11 | Context packet is bounded, deterministic, stale-sensitive, and redacted | golden/hash/oversize/secret fixtures |
| ORCA-12 | Codex, Claude, and GJC expose native session identity and enforce one real block smoke each | adapter contracts + installed-host smoke receipts |
| ORCA-13 | Completed path matches installed Orca and cleans worktree/branch/terminal resources | second isolated live E2E + cleanup receipt; completed task history noted |
| ORCA-14 | Repository gates pass | focused, full, race, build, response goldens, self-verify |

The live criterion is skipped only when Orca is unavailable; the product contract still passes through fake-runner tests and the inline fallback criterion. In the current environment Orca is ready, so ORCA-13 is required before completion.

## 17. Rollout order

1. **Completed design gate:** raw Orca spike covering create/list/collision, terminal identity, task/dispatch recovery, bare delivery, and cleanup; evidence is `.agent-harness/research/orca-live-handoff-spike-2026-07-11.md`.
2. Lock the six-state transition table, context hash, legacy normalization, and ambiguous-result rules with pure TDD.
3. Add the concrete Orca adapter with a fake runner and only the spike-verified projections.
4. Add common session parsing, SessionStart claim guidance, PreToolUse ownership enforcement, and the minimal GJC forwarding/block repair.
5. Implement coordinator worktree preparation and dispatch with pre-mutation fallback and no automatic mutation retry.
6. Implement claim, heartbeat, finish, accept, read-only resume, and exact-one recovery.
7. Wire CLI, the single MCP action tool, response goldens, and only the IssueOps/Turing skill updates.
8. Run three native-host block smokes, the second live Orca E2E, full/race/build/goldens/self-verify, ai-slop-clean, and the final Turing audit.

No production step starts until the revised design survives the independent Brooks re-review and user review.

## 18. Brooks review resolution

The first independent Brooks review returned `revise`. Its single most dangerous flaw was that the architecture fixed commands, states, identities, and recovery before a live Orca spike proved uniquely recoverable primitives.

Resolution:

- ran the disposable spike before production DTOs;
- documented that duplicate worktree names and task titles create new artifacts;
- removed automatic mutation retry and “resume next step” recovery;
- made resume observational and recover the only reconciliation writer;
- reduced nine mixed states to six lease states plus a closed disposition;
- added an actor/source/result/idempotency transition table;
- reduced persisted terminal identity to spike-supported locators;
- limited V1 hooks to SessionStart guidance and PreToolUse enforcement;
- removed the unrelated SQLite workpool reminder repair and broad skill edits;
- added a real native ownership-block smoke for all three hosts.

Spike evidence: `.agent-harness/research/orca-live-handoff-spike-2026-07-11.md`.

The independent re-review of the revised document returned `proceed`. Its remaining implementation risk is specific and testable: no crash boundary may cause a second worktree, terminal, task, or dispatch create call. ORCA-03–05 and ORCA-09 are the required falsification tests.

## 19. Source evidence

- Research synthesis: `.agent-harness/research/orca-issueops-orchestration-contract.md`.
- Current IssueOps record: `internal/core/issueops/model/types.go`.
- Current implementation readiness: `internal/core/issueops/issueops_readiness.go`.
- Current repo-scoped session binding: `internal/core/issueops/session/session.go`.
- Current source/worktree guard: `internal/core/lifecycle/lifecycle_worktree_guard.go`.
- Current GJC shim: `gjc-plugin/hook.ts`.
- Existing host-independent orchestration decision: `docs/superpowers/specs/2026-07-06-issueops-subagent-orchestration-design.md`.
