# Orca-Aware IssueOps Supervised Handoff Design

**Date:** 2026-07-11
**Status:** Implemented with the 2026-07-11 sealed-completion-authority correction
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

Add one optional nested field to `IssueOpsRecord` and use root IssueOps schema version 4. Missing/zero, v1, v2, and v3 records remain readable and upgrade on the next write, including locally created older handoff rows. A v1 binary sees v2+ as future and cannot strip the ownership lease; a v2 binary sees v3 as future and cannot strip the stable terminal tab/leaf locator; a v3 binary sees v4 as future and cannot strip the sealed mailbox recipients or completion projection intent. Versions greater than 4 remain fail-safe.

Legacy migration applies to the current attempt and every prior attempt: copy a missing live terminal from the legacy mailbox, then clear mailbox authority whenever no dispatch exists. In v4, `DispatchID` and `WorkerMailboxHandle` are either both absent or both present. ContextVersion 1 preserves v3 bytes and hashes by omitting an empty `coordinator_recipient`; every nonempty newly sealed recipient remains present in both the full context and source fingerprint.

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
    DeliveryMode    string // inject (V1)

    CoordinatorMailboxHandle string
    CoordinatorRoot string
    WorkerRoot      string
    WorkerSession   *IssueOpsHostSessionIdentity
    Orca            *IssueOpsOrcaIdentity
    PendingOperation *IssueOpsExecutionHandoffPendingOperation
    CleanupOnly     *IssueOpsOrcaCleanupArtifact
    Cancellation    *IssueOpsExecutionHandoffCancellation
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

`context_options` may include the additive optional `allow_codex_hook_trust_bypass` attestation. It remains context version 1 so an existing version-1 closed attempt can retry without a migration. The flag is false by default, is scoped to one attempt, and is reset on retry while every other sealed delivery option is preserved.

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
- `recovery_required` means external state may differ from the record. A durable `cancellation` tombstone in this state keeps the lease and hook guard active until exact external quiescence is proven.

No separate `provisioned`, `running`, `completed`, `accepted`, `failed`, or `cancelled` states are needed. External artifact fields, `claimed` plus heartbeat, `submitted`, and the closed disposition carry those facts without mixing transport progress with lease ownership.

### 6.2 Attempt and compare-and-set rules

