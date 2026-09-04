# Orca Plan Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Orca owner를 준비·재봉인·재개하기 전에 child plan artifact를 봉인하도록 강제하고, released generation에 plan을 안전하게 추가한 뒤 reseed할 수 있는 복구 경로를 제공한다.

**Architecture:** Plan readiness는 host-neutral IssueOps core가 소유한다. Fresh Orca prepare는 worktree 생성 전 staged `plan`의 non-empty digest를 먼저 검증하고, workspace receipt 직후 이를 canonical child worktree의 `.issueops/artifact/plan.md`로 materialize하여 durable `PlanPath`로 원자적으로 기록한 다음에만 Run/task/dispatch 단계로 간다. 이미 linked `PlanPath`가 있다면 staged digest와 정확히 일치해야 한다. Reseal/resume은 기존 durable `PlanPath`와 staged/sealed plan digest의 exact identity를 새 generation 또는 외부 mutation 전에 재검증한다.

**Tech Stack:** Go 1.26, SQLite-backed IssueOps state, Orca CLI adapter, standard `testing`, CLI/MCP response-contract goldens.

## Global Constraints

- schema version은 current v1 exact만 지원하고 pre-v1 fallback이나 legacy alias를 추가하지 않는다.
- fresh prepare 전에는 non-empty staged `plan`이 필수다. Worktree가 아직 없을 수 있으므로 `PlanPath` 부재 자체는 fresh preflight 실패가 아니다.
- workspace receipt 뒤에는 plan이 canonical child worktree 내부의 non-empty regular file로 materialize/link되어야 한다. staged copy와 durable `PlanPath` 원본의 SHA-256이 같아야 한다.
- active/claimable holder가 있는 generation은 restage하지 않는다. recovery staging은 released Orca execution, holder 없음, pending/completion 없음에서만 허용한다.
- `parent_plan_path`는 readiness 입력에서 완전히 제외하며 child `PlanPath`로 복사·승격하지 않는다.
- missing/invalid staged plan의 fresh preflight 실패 시 Orca readiness probe 같은 읽기 전용 관찰은 허용하지만 `BeginIntent`, worktree 생성, operation ID allocation, lease activation은 일어나지 않아야 한다. Workspace receipt 뒤 materialization/persistence 실패는 이미 생긴 worktree intent를 reconcile 가능하게 보존하되 Run/task/dispatch/terminal과 lease activation으로 더 진행하지 않는다. Resume identity 실패는 모든 새 external mutation 전에 끝난다.
- direct mode의 기존 실행 의미와 GitHub/GitLab issue identity 검사를 바꾸지 않는다.

---

### Task 1: Plan artifact readiness contract

**Files:**
- Modify: `internal/core/issueops/issueops_artifact_stage.go`
- Modify: `internal/core/issueops/execution_prepare_bridge.go`
- Modify: `cmd/issueops/issueopsapp/issueops_preparation_wiring.go`
- Create: `internal/core/issueops/issueops_artifact_stage_test.go`
- Test: `internal/core/issueops/execution_owner_packet_test.go`
- Test: `cmd/issueops/issueopsapp/issueops_preparation_wiring_test.go`

**Interfaces:**
- Produces: `RequireStagedExecutionOwnerPlan(stateRoot string, record issueops.IssueOpsRecord) (PlanIdentity, error)`; fresh records may return a digest with no durable path, while a pre-linked path must already match.
- Produces: structured error code `orca_plan_artifact_required`, missing field `plan`, and `next_command` when `record.PlanPath` supplies an exact file.
- Consumes: existing `readStagedArtifacts`, `record.PlanPath`, `record.WorktreePath`, and `issueOpsPlanPathInsideWorktree`.

- [ ] **Step 1: Write failing tests**

Add a fresh-preflight table covering:

- staged artifact 없음
- `spec`만 staged
- staged `plan`은 있고 아직 worktree/`PlanPath`가 없는 fresh positive case
- pre-linked `PlanPath`가 없거나 directory이거나 canonical child worktree 밖
- pre-linked `PlanPath`는 plan-A를 가리키지만 staged `plan`은 plan-B인 digest mismatch
- pre-linked/staged non-empty plan이 같은 positive case

All negative cases return `orca_plan_artifact_required`. The missing staged-plan case does not invoke the remote issue reader and exposes, only when `record.PlanPath` is an existing in-worktree regular file:

```text
issueops artifact stage --id 'io-262' --name plan --file '/repo.worktrees/262-orca-plan-readiness/.issueops/issues/262/plan.md' --json
```

Do not emit a misleading stage command for missing, empty, directory, or out-of-worktree `PlanPath`. Explicitly assert `delegation.parent_plan_path` alone still fails. Conversely, a valid staged plan with no `WorktreePath`/`PlanPath` passes fresh preflight because prepare owns canonical workspace creation.

