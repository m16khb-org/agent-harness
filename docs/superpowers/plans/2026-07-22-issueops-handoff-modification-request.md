# IssueOps Handoff Modification Request Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Do not dispatch sub-agents unless the user explicitly authorizes them.

**Goal:** Let a fresh session in the exact source checkout send one bounded, durable, idempotent modification request to the active handoff owner after PR publication, while preserving owner-only mutation/publication authority and blocking raw Orca identity bypasses.

**Architecture:** Extend the schema-v8 ownership handoff with an append-only modification-request projection. The core persists a no-retry intent under the IssueOps cycle lock, releases the lock for exactly one narrow Orca `status` message call, then compare-and-sets only that entry to `sent` or `failed`. CLI and MCP share the same core request and adapter. Lifecycle policy allows only the typed source command, adds owner `feedback add`, and treats raw Orca `to`/`from` identities as protected resources. Existing owner publish remains the sole Git push path and gains explicit fast-forward replacement regressions.

**Tech Stack:** Go 1.26, standard `testing`, existing IssueOps JSON store/lock, existing Orca CLI adapter, CLI `flag`, MCP action-discriminated catalog, response-contract goldens, Markdown project contracts.

**Authoritative design:** `docs/superpowers/specs/2026-07-22-issueops-handoff-modification-request-design.md`

## Global constraints

- Keep `IssueOpsCurrentSchemaVersion == 8`. This additive optional field must not invalidate active schema-v8 handoffs.
- The source session may request a correction only. It never gains feedback, phase, file, Git, remote-artifact, publish, completion, or cleanup authority in the worker root.
- The caller must be a supported host with a non-empty session, must be executing from the exact canonical source root, must match the hook actor, and must not equal the sealed `OwnerSession`. It does not need to equal the original coordinator session.
- Require `owner_active`, phase `pr`, no completion, a publish receipt, a remote artifact, consistent provider/base evidence, and sealed coordinator/worker mailbox plus task/dispatch identities.
- Normalize the request body by trimming, then applying `policy.RedactFreeform`. Accept valid UTF-8 bodies of 1–4096 bytes; allow LF and tab; reject CR, DEL, and all other ASCII C0 control bytes.
- Persist at most 32 projections. Reject request 33 before any Orca invocation; never prune or overwrite an old request.
- A persisted `intent` is a no-retry tombstone. Repeating the same request key returns the existing projection without invoking Orca, even after a prior crash or failed send.
- The request key excludes actor identity and uses length-delimited hashing over the exact design fields so identical requests from separate fresh source sessions converge.
- Do not hold the IssueOps cycle lock during the Orca process call. Finalization must re-read and update only the immutable entry identified by `request_key`; unrelated concurrent record changes must survive.
- No direct `git push` exception is added. Only the active owner can apply feedback, commit, and invoke `issueops handoff publish --confirm`.
- Use `apply_patch` for implementation edits. Preserve unrelated changes. Future commits require explicit user authorization and must follow `.issueops/COMMIT_POLICY.md`; no push is authorized by this plan.
- If implementation reveals a need to change any invariant above, stop and amend/re-review the design before continuing.
- Angle-bracket tokens in command illustrations name runtime fields supplied by tests or fixtures; they are not unresolved authoring placeholders and must never be pasted literally by an operator.

## Acceptance map

| Requirement | Primary implementation | Primary verification |
|---|---|---|
| Additive durable projection in schema v8 | `model/types.go`, handoff envelope validation | JSON round-trip and malformed-entry table tests |
| Exact bounded Orca status message | `port/orca.go`, `adapter/orca/client.go` | fixed argv and strict response-validation tests |
| Fresh exact-source authority | new core request workflow | source/owner/root/state/evidence authority table tests |
| No-retry, cross-session idempotency | request-key builder and intent-first workflow | same-key, changed-body, changed-head, crash-tombstone tests |
| Concurrent-state preservation | finalization CAS under cycle lock | concurrent unrelated mutation and immutable-entry mismatch tests |
| CLI/MCP parity | handoff CLI, MCP catalog/handler, facade | parser, catalog, handler, and output contract tests |
| Lifecycle fence remains owner-only | lifecycle authority and resource targeting | typed allowlist, owner feedback, raw `to`/`from` denial tests |
| Owner can republish only fast-forward updates | publication core tests | ancestor success and all no-push failure branches |
| Operator contract is unambiguous | project docs, skill reference, goldens | docs scan, golden tests, full suite/race/build |

