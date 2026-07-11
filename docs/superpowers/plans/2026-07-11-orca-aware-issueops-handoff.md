# Orca-Aware IssueOps Supervised Handoff Implementation Plan

> **For the fresh worktree worker:** REQUIRED SUB-SKILLS: use `superpowers:test-driven-development` for every behavior change and `superpowers:verification-before-completion` before any completion claim. Execute this plan in the linked worktree only. Do not ask the coordinator to edit implementation files.

**Issue:** [#16](https://github.com/m16khb/agent-harness/issues/16)
**IssueOps cycle:** `io-47c93d1ef742`
**Branch:** `16-orca-supervised-handoff`
**Worktree:** `/Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-supervised-handoff`

**Goal:** When the complete read-only Orca probe succeeds, let the coordinator prepare the issue, provider-linked branch, worktree, plan, and bounded context, then transfer one fenced implementation lease to a fresh Orca-hosted agent session. If Orca is absent or unready before mutation, preserve the existing inline IssueOps behavior and JSON contract.

**Architecture:** IssueOps remains the single durable authority. Add one optional handoff record and a small operation journal to the existing schema-v1 record. A concrete `internal/adapter/orca` implements only spike-verified CLI projections. Core state transitions and compare-and-set fencing are host-neutral. External commands never run while the IssueOps span lock is held. Hooks only parse identity, render claim guidance, and block unauthorized mutations. CLI and MCP are thin adapters over the same request/result DTOs.

**Design source:** `docs/superpowers/specs/2026-07-11-orca-aware-issueops-handoff-design.md` wins if this plan omits detail. Any implementation-driven deviation must update both files and receive a new Brooks review before proceeding.

## Baseline and fixed constraints

- `go test ./... -count=1` currently fails on `TestResponseContractsGolden` at `docs_count: got 78, want 77`. The same failure reproduces on untouched `main` at `2ba240b`; the preceding tracked plan document was not reflected in the golden. Do not treat this as a feature regression. The final intentional contract regeneration must include this pre-existing one-document delta and the new handoff surfaces.
- The live Orca spike proved duplicate worktree names and duplicate task titles create distinct artifacts. There is no automatic create retry anywhere in this implementation.
- Persist `pending_operation` (kind, started-at, and bounded pre-mutation identity baseline) under the current attempt **before** every Orca create/dispatch mutation. If a process restarts with a pending operation, the next command records/returns `recovery_required`; it does not invoke that operation again.
- The six lease states are exactly `coordinator_preparing`, `dispatched`, `claimed`, `submitted`, `closed`, and `recovery_required`. `closed_disposition` is exactly `accepted`, `worker_failed`, or `cancelled`.
- Missing handoff state means legacy inline behavior. Do not create a durable handoff field for `auto` probe fallback or explicit `inline` mode.
- `auto` falls back only after a read-only probe failure. Explicit `orca` returns an error. After any mutation may have run, neither mode falls back inline.
- Reconciliation persists only an exact-one match. Zero or multiple marker/task/PTY candidates stay fail-closed. Reconciliation never executes the next operation and never imports completion from Orca.
- `issueops resume` remains observational for Orca handoffs. Preserve legacy inline `--bind` behavior; reject `--bind` when a handoff field exists instead of silently changing legacy behavior.
- Coordinator preparation exceptions end when dispatch is persisted. A claimed worker is the only session allowed implementation mutations inside the worktree. The coordinator retains accept, feedback, PR, merge, and cleanup authority.
- No hook creates issues, branches, worktrees, terminals, tasks, dispatches, state claims, heartbeats, or background polls.
- No Orca installation/update/readiness requirement is added to bootstrap, install-native, update, ordinary hooks, or self-verify.
- Do not repair the unrelated legacy workpool reminder defect in this branch.

## Turing criteria

| ID | Binary pass condition | Primary artifact |
|---|---|---|
| ORCA-01 | Missing/unreachable Orca in `auto` returns the legacy inline result, performs probe-only calls, and leaves `execution_handoff` absent. | fake-runner trace + legacy fixture |
| ORCA-02 | Explicit `orca` probe failure returns error before mutation. | fake-runner trace |
| ORCA-03 | Ready Orca prepares and dispatches one worktree, terminal, task, and dispatch at most once. | crash matrix call counts + persisted IDs |
| ORCA-04 | Timeout/error after mutation invocation yields `recovery_required`, zero inline actions, and no automatic retry. | fake-runner crash matrix |
| ORCA-05 | Worktree/task marker and terminal PTY delta accept exactly one candidate; zero/multiple candidates fail closed. | table-driven recovery fixtures |
| ORCA-06 | Only a matching fresh worker can claim; readiness is blocked before claim and stale attempt/session/context tuples fail CAS. | core transition tests |
| ORCA-07 | Coordinator/wrong-session/out-of-tree mutations block; claimed in-tree worker mutations pass. | lifecycle and hook adapter tests |
| ORCA-08 | Finish, submit, accept, failure, cancel, and retry obey the actor/state/idempotency table. | table-driven transition tests |
| ORCA-09 | Resume causes no state or external mutation; only explicit recover persists one unique identity. | before/after record hash + fake trace |
| ORCA-10 | Missing/zero schema legacy records remain readable and inline; future schemas still fail safe. | schema fixtures |
| ORCA-11 | Context is deterministic, <=64 KiB, redacted, and changes hash when stable source inputs change. | context golden/hash fixtures |
| ORCA-12 | Codex, Claude, and GJC forward native session identity and each produces a real ownership block result. | installed-host smoke receipts |
| ORCA-13 | The installed Orca completed path launches a fresh agent, joins a submitted result, and removes disposable worktree/branch/terminal resources. | live E2E transcript + cleanup receipt |
| ORCA-14 | Focused/full/race/build/goldens/self-verify gates pass. | final verification transcript |

## TDD execution protocol

For every task below:

1. Add the named behavior test first.
2. Run the narrow command and capture a failure caused by the missing behavior, not a compile typo.
3. Implement the smallest production slice that makes that test pass.
4. Re-run the narrow test, then its package suite.
5. Review `git diff --check` and the task diff before committing.

Do not regenerate goldens until Task 8 registers the final CLI/MCP shape. Do not weaken assertions to accommodate implementation output.

### Task 1: Durable handoff model, operation journal, transition table, and context projection

**Files:**

- Modify: `internal/core/issueops/model/types.go`
- Modify: `internal/core/issueops/package.go`
- Modify: `internal/core/issueops/issueops_readiness.go`
- Create: `internal/core/issueops/handoff/state.go`
- Create: `internal/core/issueops/handoff/context.go`
- Create: `internal/core/issueops/handoff/state_test.go`
- Create: `internal/core/issueops/handoff/context_test.go`
- Extend: `internal/core/issueops/issueops_schema_version_test.go`

**Interfaces:**

- Add the optional `ExecutionHandoff *IssueOpsExecutionHandoff \`json:"execution_handoff,omitempty"\`` to `IssueOpsRecord`; keep `IssueOpsCurrentSchemaVersion = 1`.
- Model the protocol/state/disposition, monotonic attempt, random ownership epoch, context version/hash, coordinator/worker roots, native host session, Orca identity, pending operation, bounded result/failure, and timestamps from the design.
- `pending_operation` holds only the current attempt's operation kind and the pre-mutation worktree/task/PTY ID baseline; it is not a seventh lease state.
- Pure handoff functions validate the actor/source state and return a copied record; package-level wrappers own locks and persistence.
- Context projection uses stable source fields only, canonical JSON hashing, `policy.RedactFreeform`, sorted/deduplicated lists, plan file SHA-256, and a hard 64 KiB rendered limit.
- `IssueOpsImplementationReadiness` adds `handoff_worker_claim` only when a handoff exists; inline records retain the exact old missing-key set.

**RED tests:**

- `TestLegacyIssueOpsRecordWithoutExecutionHandoffRemainsInline`
- `TestIssueOpsExecutionHandoffTransitionTable`
- `TestIssueOpsExecutionHandoffRejectsStaleAttemptEpochAndContext`
- `TestIssueOpsExecutionHandoffPendingOperationSurvivesRoundTrip`
- `TestIssueOpsHandoffContextDeterministicAndBounded`
- `TestIssueOpsHandoffContextRedactsSecrets`
- `TestIssueOpsHandoffContextHashChangesForPlanBranchIntentAndWorktree`

**Commands:**

```bash
go test ./internal/core/issueops/handoff ./internal/core/issueops -run 'TestLegacyIssueOpsRecord|TestIssueOpsExecutionHandoff|TestIssueOpsHandoffContext' -count=1
```

**Commit:** `feat(issueops): add durable supervised execution lease`

### Task 2: Concrete Orca adapter and read-only capability probe

**Files:**

- Create: `internal/port/orca.go`
- Create: `internal/adapter/orca/client.go`
- Create: `internal/adapter/orca/runner.go`
- Create: `internal/adapter/orca/decode.go`
- Create: `internal/adapter/orca/client_test.go`
- Create: `internal/adapter/orca/testdata/` JSON fixtures only for spike-verified envelopes

**Interfaces:**

- Define one narrow `port.OrcaClient` used by the handoff use case; do not add a driver registry.
- The real runner uses `exec.CommandContext`, fixed per-operation timeouts, argv slices, explicit cwd, and separate stdout/stderr buffers. Decode stdout only; redact/truncate stderr diagnostics.
- Probe in order: `LookPath`, `orca status --json`, runtime reachable/state, graph state when present, exact repo resolution, required help command support, and built-in host launch support. Never call `orca --version`, `orca open`, install, or update.
- Implement narrow projections for status, repo, worktree list/create/remove, terminal list/create, task list/create/update, dispatch/show, and message send/check. Never persist the top-level RPC correlation `id` as a domain ID.
- Safe argv builders allow only built-in host mappings (`codex`, `claude`, `gjc` when verified); no arbitrary command string.

**RED tests:**

- `TestProbeDoesNotUseVersionOrMutate`
- `TestProbeRequiresReachableReadyRuntimeAndGraph`
- `TestClientDecodesStdoutWithHandshakeNoiseOnStderr`
- `TestClientRejectsMalformedOrOversizedEnvelope`
- `TestClientBuildsSpikeVerifiedArgvWithoutShell`
- `TestClientRefreshesTerminalHandleByWorktreeAndPTY`

**Commands:**

```bash
go test ./internal/adapter/orca ./internal/port -count=1
```

**Commit:** `feat(orca): add bounded optional cli adapter`

### Task 3: Coordinator worktree preparation with pre-mutation fallback and crash fencing

**Files:**

- Create: `internal/core/issueops/issueops_handoff_prepare.go`
- Create: `internal/core/issueops/issueops_handoff_prepare_test.go`
- Modify: `cmd/harness/issueopscli/worktreecmd/worktree.go`
- Extend: `cmd/harness/issueopscli/worktreecmd/worktree_test.go`
- Modify: `cmd/harness/issueopscli/dependencies.go`

**Interfaces:**

- Extend `issueops worktree prepare` with `--orchestrator auto|orca|inline`, `--agent`, and `--confirm`; default is `auto`, and no `--confirm` remains a preview.
- Inline/auto-fallback output keeps the existing `ok,id,repo,branch,base_branch,worktree_path,exists,command,next_step` fields. Optional orchestration fields use `omitempty`.
- Confirmed Orca preparation verifies IssueOps design/issue/provider-branch prerequisites, writes `coordinator_preparing`, attempt, epoch, and `pending_operation=worktree_create` under the cycle lock, releases the lock, invokes Orca once, verifies exact branch/path/lineage/linked issue, then CAS-persists the Orca worktree identity and links the worktree.
- A repeated call with an active handoff returns status/recovery guidance and makes zero create calls.
- A restart that sees `pending_operation=worktree_create` records/returns `recovery_required`; it never assumes the call did not happen.

**RED tests:**

- `TestWorktreePrepareAutoProbeFailurePreservesLegacyInlineResult` (ORCA-01)
- `TestWorktreePrepareExplicitOrcaProbeFailureHasProbeOnlyTrace` (ORCA-02)
- `TestWorktreePreparePreviewNeverMutates`
- `TestWorktreePrepareReadyOrcaCreatesExactlyOnce`
- `TestWorktreePrepareCrashAfterInvocationNeverCreatesTwice`
- `TestWorktreePrepareRejectsReturnedBranchPathOrInstanceMismatch`
- `TestWorktreePrepareExactOneMarkerRecovery` with zero/one/multiple rows

**Commands:**

```bash
go test ./internal/core/issueops ./cmd/harness/issueopscli/worktreecmd -run 'TestWorktreePrepare' -count=1
```

**Commit:** `feat(issueops): prepare optional Orca worktree exactly once`

### Task 4: Dispatch preparation, fresh terminal, task, and delivery

**Files:**

- Create: `internal/core/issueops/issueops_handoff_dispatch.go`
- Create: `internal/core/issueops/issueops_handoff_dispatch_test.go`
- Extend: `internal/core/issueops/handoff/context.go`

**Interfaces:**

- Add a pre-dispatch readiness projection containing all implement-entry gates except the future worker claim.
- `StartIssueOpsHandoff` renders/persists context version/hash, then executes terminal-create, task-create, and dispatch one at a time.
- For Codex only, probe installed support for `--dangerously-bypass-hook-trust` and require an explicit per-attempt context attestation before confirmed dispatch. The skill owns the read-only `hooks/list` review; do not embed Codex app-server/fingerprint logic in core.
- Expose required/attested state in preview. After review, a second attested no-confirm preview supplies the reviewed context hash; the final request adds only confirm and must render the same hash. Keep ContextVersion 1 and reset only the attestation on retry.
- Before each call, persist `pending_operation` plus the relevant bounded ID baseline. After each success, CAS-persist its domain identity and clear the pending operation before starting the next one.
- Terminal recovery accepts exactly one new PTY relative to the persisted baseline. Task recovery accepts exactly one new task carrying the attempt/epoch marker. Dispatch recovery uses only the persisted task ID and `dispatch-show`.
- V1 delivery is a recognized built-in host terminal plus `dispatch --inject --return-preamble`. Persist delivery mode `inject` and the refreshed exact assignee before dispatch; `dispatch-show` recovery validates its exact identity and `dispatched` status without relying on an absent `injected` field. There is no V1 `terminal send` fallback and no arbitrary shell command.
- Return after `dispatched`; no wait loop, automatic claim, or automatic next operation.

**RED tests:**

- `TestHandoffStartRequiresPreDispatchReadiness`
- `TestHandoffStartPersistsStableContextBeforeMutation`
- `TestHandoffStartCreatesTerminalTaskDispatchExactlyOnce`
- `TestHandoffStartCrashMatrixNeverRepeatsCreate` with before-invoke / after-side-effect / before-persist / after-persist rows for all three operations
- `TestHandoffStartTerminalDeltaRequiresExactlyOne`
- `TestHandoffStartTaskMarkerRequiresExactlyOne`
- `TestHandoffStartDispatchRecoveryRequiresPersistedTask`

**Commands:**

```bash
go test ./internal/core/issueops -run 'TestHandoffStart' -count=1
```

**Commit:** `feat(issueops): dispatch fresh Orca worker with crash fencing`

### Task 5: Worker claim, heartbeat, finish, coordinator accept, and explicit recovery

**Files:**

- Create: `internal/core/issueops/issueops_handoff_lifecycle.go`
- Create: `internal/core/issueops/issueops_handoff_recovery.go`
- Create: `internal/core/issueops/issueops_handoff_lifecycle_test.go`
- Create: `internal/core/issueops/issueops_handoff_recovery_test.go`
- Modify: `internal/core/issueops/issueops_heartbeat.go`
- Modify: `internal/core/issueops/package.go` (`IssueOpsResume` projection only)
- Extend: `internal/core/issueops/issueops_resume_heartbeat_cli_test.go` as appropriate for core behavior

**Interfaces:**

- Add request DTOs carrying cycle ID, attempt, epoch, context hash, native host/session/agent identity, canonical cwd/root, and outcome evidence.
- Claim re-renders the stable context, verifies branch/root/Orca locator, and transitions `dispatched -> claimed`; identical owner repeats succeed, different owners fail.
- Extend heartbeat arguments additively. Inline callers can still send only `id`; handoff callers must match the full claimed worker tuple.
- Completed finish validates bounded HEAD/files/Turing report/command evidence/cleanup receipts, writes IssueOps first, and transitions `claimed -> submitted`. Failed finish closes with `worker_failed`. Identical finish repeats are idempotent; conflicting repeats fail.
- Accept revalidates HEAD/evidence/context and transitions `submitted -> closed/accepted`.
- Recover supports `reconcile`, confirmed `cancel`, and confirmed `retry`. Reconcile persists one identity only and returns the next explicit command. Cancel closes before cleanup. Retry increments attempt and changes epoch only after the prior attempt is safely terminal/reconciled.
- Orca handoff resume is read-only; a before/after serialized record and fake trace must be byte-equivalent. Legacy inline `--bind` remains available.

**RED tests:**

- `TestHandoffClaimRequiresMatchingWorkerTuple`
- `TestHandoffClaimIsIdempotentForSameOwnerOnly`
- `TestHandoffHeartbeatFencesStaleWorker`
- `TestHandoffFinishSubmitAcceptLifecycle`
- `TestHandoffFinishFailureClosesWorkerFailed`
- `TestHandoffFinishAndAcceptIdempotency`
- `TestHandoffCancelClosesBeforeCleanup`
- `TestHandoffRetryUsesNewAttemptAndEpoch`
- `TestHandoffRecoverExactOneOnlyAndNeverAdvances`
- `TestHandoffResumeIsReadOnly`
- `TestInlineHeartbeatAndResumeRemainCompatible`

**Commands:**

```bash
go test ./internal/core/issueops -run 'TestHandoff|TestInlineHeartbeatAndResume' -count=1
```

**Commit:** `feat(issueops): fence worker lifecycle and result join`

### Task 6: Common hook identity, SessionStart claim guidance, and PreToolUse ownership fence

**Files:**

- Modify: `cmd/harness/hookcli/hookinput/hook_input.go`
- Extend: `cmd/harness/hookcli/hookinput/hook_input_test.go`
- Modify: `internal/core/lifecycle/model/types.go`
- Modify: `internal/core/lifecycle/lifecycle_state.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_guard.go`
- Create: `internal/core/lifecycle/lifecycle_handoff_guard_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_use.go`
- Modify: `cmd/harness/hookcli/hookcatalog/catalog.go`
- Create/extend: `cmd/harness/hookcli/hook_handoff_ownership_test.go`, `hook_prompt_session_test.go`

**Interfaces:**

- Parse exact `cwd`, `session_id`, optional `agent_id`/`agent_type`, and host without reading transcript contents or tokens.
- Add those fields to `HookToolUseLifecycleRequest`; `runHookPreToolUse` passes them through.
- Evaluate the handoff ownership fence before the existing general worktree/mirror guards whenever mutation is possible.
- Before claim, allow only narrowly parsed coordinator preparation commands and the exact claim command; block implementation mutation. After claim, allow mutation only for the matching native worker session in the canonical worker root. Always block worktree escape and coordinator absolute-path writes.
- SessionStart renders the exact claim/resume command and role/attempt/context facts. It does not mutate state. Join this guidance even when the project-doc catalog is empty; compact-source behavior remains unchanged.
- Legacy inline records follow the old guard path byte-for-byte.

**RED tests:**

- `TestHookInputParsesCodexClaudeNativeSessionIdentity`
- `TestHandoffGuardBlocksBeforeClaim`
- `TestHandoffGuardAllowsMatchingClaimedWorkerInTree`
- `TestHandoffGuardBlocksCoordinatorAbsolutePathIntoWorkerTree`
- `TestHandoffGuardBlocksWrongOrRestartedSession`
- `TestHandoffGuardBlocksWorkerEscape`
- `TestHandoffGuardAllowsExactLifecycleCommandsOnly`
- `TestSessionStartRendersClaimWithoutMutation`
- Existing linked-worktree hook tests remain green for inline cycles.

**Commands:**

```bash
go test ./cmd/harness/hookcli/hookinput ./internal/core/lifecycle ./cmd/harness/hookcli -run 'TestHookInput|TestHandoffGuard|TestSessionStart' -count=1
```

**Commit:** `feat(hooks): enforce supervised handoff ownership`

### Task 7: GJC native HookAPI parity and installed-host contracts

**Files:**

- Modify: `gjc-plugin/hook.ts`
- Modify: `internal/adapter/gjc/install_hooks.go` comments only where contract changed
- Extend: `internal/adapter/gjc/install_test.go`
- Extend: `internal/adapter/install_contract_matrix_test.go`
- Update later with Task 10: `internal/adapter/testdata/native_install_contract_matrix.golden.json`

**Interfaces:**

- Verify the installed GJC 0.7.8 HookAPI type source before editing. Use `(event, ctx)` and `ctx.sessionManager.getSessionId()` / `ctx.cwd` (or the verified equivalent).
- Replace the repeated `context` mapping with the verified prompt-bearing `before_agent_start` event for user-prompt semantics.
- Send snake_case JSON to the harness stdin with `host: "gjc"`, session ID, cwd, agent/tool request fields, and no secret environment values.
- Await SessionStart and PreToolUse. Translate a harness raw `block/reason` result into the exact GJC HookAPI return shape verified from installed types. Other lifecycle events may remain bounded best-effort, but must consume child stdout/stderr and avoid zombies.
- Missing agent-harness stays non-fatal only for non-enforcement events; PreToolUse parse/transport failure must follow the existing host safety policy documented in the implementation.

**RED tests:**

- Installed shim content forwards `session_id`, `cwd`, `tool_name`, and `tool_input` via stdin.
- GJC PreToolUse mock returns the verified block shape when the harness says block.
- SessionStart mock relays claim guidance.
- The shim no longer maps every LLM `context` event as one user prompt.

**Commands:**

```bash
go test ./internal/adapter/gjc ./internal/adapter -run 'Test.*GJC|TestNativeInstallContract' -count=1
```

**Commit:** `fix(gjc): forward native session identity and hook blocks`

### Task 8: CLI, one MCP action tool, response DTO parity, and intentional goldens

**Files:**

- Create: `cmd/harness/issueopscli/issueops_handoff_cli.go`
- Modify: `cmd/harness/issueopscli/issueops.go`
- Modify: `cmd/harness/issueopscli/issueops_cli_support.go`
- Modify: `internal/adapter/cli/usage.go`
- Create: `cmd/harness/issueopscli/issueops_handoff_cli_test.go`
- Modify: `internal/adapter/mcp/issueops_lifecycle_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_handlers.go`
- Create: `cmd/harness/mcpcli/issueops/handoff_test.go`
- Update: `cmd/harness/testdata/mcp_tools.golden.json`
- Update: `cmd/harness/testdata/response_contracts.golden.json`
- Update any command-list/usage/contract goldens identified by the focused golden tests

**Interfaces:**

- CLI: `issueops handoff start|claim|finish|accept|recover`; start includes the explicit Codex-only `--allow-codex-hook-trust-bypass` attestation. Extend existing heartbeat and resume flags rather than duplicating them.
- Every external mutation command is preview/dry-run unless `--confirm` is present where the design requires confirmation.
- MCP: one `issueops_handoff` tool with `action: start|claim|finish|accept|recover`; handlers invoke the same core DTOs as CLI. Do not advertise five duplicate tools.
- JSON includes requested/resolved mode, state, disposition, attempt, context hash, redacted fallback/recovery code, and stable Orca domain IDs. CLI text derives from those DTOs.
- Orca handoff `resume bind=true` returns a normalized refusal without mutation; inline bind stays compatible.
- Regenerate response goldens once after all schemas stabilize. Review the diff and explicitly retain the pre-existing docs-count 77->78 correction plus this branch's intentional tool/field changes.

**RED tests:**

- `TestRunIssueOpsHandoffLifecycle`
- `TestRunIssueOpsHandoffRequiresConfirmationForMutation`
- `TestMCPIssueOpsHandoffLifecycleParity`
- `TestMCPIssueOpsHandoffUsesOneActionTool`
- `TestOrcaHandoffResumeBindRefusedReadOnly`
- Response-contract and MCP tool golden failures show only expected new surfaces.

**Commands:**

```bash
go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli/issueops ./internal/adapter/mcp -run 'Test.*Handoff' -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1
```

**Commit:** `feat(issueops): expose supervised handoff through cli and mcp`

### Task 9: Skill RED/GREEN contract and project operating docs

**Files:**

- Baseline evidence already created: `.agent-harness/research/orca-skill-baseline-2026-07-11.md`
- Modify: `skills/issueops/SKILL.md`
- Create: `skills/issueops/references/orca-handoff.md`
- Modify: `skills/issueops/references/worktree-context.md`
- Modify: `skills/turing/SKILL.md`
- Extend: `internal/core/issueops/issueops_skill_contract_test.go`
- Modify narrowly: `.agent-harness/ARCHITECTURE.md`, `AGENT_WORKFLOW.md`, `ADR.md`, `CAUTIONS.md`, `CONVENTIONS.md`, `TESTING.md`, `OPERATIONS.md`, `TECH_STACK.md`

**Interfaces:**

- IssueOps uses a conditional positive recipe: `auto` probe -> either unchanged inline execution with no handoff record, or coordinator prepare -> dispatch -> worker claim/heartbeat/finish -> coordinator accept.
- State the no-create-retry and exact-one recovery rules, worker stop boundary, and coordinator PR/cleanup ownership.
- Keep detailed commands/recovery in one reference so the already-large SKILL stays concise.
- Turing renders ORCA criterion IDs, requires the worker report and cleanup receipts at finish, and replaces the incorrect statement that `issueops heartbeat` is stale with the current command contract.
- Do not add Orca recipes to any other skill in V1.
- Document the optional adapter boundary, operation journal, host identity, lock/no-external-call invariant, fallback behavior, native smoke contract, and the deferred workpool defect in the appropriate project docs only.

**RED tests:**

- Extend `TestIssueOpsSkill...` assertions before editing the skill: mode triad, coordinator/worker commands, no-retry, exact-one recovery, worker stop boundary, and reference link.
- Add a Turing contract assertion for current `issueops heartbeat`, handoff result report, and ORCA criterion IDs.
- The existing fresh-context baseline in `.agent-harness/research/orca-skill-baseline-2026-07-11.md` is the writing-skills RED artifact.

**GREEN/forward test:**

- Run the same three fresh-context scenarios with the revised skill supplied. Pass only if agents use status (not version), distinguish pre/post-mutation failure, never retry a create, use exact-one recovery, and name claim/heartbeat/finish/accept ownership correctly.

**Commands:**

```bash
go test ./internal/core/issueops -run 'TestIssueOpsSkill|TestTuring' -count=1
python3 scripts/validate-skill.py skills/issueops
python3 scripts/validate-skill.py skills/turing
```

**Commit:** `docs(skills): teach optional Orca supervised handoff`

### Task 10: Turing evidence, native host smokes, live Orca E2E, and repository gates

**Files/artifacts:**

- Create: `.agent-harness/research/orca-handoff-turing-evidence-2026-07-11.md`
- Update only if implementation evidence requires it: design spec and this plan
- Runtime-only temp state belongs under `mktemp`/`HARNESS_STATE_DIR` and must be removed.

**Verification sequence:**

1. Run focused core/adapter/CLI/MCP/hook suites and record ORCA-01..12 mapping.
2. Build `bin/agent-harness`; run install-native into an isolated home first, then the real user installation only as the existing project verification contract requires.
3. Codex and Claude smokes: feed their verified native hook payloads with distinct `session_id` values into the installed hook command and assert host-native block output for coordinator/wrong-session writes.
4. GJC smoke: exercise the installed shim through a HookAPI mock or the narrowest installed GJC hook runner, proving native context reaches the harness and the verified GJC block shape returns.
5. Live Orca E2E in a disposable local git repo or uniquely named disposable branch: prepare a complete IssueOps record, run the new confirmed Orca preparation/start path, let the fresh agent claim and submit a bounded no-secret result, accept it, close the Orca task, and remove worktree/branch/terminal/repo temp state. Do not use global `orchestration reset`; completed task history may remain and must be named in the receipt.
6. Inspect the full diff for unrelated edits and run ai-slop-clean. Record categories and verification in IssueOps.
7. Run all repository gates below with fresh output.

```bash
gofmt -w <changed-go-files>
git diff --check
go mod tidy
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/adapter/orca ./internal/core/lifecycle ./internal/core/commandparse ./internal/core/skillcontract ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/issueopscli ./cmd/harness/harnessapp -count=1
go test ./... -count=1
go test -race ./... -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./scripts/install-native.sh
bun scripts/smoke-gjc-native-hook.ts "$HOME/.gjc/agent/hooks/agent-harness.ts"
./bin/agent-harness bootstrap --dry-run
./bin/agent-harness install-native --dry-run --json
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness daemon status --json
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
codex mcp get agent_harness
claude mcp list
gjc plugin list
gjc config get skills.enabled
gjc config get skills.customDirectories
```

**Required evidence report shape:**

```text
Success criteria: ORCA-01..ORCA-14, each PASS/FAIL with one binary observation
Evidence artifact: exact test/smoke/live transcript paths and relevant persisted record projections
Cleanup receipt: every temp repo/state dir/worktree/branch/terminal removed; completed Orca task ids noted
Verification mode: full Turing loop
Skipped checks: none, because Orca is ready in this environment
```

**Commit:** `test(issueops): prove Orca handoff compatibility and recovery`

## Coordinator handoff packet

The coordinator injects only:

- issue/cycle/branch/base/worktree identities;
- approved intent/design/compatibility/Brooks conclusions;
- this plan path and SHA-256;
- ORCA-01..14 criteria and verification commands;
- attempt/epoch/context hash and exact claim/heartbeat/finish templates;
- worker scope and stop conditions.

It does not inject the conversation transcript, environment dump, credentials, hook tokens, or unbounded source text.

## Worker stop condition

The fresh worker stops only after it has committed the scoped implementation, submitted `issueops handoff finish --outcome completed` with the Turing report and cleanup receipts, sent the bounded Orca `worker_done` evidence, and reported the commit/changed files to the coordinator. It must not push, open/merge a PR, accept its own handoff, delete the worktree, or clean the provider branch.