At the application service boundary, use counters to prove the Orca readiness probe may run once but `BeginIntent`, workspace creation, Run/task/dispatch invocation, and lease transition counters remain zero when plan identity is invalid.

- [ ] **Step 2: Observe RED**

Run:

```bash
go test ./internal/core/issueops ./cmd/issueops/issueopsapp -run 'PlanArtifact|Preparation.*Plan' -count=1
```

Expected: missing symbol or prepare path calls the issue reader despite no staged plan.

- [ ] **Step 3: Implement the minimum contract**

Read the staged artifact map and require a non-empty staged `plan`. If `record.PlanPath` is already present, canonicalize it inside `record.WorktreePath`, hash it, and require equality with the staged digest; if both are absent, do not invent a path before workspace creation. Return a typed error implementing:

```go
IssueOpsErrorFields() map[string]any
```

with `code`, `missing`, and optional `next_command`. Call this check from `ReadExecutionPreparationOwnerEvidence` before `readExecutionOwnerSnapshot`, passing `stateRoot` through the composition wiring. The helper itself performs no remote read or Orca operation. The enclosing preparation service may complete its already-existing read-only Orca probe, but must fail before `BeginIntent` and every external mutation.

- [ ] **Step 4: Verify GREEN**

Run the Task 1 focused command and confirm the helper's remote reader count is zero; at service level confirm probe count is at most one and every intent/mutation count is zero.

### Task 2: Seal plan digest in prepare and replacement

**Files:**
- Modify: `internal/core/issueops/issueops_artifact_stage.go`
- Modify: `internal/core/issueops/execution_prepare_bridge.go`
- Modify: `internal/core/issueops/execution_orca_intent.go`
- Modify: `internal/core/issueops/execution_lease.go`
- Modify: `internal/contract/issueopspreparation/prepare.go`
- Modify: `internal/adapter/outbound/issueopspreparation/repository.go`
- Test: `internal/core/issueops/issueops_artifact_stage_test.go`
- Test: `internal/core/issueops/execution_owner_packet_test.go`
- Create: `internal/core/issueops/execution_orca_intent_test.go`
- Create: `internal/core/issueops/execution_replace_test.go`
- Test: `internal/adapter/outbound/issueopspreparation/repository_orca_test.go`

**Interfaces:**
- Produces: `materializeExecutionOwnerArtifacts(stateRoot string, record issueops.IssueOpsRecord) (OwnerPlanIdentity, map[string]string, error)` that returns a canonical durable plan path and matching manifest.
- Produces: `preparationcontract.OwnerArtifacts.PlanPath`, persisted atomically with the accepted worktree receipt before the next Orca intent stage.
- Consumes: immutable `materializeStagedArtifacts` and `buildExecutionOwnerArtifacts`.

- [ ] **Step 1: Write failing tests**

Cover prepare owner artifact creation, worktree-receipt persistence, reconcile worktree receipt, and replacement reseal with: no staged plan, invalid/empty staged plan, missing staged copy, staged-copy digest mismatch, missing/outside durable `PlanPath`, and staged-vs-linked digest mismatch. For a fresh record with no `PlanPath`, assert the worktree receipt materializes `.issueops/artifact/plan.md`, returns it through `OwnerArtifacts.PlanPath`, and atomically persists it before the next external stage. For a pre-linked plan, preserve that path and require exact digest equality. Assert post-workspace failures create no Run/task/dispatch/terminal and do not advance the lease. Add positive fixtures whose manifest includes the exact 64-character SHA-256 of the durable linked file.

- [ ] **Step 2: Observe RED**

```bash
go test ./internal/core/issueops -run 'Owner.*Plan|Replace.*Plan|Intent.*Plan' -count=1
```

Expected: current empty manifest is accepted.

- [ ] **Step 3: Implement the minimum owner materializer**

Wrap `materializeStagedArtifacts` and require `manifest["plan"]`. On fresh prepare, materialize it at the deterministic artifact path after the workspace receipt, return that path in the preparation contract, and persist `record.WorktreePath` plus `record.PlanPath` in the same receipt transition before Run creation. If a durable linked path already exists, validate it against the staged digest and preserve its identity. Replacement reseal never creates or changes `PlanPath`; it requires the existing durable path to match. Remove the “empty manifest backward compatibility” production comment and behavior; generic staged materialization may remain reusable for direct mode.

- [ ] **Step 4: Verify GREEN**

Run Task 2 focused tests and `go test ./internal/application/issueopspreparation ./internal/adapter/outbound/issueopspreparation -count=1`.

### Task 3: Resume rejects unsealed plans before Orca mutation