---

### Task 0: Version the approved plan

**Files:**
- Add: `docs/superpowers/plans/2026-07-22-issueops-handoff-modification-request.md`

- [ ] **Step 1: Re-read the design and plan together**

Run:

```bash
git diff --check
git diff -- docs/superpowers/specs/2026-07-22-issueops-handoff-modification-request-design.md docs/superpowers/plans/2026-07-22-issueops-handoff-modification-request.md
```

Expected: no whitespace errors; each acceptance-map row is backed by a later task and no implementation file has changed.

- [ ] **Step 2: Scan for unresolved placeholders and forbidden scope**

Run:

```bash
rg -n 'TO[D]O|TB[D]|FIXM[E]|schema version 9|direct git push|automatic retry|silent pruning' docs/superpowers/plans/2026-07-22-issueops-handoff-modification-request.md
```

Expected: no unresolved placeholder. Forbidden phrases appear only in explicit prohibitions or negative tests.

- [ ] **Step 3: Commit only after explicit user authorization**

Use `atomic-commit-push` for one local documentation commit with subject:

```text
docs(issueops): plan bounded handoff corrections
```

Do not push.

---

### Task 1: Add and validate the schema-v8 modification projection

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Modify: `internal/core/issueops/handoff/envelope.go`
- Modify: `internal/core/issueops/handoff/ownership_envelope.go`
- Modify: `internal/core/issueops/handoff/ownership_envelope_test.go`
- Modify: `internal/core/issueops/issueops_schema_version_test.go`

**Interfaces:**
- Produces `IssueOpsExecutionHandoffModificationRequest` and optional `ModificationRequests` on `IssueOpsExecutionHandoff`.
- Keeps `IssueOpsCurrentSchemaVersion` at 8.
- Makes malformed persisted projection data fail closed on both read and write.

- [ ] **Step 1: Write the schema and envelope RED tests**

Add table coverage for:

1. a valid `intent`, `sent`, and `failed` entry;
2. 32 valid entries accepted and 33 rejected;
3. empty/duplicate request key;
4. bad attempt, epoch, context hash, payload hash, published head, state, or diagnostic code;
5. missing sealed from/to/task/dispatch identity;
6. `sent` without a non-empty message ID and positive sequence;
7. `intent` carrying completion fields;
8. invalid timestamps or mutable values that do not match the surrounding handoff fence;
9. schema-v8 JSON without the field still decoding successfully;
10. schema-v8 JSON with the field round-tripping without loss.

The model shape is exact:

```go
type IssueOpsExecutionHandoffModificationRequest struct {
    RequestKey         string `json:"request_key"`
    Attempt            int    `json:"attempt"`
    OwnershipEpoch     string `json:"ownership_epoch"`
    ContextSHA256      string `json:"context_sha256"`
    State              string `json:"state"`
    Invoked            bool   `json:"invoked"`
    DiagnosticCode     string `json:"diagnostic_code"`
    PayloadSHA256      string `json:"payload_sha256"`
    FromHandle         string `json:"from_handle"`
    ToHandle           string `json:"to_handle"`
    Subject            string `json:"subject"`
    Body               string `json:"body"`
    TaskID             string `json:"task_id"`
    DispatchID         string `json:"dispatch_id"`
    PublishedHead      string `json:"published_head"`
    RemoteArtifactURL  string `json:"remote_artifact_url"`
    RequestedByHost    string `json:"requested_by_host"`
    RequestedBySession string `json:"requested_by_session"`
    RequestedByAgentID string `json:"requested_by_agent_id,omitempty"`
    MessageID          string `json:"message_id,omitempty"`
    MessageSequence    int64  `json:"message_sequence,omitempty"`
    StartedAt          string `json:"started_at"`
    CompletedAt        string `json:"completed_at,omitempty"`
}
```