- `attempt` is monotonically increasing per cycle.
- `ownership_epoch` is a random, non-secret nonce generated before mutation.
- Every mutating handoff operation carries cycle ID, attempt, and epoch. Once dispatch preparation assigns the context hash, claim and all later mutations carry it as well.
- State writes run under the existing per-cycle IssueOps lock.
- Orca/network calls and mutating subprocesses never run while the lock is held. The only subprocess exception is the fixed read-only local Git checkpoint (`branch --show-current`, `rev-parse --verify HEAD^{commit}`, `status --porcelain=v1`) needed to seal filesystem evidence immediately before a write.
- After every external command, the result is persisted only if attempt, epoch, hash, and prior state still match.
- The epoch is embedded in worktree comments and task specifications as a reconciliation marker, not treated as an Orca idempotency key.
- No create mutation is automatically retried. The live spike proved that repeated worktree names and task titles create duplicates.
- Repeating worktree preparation or `handoff start` for an active attempt returns resume/recovery guidance instead of invoking the corresponding Orca mutation again.
- Ambiguous worktree/task creation is reconciled only when the epoch marker identifies exactly one artifact. Ambiguous terminal creation is reconciled only when the current PTY set minus the persisted baseline contains exactly one item. Zero or multiple candidates remain `recovery_required`.
- Force-abandon ignores only post-baseline rows with a stable unique identity and every field needed to classify them as nonmatching. Missing or duplicate identities and incomplete classification rows remain ambiguous and block abandonment.
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
| coordinator `recover --action cancel --confirm` | non-closed; claimed/submitted require `--force` and a bounded reason | `recovery_required` with cancellation tombstone | Preserve the pending journal, worker identity, and guard. Only a truly pre-mutation attempt closes directly. |
| coordinator `recover --action finalize-cancel --confirm` | cancellation tombstone with authoritative absence or exact terminal/task/dispatch quiescence and stale worker liveness | `closed/cancelled` | Close only after the external lease is proven quiescent; a failed check leaves the tombstone byte-equivalent. |
| coordinator `recover --action retry --confirm` | safely finalized `closed/worker_failed` or `closed/cancelled` | `coordinator_preparing` with attempt+1 | Never reuses the prior epoch/task/dispatch or retries an abandoned ambiguous create. |
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
    WorkerTerminalHandle  string
    WorkerTabID           string
    WorkerLeafID          string
    TaskID                string
    DispatchID            string
}
```

Rules:

- top-level Orca RPC correlation IDs are never stored as domain IDs;
- the coordinator mailbox recipient is sealed before the first Orca dispatch and cannot be derived later from the current task, caller environment, or live terminal inventory;
- the worker handle captured by dispatch is sealed as `WorkerMailboxHandle`, the historical mailbox/assignee identity used for `worker_done` and mailbox recovery;
- `WorkerTerminalHandle` is separate live control identity. Exact runtime recovery may refresh only this live handle and its runtime locators; it never overwrites either sealed mailbox recipient;
- current Orca terminal-list `tabId`/`leafId` are persisted as a pair. The observed runtime rollover reissued handle/PTY and worktree instance while retaining tab/leaf, so a v3 attempt prefers that exact stable pair;
- v2 attempts that never observed tab/leaf may fall back only to the bounded custom tab title joined from `visualLayouts[].root.tabs[]` by the current exact tab/leaf. The dynamic terminal `title` is never a fallback marker;
- worktree ID, instance ID, canonical path, and branch are cross-checked to prevent stale path reuse;
- a nonempty current-runtime worktree instance may equal the persisted instance. Missing instance, terminal/worktree mismatch, or conflicting duplicate evidence fails closed;
- runtime-refresh completion exact-compares the journaled record and re-renders context source plus clean exact branch/attempt-base HEAD inside the cycle lock immediately before the one-write identity replacement. Generic post-mutation operation completion remains unchanged;
- a connected/writable recovered terminal or uncommitted worker checkout forbids replacement. Stale relay environment pins may prove only a transport handshake; caller cancellation bounds observation without sending control input to the target PTY;
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
- sealed coordinator mailbox recipient in the canonical packet; the official dispatch preamble completes the delivered bounded context with the exact task label and `--dispatch-id` token once those IDs exist;
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

1. Require and persist a concrete coordinator mailbox recipient, then re-read the `coordinator_preparing` handoff and a pre-dispatch readiness projection containing every implementation-entry prerequisite except the not-yet-possible worker claim.
2. For Codex, verify installed bypass-flag support and perform the documented read-only `hooks/list` review for the exact worker cwd. This human attestation is not implemented as an automatic app-server/fingerprint verifier in V1.
3. Render an unattested preview, then an attested no-confirm preview. Record the latter context hash and require the final confirm request to differ only by `confirm=true`.
4. Under the per-cycle lock, re-read the same record, re-render its source fingerprint, and require the canonical worker checkout to remain on the exact branch and attempt-base HEAD with a clean status before persisting that stable context version/hash. Missing Codex attestation or a changed checkout fails before any terminal/task/dispatch call.
5. Before each first-time terminal, task, and dispatch journal write, repeat the locked record/source/branch/HEAD/clean checkpoint. Persist the worktree's current PTY IDs, then start a fresh agent terminal in the existing worktree exactly once.
6. Reacquire and verify the live terminal handle through `terminal list`.
7. Create one Orca task whose title/display name contains the cycle ID and attempt marker.
8. Dispatch/deliver the task and persist the task/dispatch tuple plus the historical worker mailbox while transitioning to `dispatched`. Dispatch recovery requires the sealed coordinator to pass the same concrete bounded `term_*` validation before `dispatch-show`, then validates the returned preamble by its official exact coordinator and task label lines and exact `--dispatch-id <id>` token. A rejected later-stage checkpoint preserves all identities completed by prior stages and leaves no new pending journal.
9. Return immediately with worker status and recovery commands; do not run a background wait loop.

Every operation journal receives a fresh start timestamp immediately before its write. Every post-call completion, failure, or dispatch transition obtains another fresh timestamp after the external call; it never reuses the pre-call journal time.

### 8.3 Host launch and delivery

V1 path:

- capability-negotiate `terminal create --worktree id:<worktree-id> --agent <built-in-host>` when the installed help exposes the fixed agent surface, otherwise use the verified current `--command <built-in-host-command>` surface;
- after the explicit attestation above, use the installed Codex-only `--dangerously-bypass-hook-trust` launch flag; Claude and GJC commands are unchanged;
- reacquire that terminal with `terminal list`;
- use `dispatch --inject` after the task exists.

Only built-in host mappings are allowed. Arbitrary command input is out of scope. Host launch/delivery support is checked by the pre-worktree capability probe and again immediately before terminal create. If it disappears after provisioning, the terminal journal remains `recovery_required` even though no terminal mutation was invoked; it cannot clear to ordinary retry or fall back inline.

V1 has no `terminal send` compatibility delivery. The durable dispatch journal seals delivery mode `inject` and the refreshed exact assignee before invocation; recovery validates that tuple against `dispatch-show` without inventing an `injected` response field that the installed show command does not expose.

### 8.4 Provider-linked branch invariant

The returned checkout must use the exact IssueOps branch prepared through GitHub/GitLab. The adapter may base creation on the verified provider ref, but it must not accept an unrelated Orca-generated branch merely because the worktree exists.

GitHub worktree creation passes the verified numeric issue through Orca's public `--issue` flag. The installed Orca contract labels that flag as GitHub-only and exposes no GitLab issue flag, so GitLab supervised creation uses the exact provider tracking ref and omits `--issue` and every invented metadata flag. A nonzero GitHub link or a mismatched nonzero `linkedGitLabIssue` rejects the returned GitLab worktree. A null or zero GitLab link is accepted with the stable `orca_gitlab_native_metadata_unavailable` warning; an exact native link removes that warning. The bounded observation is durable in the Orca identity so restart/status projection cannot change the warning accidentally.

The sealed context includes both the verified provider and exact IssueOps issue URL. GitLab `auto` retains the legacy inline response without an Orca warning when Orca is absent, unready, or capability-incomplete; only a successfully resolved Orca preview or confirmation can report native GitLab metadata unavailability. No handoff step creates or mutates a remote GitLab issue, branch, work item, or merge request.

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

SessionStart shows the native session ID, role, attempt, and exact claim/resume command. It does not claim, write IssueOps state, poll Orca, or advance a phase. PostToolUse, compact, and user-prompt behavior are unchanged in V1, with one narrow observed-host exception: Stop's numbered-next-action relay and missing-choice re-entry are suppressed for the exact worker whose durable handoff record already carries a completed result and a terminal `worker_done_projection` (sent, failed, or intent), matched by native host/session/agent identity and canonical worktree path — see `SuppressStopNextActionForCompletedWorker` in `internal/core/lifecycle`. Because the installed Codex Stop command and its native payload are hostless, that exact no-flag/no-payload case defaults only the identity match to `codex`; explicit host conflicts stay fail-closed and legacy output formatting remains flag-driven. This predicate never inspects a transcript, shell output, or `ORCA_TERMINAL_HANDLE`; it does not alter lifecycle computation, Engelbart checks, `--json` fields, or any other Stop behavior, and any mismatch or ambiguity falls back to legacy Stop output byte-for-byte.

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

The harness validates the worker identity and current context. For a completed result, it derives a deterministic bounded projection exclusively from the durable record and freshly verified exact worker evidence, then transitions `claimed -> submitted` and persists the projection intent in the same IssueOps cycle-lock write. Only after that durable write is visible does it attempt exactly one external `worker_done` outside the lock through the safe argv-only Orca adapter. A failed result transitions `claimed -> closed` with disposition `worker_failed` and performs no completion projection.

The completion payload uses the sealed historical worker mailbox as sender and the separately sealed coordinator mailbox as recipient. Exact task/dispatch, changed files, report path, final head, host/attempt identity, subject, and three-sentence body come from persisted evidence, never from caller environment, a current Orca task, request-only values, or the refreshable live terminal. Success records bounded message identity/evidence. Failure, timeout, malformed output, ambiguity, or crash leaves `submitted` authoritative and is never automatically retried; an identical finish returns stable diagnostic evidence without another send. Manual submitted-worker shell `worker_done` is blocked.

The IssueOps result is the only completion authority. Orca task/message data may diagnose a projection failure but is never imported as a replacement worker result.

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

- refresh a changed runtime only after complete bounded worktree and terminal inventories agree. The unique current-runtime worktree row must match exact worktree ID, nonempty instance, repo, base ref, path, branch, attempt-base HEAD, and comment marker; the terminal must match persisted tab/leaf or the legacy stable visual-tab marker. Runtime, instance, handle, PTY, tab, and leaf update in one CAS;
- locate the unique ownership-epoch marker on a worktree or task;
- compute the one-item PTY delta from the pre-terminal baseline;
- inspect `dispatch-show`;
- read historical messages with the preserved worker mailbox handle;
- persist exactly one matching worktree, terminal, task, or dispatch identity into the current attempt;
- return the next explicit coordinator command without executing it.

Recovery may not:

- assume absence after a transport timeout without listing/reconciling;
- choose among zero or multiple candidates;
- retain a stale worktree instance while adopting a current-runtime terminal, or require the current nonempty instance to differ from the old one;
- reuse an external artifact from another attempt;
- import worker completion from Orca as an IssueOps result;
- switch to inline after partial mutation;
- delete a worktree or terminal without explicit confirmation.

`recover --action retry` creates a new attempt only after the prior attempt is safely closed; a force-abandoned ambiguous operation is never retryable. `recover --action cancel --confirm` first writes a `recovery_required` tombstone for every provisioned attempt and therefore does not release the live-worker guard. `recover --action finalize-cancel --confirm` closes only after complete authoritative inventory proves an exact pending candidate absent, the persisted terminal disconnected or absent, the exact task/dispatch terminal or authoritatively absent, and any claimed heartbeat older than the minimum age. A failed check leaves the tombstone active. Orca-owned worktrees are removed through `orca worktree rm`; inline worktrees continue through existing Git cleanup.

## 13. CLI and MCP surface

CLI family:

```text
issueops worktree prepare --orchestrator auto|orca|inline [--confirm]
issueops handoff start --coordinator-recipient <term_...> [--allow-codex-hook-trust-bypass] [--confirm]
issueops handoff claim
issueops handoff finish
issueops handoff accept
issueops handoff recover --action reconcile|abandon|cancel|finalize-cancel|retry [--confirm]
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
- one-attempt bounded `worker_done` projection with safe argv and no shell.

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