**Files:**
- Modify: `internal/core/issueops/execution_resume.go`
- Test: `internal/core/issueops/execution_resume_identity_test.go`
- Test: `cmd/issueops/issueopsapp/issueops_resume_wiring_test.go`

**Interfaces:**
- Consumes: sealed `executionOwnerContextPacket.ArtifactManifest`.
- Produces: resume failure code `orca_plan_artifact_required` with replacement/recovery next action and no new operation ID, external intent, Run/task/dispatch call, or lease transition.

- [ ] **Step 1: Write failing tests**

Start from a valid sealed packet and cover this matrix while recomputing the outer packet digest as needed:

- `artifact_manifest.plan` missing
- manifest plan digest is not valid SHA-256
- sealed plan artifact file missing
- sealed artifact content does not match its manifest digest
- durable `PlanPath` file missing/outside canonical child worktree
- durable `PlanPath` digest does not match the sealed plan digest
- fully matching positive case

Assert every negative case fails before operation ID allocation and before `ResumeStage`, BeginIntent, Orca Run/task/dispatch, or lease calls.

- [ ] **Step 2: Observe RED**

```bash
go test ./internal/core/issueops ./cmd/issueops/issueopsapp -run 'Resume.*Plan' -count=1
```

Expected: the digest-valid empty manifest currently reaches resume orchestration.

- [ ] **Step 3: Implement the minimum validation**

Require a valid `plan` digest in `validateExecutionResumePacket`, verify the sealed artifact file and digest, then compare that digest with the current durable in-worktree `PlanPath` content. Reuse the typed plan readiness error so CLI and MCP expose the same `code`, `missing`, and recovery instruction. This validation runs before operation ID allocation or external intent persistence.

- [ ] **Step 4: Verify GREEN**

Run Task 3 focused tests and the execution response-contract goldens.

### Task 4: Released-generation recovery staging

**Files:**
- Modify: `internal/core/issueops/issueops_artifact_stage.go`
- Modify: `internal/core/lifecycle/lifecycle_execution_guard.go` only if the exact released-owner command is currently rejected
- Test: `internal/core/issueops/issueops_artifact_stage_test.go`
- Test: `internal/core/lifecycle/lifecycle_execution_matrix_test.go` only if guard behavior changes

**Interfaces:**
- Produces: `artifact stage --name plan` may update staging after execution only when mode is Orca, lease is released, holder/pending/completion are absent.
- Preserves: active, claimable, completed, direct, and pending executions remain fail-closed.

- [ ] **Step 1: Write failing recovery matrix**

Add table cases for no execution, released clean Orca, released Orca with holder, released Orca with pending/completion, active Orca, claimable Orca, revoking Orca, completed Orca, and released direct. Assert only no-execution and released clean Orca permit staging. A staged recovery artifact changes only the next reseal input; the currently sealed packet is byte-identical.

- [ ] **Step 2: Observe RED**

```bash
go test ./internal/core/issueops ./internal/core/lifecycle -run 'Artifact.*Released|Released.*Artifact' -count=1
```

Expected: released Orca is currently rejected solely because `Execution != nil`.

- [ ] **Step 3: Implement exact recovery predicate**

Replace the blanket post-prepare rejection with a small predicate covering the matrix. The success path updates staging only; it does not modify the existing packet. Error text and structured fields must require `execution replace --reseed` before resume. Do not add actor flags to `artifact stage`: the actual CLI accepts only `--id`, `--name`, `--file`, and `--json`.

- [ ] **Step 4: Verify GREEN**

Run Task 4 focused tests and confirm active/claimable near-misses remain blocked.

### Task 5: Docs, contracts, and evidence