Add to `IssueOpsExecutionHandoff`:

```go
ModificationRequests []IssueOpsExecutionHandoffModificationRequest `json:"modification_requests,omitempty"`
```

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/core/issueops/handoff ./internal/core/issueops -run 'ModificationRequest|SchemaV8' -count=1
```

Expected: FAIL because the projection field and validation do not exist.

- [ ] **Step 3: Implement the minimal model and validator**

Add projection constants in the handoff package or the new core workflow, reuse the existing SHA/timestamp/host helpers, and call a dedicated validator from `validateOwnershipEnvelope` before its state switch.

Validation must be state-aware and append-only-safe. It must not require the slice to exist, change current ownership-state semantics, or bump the schema. Check uniqueness with a local map and reject over 32 entries before iterating.

- [ ] **Step 4: Verify GREEN**

Run the focused test command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit only if implementation commits are explicitly authorized**

Suggested subject:

```text
feat(issueops): persist handoff correction intents
```

---

### Task 2: Add the narrow Orca modification-message adapter

**Files:**
- Modify: `internal/port/orca.go`
- Modify: `internal/adapter/orca/client.go`
- Modify: `internal/adapter/orca/client_test.go`

**Interfaces:**
- Produces `OrcaModificationRequest`, `OrcaModificationResult`, and `OrcaModificationClient`.
- Invokes only `orca orchestration send` with type `status` and the sealed identifiers supplied by core.

- [ ] **Step 1: Write adapter RED tests beside `SendWorkerDone` tests**

Cover:

- exact argv and one invocation;
- valid message response projection;
- empty/malformed input rejected before invoking the runner;
- command failure preserves the adapter's `Invoked` truth;
- malformed JSON and redacted diagnostics;
- mismatched message ID fields: from, to, type, subject, body;
- payload mismatch for `taskId` or `dispatchId`;
- empty message ID or non-positive sequence.

The exact argv is:

```text
orca orchestration send
  --to <WorkerMailboxHandle>
  --from <CoordinatorMailboxHandle>
  --type status
  --subject "IssueOps <id> modification requested"
  --body <redacted body>
  --task-id <TaskID>
  --dispatch-id <DispatchID>
  --json
```

The port stays separate from the broad `OrcaClient`, following `OrcaWorkerDoneClient`:

```go
type OrcaModificationClient interface {
    SendModificationRequest(context.Context, OrcaModificationRequest) (OrcaModificationResult, error)
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/adapter/orca -run 'ModificationRequest' -count=1
```

Expected: FAIL because the port and adapter method are absent.

- [ ] **Step 3: Implement the fixed adapter**

Reuse the existing runner, output envelope, payload decoding, and `port.OrcaError` conventions from `SendWorkerDone`. Build argv as a slice, never through a shell. Validate the returned `status` message against every request field before returning `MessageID` and `Sequence`.

- [ ] **Step 4: Verify GREEN and the existing worker-done contract**

Run:

```bash
go test ./internal/adapter/orca -run 'ModificationRequest|WorkerDone' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit only if authorized**

Suggested subject:

```text
feat(orca): add bounded correction delivery
```

---

### Task 3: Implement intent-first, no-retry core delivery

**Files:**
- Add: `internal/core/issueops/issueops_modification_request.go`
- Add: `internal/core/issueops/issueops_modification_request_test.go`
- Modify: `internal/core/issueops_facade.go`

**Interfaces:**
- Produces `IssueOpsHandoffModificationRequest`, `IssueOpsHandoffModificationResult`, `RequestIssueOpsHandoffModification`, and `RequestIssueOpsHandoffModificationWithProjection`.
- Consumes `port.OrcaModificationClient` without exposing the adapter to model or handoff packages.

- [ ] **Step 1: Write request validation and authority RED tests**

Start from an `owner_active` fixture in phase `pr` with a current publish receipt, verified remote artifact, exact source root, distinct owner session, and sealed Orca identities. Add table tests that reject before invoking Orca when any one invariant is absent or mismatched:

- `--confirm` false;
- unsupported host, empty session, non-canonical agent ID;
- source CWD not the exact canonical source root;
- caller equals `OwnerSession`;
- state not `owner_active`, phase not `pr`, or completion present;
- publish receipt or remote artifact missing;
- published head/provider/base evidence inconsistent;
- coordinator/worker mailbox, task, or dispatch identity missing;
- invalid UTF-8, empty-after-redaction, over 4096 bytes, CR, DEL, or disallowed C0 control;
- 32-entry capacity already full for a new key.

Also prove a fresh source session different from `CoordinatorSession` is accepted when host/session/root/hook identity are valid.

- [ ] **Step 2: Write key and durability RED tests**

Pin the key as a known SHA-256 test vector. Build it with a helper that writes a fixed domain followed by each field as an explicit length and raw bytes:

```text
issueops-handoff-modification-request:v1
record ID
attempt
ownership epoch
context SHA-256
publish receipt final head
remote artifact URL
normalized redacted body
```

Tests must prove:

1. actor identity does not affect the key;
2. body or published head changes the key;
3. first call persists `intent` before the fake client observes control;
4. same-key `intent`, `sent`, or `failed` returns the existing projection with zero client calls;
5. a send success transitions only that entry to `sent`;
6. a send error transitions only that entry to `failed` and preserves `Invoked`/diagnostic code;
7. a concurrent unrelated record mutation between intent and outcome survives;
8. missing or mutated intent at finalization returns a compare-and-set error and does not overwrite the record;
9. a second distinct key appends rather than replacing history;
10. request 33 fails before a client call.

- [ ] **Step 3: Verify RED**

Run:

```bash
go test ./internal/core/issueops -run 'ModificationRequest' -count=1
```

Expected: FAIL because the workflow does not exist.

- [ ] **Step 4: Implement normalization and the intent transaction**

The public request is:

```go
type IssueOpsHandoffModificationRequest struct {
    ID, Host, SessionID, AgentID, SourceCWD, Body string
    Confirm bool
}
```

Inside the first `withIssueOpsLock` block:

1. normalize actor/path/body;
2. read and validate current record/evidence;
3. derive the request key and return an existing exact entry immediately;
4. reject if the slice already has 32 entries;
5. append an immutable `intent` with `DiagnosticCode: "intent_persisted"` and persist it.

The `PayloadSHA256` is computed from the exact adapter request. `FromHandle` is the coordinator mailbox; `ToHandle` is the worker mailbox. Record both the published head and remote artifact URL used in the key/evidence.

- [ ] **Step 5: Invoke Orca once outside the lock**

Call `SendModificationRequest` only for a newly persisted intent. There is no retry path and no preview mode. The adapter error must be converted to a bounded diagnostic code using the same pattern as worker-done projection.

- [ ] **Step 6: Compare-and-set only the exact entry**

In a second `withIssueOpsLock` block:

1. re-read the record;
2. find exactly one entry with `request_key`;
3. compare every immutable field to the persisted intent snapshot;
4. require its state still be `intent`;
5. update only `state`, `invoked`, `diagnostic_code`, `message_id`, `message_sequence`, and `completed_at`;
6. update handoff/record timestamps and write the re-read record.

Do not fall back to the old snapshot on lock/write/CAS errors. Return the current persisted record only when the operation can prove it is the same immutable entry.

- [ ] **Step 7: Expose the exact types through the facade**

Add aliases and a wrapper in `internal/core/issueops_facade.go`, following the ownership-completion/publication wrappers. Keep the core package as the CLI/MCP dependency boundary.

- [ ] **Step 8: Verify GREEN**

Run:

```bash
go test ./internal/core/issueops -run 'ModificationRequest' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit only if authorized**

Suggested subject:

```text
feat(issueops): deliver durable correction requests
```

---

### Task 4: Add CLI and MCP action parity

**Files:**
- Modify: `cmd/issueops/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/issueops/issueopscli/issueops_handoff_cli_test.go`
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops_handlers.go`
- Modify: `cmd/issueops/mcpcli/mcp_issueops_helpers_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`

**Interfaces:**
- Adds CLI `issueops handoff request-modification`.
- Extends the existing MCP `issueops_handoff` action enum with `request-modification`; it does not add another MCP tool.

- [ ] **Step 1: Write CLI RED tests**

Extend `TestIssueOpsHandoffExposesOnlyCurrentActions` and add focused parsing/output tests for:

```text
issueops handoff request-modification \
  --id <id> \
  --host <host> \
  --session-id <session> \
  [--agent-id <agent>] \
  --source-cwd <root> \
  --body <request> \
  --confirm \
  --json
```

Assert required fields map exactly to the core request, `--confirm` is required by core, and JSON contains the record plus the selected projection/result fields without losing the durable state.

- [ ] **Step 2: Write MCP schema/handler RED tests**

Update parity expectations so the action enum includes `request-modification`, the shared schema exposes `body`, and the handler maps `source_cwd`, actor fields, `confirm`, and body into the same core workflow. Assert the tool count is unchanged.

- [ ] **Step 3: Verify RED**

Run:

```bash
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./internal/adapter/mcp -run 'Handoff|ModificationRequest|ActionParity' -count=1
```

Expected: FAIL because the surface is absent.

- [ ] **Step 4: Implement the CLI handler**

Add the subcommand to `issueOpsHandoffUsage` and the switch. The handler constructs the core request and calls the narrow adapter through the same injectable pattern used for worker-done projection. It must not call raw orchestration logic from the CLI package.

- [ ] **Step 5: Extend the MCP action-discriminated tool**

Add `request-modification` to the enum, add a bounded `body` property, route the action to the facade with `IssueOpsModificationClient()`, and update the invalid-action diagnostic. Preserve all existing actions and field names.

- [ ] **Step 6: Verify GREEN**

Run the focused command from Step 3. Expected: PASS.

- [ ] **Step 7: Commit only if authorized**

Suggested subject:

```text
feat(issueops): expose handoff correction requests
```

---

### Task 5: Tighten lifecycle authority and raw Orca targeting

**Files:**
- Modify: `internal/core/commandparse/issueops.go`
- Modify: `internal/core/commandparse/issueops_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_authority.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_ownership_authority_test.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target.go`
- Modify: `internal/core/lifecycle/lifecycle_handoff_resource_target_test.go`

**Interfaces:**
- Allows only the typed source `handoff request-modification` mutation in the active published handoff.
- Adds CLI `feedback add` to the exact owner recorder allowlist, matching the already-supported MCP owner flow.
- Treats raw orchestration sender/recipient terminal identities as ownership resources.

- [ ] **Step 1: Add exact command parser RED tests**

Add `IssueOpsCommandSpec` entries/tests for:

```go
case "handoff request-modification":
    return v("--id", "--host", "--session-id", "--agent-id", "--source-cwd", "--body"), b("--confirm", "--json"), r, true
case "feedback add":
    return v("--id", "--source", "--body", "--classification", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
```

Pin unknown flags, duplicate body, missing body, and removed/raw aliases as rejected.

- [ ] **Step 2: Add lifecycle authority RED tests**

Prove:

- a fresh exact-source actor can invoke only the fully formed typed request command/MCP action;
- the sealed owner cannot invoke that source request path;
- source attempts to call `feedback add`, phase, edit, Git, publish, or raw Orca send remain blocked;
- the active owner can invoke CLI `feedback add` with exact flags and then existing phase/implement/commit/publish flows;
- an unrelated ordinary source operation remains outside the fence;
- wrong cycle ID, root, actor identity, or missing confirmation fails closed.

- [ ] **Step 3: Add raw identity targeting RED tests**

Extend the current CLI/MCP resource matrix with:

```text
orca orchestration send --to <worker-mailbox> ...
orca orchestration send --from <coordinator-mailbox> ...
```

and MCP inputs using `to`, `from`, `to_handle`, and `from_handle`. Each persisted worker/coordinator terminal or mailbox identity must select its active record and be blocked unless it is the typed IssueOps path. Unrelated literal handles remain ordinary source work; dynamic/non-literal identities fail closed.

- [ ] **Step 4: Verify RED**

Run:

```bash
go test ./internal/core/commandparse ./internal/core/lifecycle -run 'OwnerRecorder|ModificationRequest|ResourceTarget|Ownership' -count=1
```

Expected: FAIL because the new allowlist and identity fields are absent.

- [ ] **Step 5: Implement the narrow policy changes**

Add the two exact command specs. In the handoff authority switch, allow `request-modification` only after exact flag parsing plus current-record/source-actor validation; never treat it as a generic owner recorder. Add `feedback add` to `allowedOwnerLifecycleRecorder` using its real CLI flag contract.

Extend orchestration target extraction to inspect sender/recipient flags and MCP keys, representing them as terminal identity targets so existing `protectedOrcaIdentityOwns` comparison is reused. Do not add a general raw-send exception.

- [ ] **Step 6: Verify GREEN**

Run the focused command from Step 4. Expected: PASS.

- [ ] **Step 7: Commit only if authorized**

Suggested subject:

```text
fix(issueops): fence handoff correction authority
```

---

### Task 6: Pin owner fast-forward republish behavior

**Files:**
- Modify: `internal/core/issueops/issueops_handoff_publication_test.go`
- Modify only if a RED test exposes a defect: `internal/core/issueops/issueops_handoff_publication.go`

**Interfaces:**
- Uses the existing optional `IssueOpsPublicationAncestryReader` and owner publish flow.
- Replaces the stored publish receipt only after a verified fast-forward publication of the current owner head.

- [ ] **Step 1: Add a focused table-driven regression**

Start from a valid owner-active PR-phase record with an old publish receipt and verified remote artifact. The cases are:

| Case | Ancestry result | Expected push | Expected receipt |
|---|---|---:|---|
| same head | not needed | 0 | unchanged/idempotent |
| old head is ancestor of new head | true | 1 | replaced with new head |
| non-fast-forward | false | 0 | unchanged |
| ancestry reader returns error | error | 0 | unchanged |
| reader lacks ancestry capability | unavailable | 0 | unchanged |
| actor/worker/branch/remote identity mismatch | irrelevant | 0 | unchanged |

The fake publication reader must count pushes separately from ancestry reads so every failure branch can assert no remote mutation.

- [ ] **Step 2: Run RED or characterization**

Run:

```bash
go test ./internal/core/issueops -run 'HandoffPublish.*(FastForward|Replacement|Idempotent)' -count=1
```

Expected: PASS if the current implementation already satisfies the design. If any case fails, retain the test as RED and patch only the smallest publication branch required.

- [ ] **Step 3: Verify receipt and remote-artifact consistency**

Assert provider, project key, remote, branch, base, remote ref, push target hash, final head, and verification timestamp are coherent after replacement. A failure must preserve both prior receipt and remote-artifact evidence byte-for-byte.

- [ ] **Step 4: Re-run the full publication package tests**

Run:

```bash
go test ./internal/core/issueops -run 'HandoffPublish|Publication' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit only if authorized**

Suggested subject:

```text
test(issueops): pin fast-forward handoff republish
```

If production code changed, use `fix(issueops)` and explain the exposed defect in the Lore body.

---

### Task 7: Align operator docs, skill guidance, and public goldens

**Files:**
- Modify: `.issueops/AGENT_WORKFLOW.md`
- Modify: `.issueops/ARCHITECTURE.md`
- Modify: `.issueops/OPERATIONS.md`
- Modify: `.issueops/TESTING.md`
- Modify: `.issueops/CAUTIONS.md`
- Modify: `skills/issueops/references/worktree-context.md`
- Modify: `cmd/issueops/testdata/usage.golden.txt`
- Modify: `cmd/issueops/testdata/response_contracts.golden.json`
- Modify only if generated contract content requires it: `cmd/issueops/testdata/mcp_tools.golden.json`

**Interfaces:**
- Documents the typed source-to-owner correction channel and the owner feedback/implement/republish loop.
- Removes stale guidance that permits raw terminal/orchestration steering for modification requests.
- Updates public CLI/MCP/response snapshots only from verified test generators.

- [ ] **Step 1: Update docs from one lifecycle sequence**

Document this exact sequence consistently:

1. PR is already published and the handoff is `owner_active` in phase `pr`.
2. A fresh exact-source session invokes `handoff request-modification --confirm`.
3. The durable projection records `intent` then `sent` or `failed`; same-key calls never retry.
4. The owner receives the typed status message, records `feedback add`, moves `feedback -> implement`, edits/tests/cleans, and commits locally.
5. The owner invokes `handoff publish --confirm`; only a verified fast-forward replaces the receipt.
6. Direct source mutation, raw Orca steering, and direct Git push remain blocked.

State the 32-entry cap, redaction/body bounds, crash tombstone, and the fact that a fresh source session need not be the original coordinator.

- [ ] **Step 2: Remove the raw guidance exception**

Search first:

```bash
rg -n 'terminal send|orchestration send|continue|resume|modification|feedback add|handoff publish' .issueops skills/issueops/references/worktree-context.md
```

Replace only statements that authorize free-form correction steering. Preserve the narrow existing wake/resume and cancellation/recovery contracts where they serve different states.

- [ ] **Step 3: Verify the skill source**

Run:

```bash
python3 scripts/validate-skill.py skills/issueops
```

Expected: PASS.

- [ ] **Step 4: Refresh goldens through their owning tests**

Run the normal golden tests first and inspect the diff. Use the repository's supported update switch only for intended public-surface changes:

```bash
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run Golden -update -count=1
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
```

Expected: first run reports only the intended handoff surface drift; update changes the relevant snapshots; final run passes. Inspect every golden diff and reject unrelated churn.

- [ ] **Step 5: Run docs and contract scans**

```bash
git diff --check
rg -n 'request-modification|modification_requests|feedback add|fast-forward' .issueops skills/issueops docs/superpowers/specs/2026-07-22-issueops-handoff-modification-request-design.md
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./internal/adapter/mcp -count=1
```

Expected: no whitespace error; the typed lifecycle appears in every required contract; tests pass.

- [ ] **Step 6: Commit only if authorized**

Suggested subject:

```text
docs(issueops): document handoff correction loop
```

---

### Task 8: Run the final verification gate

**Files:**
- No planned edits. Any failure must return to the owning task; do not patch around the gate.

- [ ] **Step 1: Inspect exact scope**

```bash
git status --short
git diff --stat
git diff --check
```

Expected: only files named in this plan are changed, and there are no whitespace diagnostics.

- [ ] **Step 2: Run the focused cross-layer suite**

```bash
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/adapter/orca ./internal/core/lifecycle ./internal/core/commandparse ./internal/adapter/mcp ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full tests, race detector, and build**

```bash
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
```

Expected: all commands exit 0. Re-run `git status --short` and ensure the built binary did not create an unintended tracked diff.

- [ ] **Step 4: Re-run the public contracts**

```bash
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
```

Expected: PASS with no unreviewed golden drift.

- [ ] **Step 5: Optional disposable live Orca E2E**

Only run this if Orca readiness is proven and the user separately authorizes creation and cleanup of disposable resources. Do not reuse a user terminal, task, dispatch, IssueOps record, or worktree.

The E2E must prove: fresh source request -> one status message -> durable `sent` projection -> same-key no-op -> owner feedback/edit/commit -> fast-forward republish -> updated receipt. Cleanup remains a separate approval boundary.

- [ ] **Step 6: Final audit**

```bash
git diff --name-only 8ec5ae7...HEAD
git log --oneline 8ec5ae7..HEAD
git status --short
```

Expected: an auditable sequence of authorized atomic commits, no unrelated paths, no push unless separately approved, and a clean or explicitly explained worktree.