**Files:**
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/execution.md`
- Modify: `skills/issueops/references/operational-start.md`
- Modify: `.issueops/OPERATIONS.md`
- Modify: `.issueops/TESTING.md` if the plan-ready regression command is not already covered
- Modify: `cmd/issueops/testdata/response_contracts.golden.json` only if the structured error changes the frozen surface
- Create: `.issueops/issueops/262-verified-execution-report.md`

**Interfaces:**
- Documents: fresh stage plan → prepare creates workspace → materialize/link durable plan → owner launch; live blocked owner self-release → link/stage plan → reseed → resume; non-cooperative owner → generation-fenced replacement only after quiescence.
- Records: AC mapping, RED/GREEN commands, full verification, live #248 Orca readback.

- [ ] **Step 1: Update active docs**

Remove language that treats empty owner artifact manifests as compatible and update the IssueOps skill's execution order. For a fresh Orca cycle, author the approved plan in a coordinator temporary file outside the source checkout, stage it, and then run prepare; after workspace creation, the materialized child file becomes the durable `PlanPath` and the temporary source may be removed. Document that prepare materializes/links this identity before owner launch; reseal/resume require staged/sealed and durable digests to match. Released-generation staging changes only the next reseal input. Document the actual `artifact stage` CLI flags without invented actor flags.

- [ ] **Step 2: Run verification**

```bash
go test ./internal/core/issueops ./internal/application/issueopspreparation ./internal/adapter/inbound/issueopspreparation ./internal/adapter/outbound/issueopspreparation ./cmd/issueops/issueopsapp ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/issueops ./cmd/issueops
git diff --check
```

- [ ] **Step 3: Run live #248 recovery**

Use lifecycle `io-268bd6ac6e7a`, generation 3, Run `run_a7765e771192`, task `task_e408c174ecd9`, dispatch `ctx_9c161cb76400`, and resolve the current runtime terminal handle from `orca orchestration dispatch-show` rather than treating durable PTY `ssh:ssh-1783678538888-sl28jq@@pty-10` as a runtime handle.

Required preconditions, all re-read immediately before mutation:

- #248 worktree HEAD is exactly `511457c2e56e73a2c7451ba547a8fd9cfa58ab74` and `git status --porcelain` is empty.
- branch link is verified; `plan_path` and sealed `artifact_manifest` are still empty.
- generation is exactly 3, completion and pending operation are absent, and the holder is session `019fc4df-5d94-7280-a171-16d15d1cb18c` / PID `42810`.
- Orca task/dispatch are failed and the runtime terminal is still connected+writable with that exact live process, or the replacement fallback has proven the process and Orca writer quiescent.

Primary clean-release sequence after building and installing the exact #262 binary:

```bash
orca orchestration dispatch-show --task task_e408c174ecd9
orca terminal send --terminal <runtime-handle-from-dispatch> --text "Release IssueOps lifecycle io-268bd6ac6e7a generation 3 now with your own sealed owner identity, report the JSON result, then stop without editing production files." --enter
orca terminal read --terminal <same-runtime-handle> --cursor <last-cursor> --limit 400 --json
issueops status --id io-268bd6ac6e7a --json
```

Proceed only when the owner itself has produced a successful release and status shows generation 3 `released`, holder nil, pending nil, completion nil. If the owner cannot release, record the failed release evidence, stop the exact worktree terminal with `orca terminal stop --worktree path:/Users/m16khb/Workspace/issueops.worktrees/248-orca-ready-issueops-dogfood --json`, wait on the resolved runtime handle with `orca terminal wait --terminal <runtime-handle> --for exit --timeout-ms 60000 --json`, prove PID/task/terminal quiescence, then use the documented `execution replace --preview` → `--revoke --confirm` → `--finalize-preview` → `--finalize --confirm` inventory/quiescence-fenced workflow. Do not impersonate the old holder and do not stage while active, revoking, or claimable. If this fallback ends claimable instead of released, abort live recovery and create a follow-up issue rather than bypassing the staging predicate.

In the released primary path, write and independently review an issue-specific child plan inside the #248 worktree without production edits, then run with coordinator actor fields returned by `execution whoami` where the command supports them:

```bash
issueops link-plan --id io-268bd6ac6e7a --plan-path /Users/m16khb/Workspace/issueops.worktrees/248-orca-ready-issueops-dogfood/.issueops/issues/248/plan-orca-ready-issueops-dogfood.md --host codex --session-id <coordinator-session-id> --cwd /Users/m16khb/Workspace/issueops.worktrees/248-orca-ready-issueops-dogfood --json
issueops artifact stage --id io-268bd6ac6e7a --name plan --file /Users/m16khb/Workspace/issueops.worktrees/248-orca-ready-issueops-dogfood/.issueops/issues/248/plan-orca-ready-issueops-dogfood.md --json
issueops execution replace --id io-268bd6ac6e7a --expected-generation 3 --preview <ACTOR_FLAGS> --json
issueops execution replace --id io-268bd6ac6e7a --expected-generation 3 --reseed --inventory-fingerprint <preview-fingerprint> <ACTOR_FLAGS> --confirm --json
issueops execution resume --id io-268bd6ac6e7a --expected-generation 4 <ACTOR_FLAGS> --confirm --json
```

Abort before each mutation if generation/holder/pending/completion, HEAD/dirty paths, branch link, exact installed binary SHA, Orca runtime version, or preview fingerprint differs. After resume, use Orca CLI `run-show`, `task-list`, `dispatch-show`, and runtime-handle `terminal read` to prove packet `artifact_manifest.plan` equals the child `PlanPath` SHA-256, generation 4 is claimed once, and phase reaches `implement`. The recovery coordinator does not edit production code; implementation remains owned by the newly claimed Orca owner.

- [ ] **Step 4: Record Turing evidence and review**

Map every #262 acceptance item to test names and live commands. Run AI-slop cleanup and a fresh xhigh Brooks implementation review before commit/push/PR.
