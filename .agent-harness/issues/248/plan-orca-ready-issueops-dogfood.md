# Orca-ready IssueOps Selection Receipt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` and `superpowers:test-driven-development` task-by-task. Keep production changes on lifecycle `io-268bd6ac6e7a`, generation 4, in the canonical #248 worktree.

**Goal:** Make `execution prepare --mode auto` the auditable IssueOps child default, preserve the complete mode/readiness decision as a durable current-v1 receipt, reject unreasoned explicit-direct execution, distinguish native-host smoke from Orca orchestration evidence, and complete this child through a real Orca-owned implementation cycle.

**Architecture:** The preparation domain owns the pure selection/readiness decision and fingerprint input. The preparation application owns preview/confirm comparison and selection time. The outbound repository atomically persists an additive `Selection` receipt with the first execution write. Inbound CLI/MCP adapters only map the shared contracts. Orca probing remains behind the application port; no host-specific or provider-specific decision logic moves into adapters. Existing current-v1 records without the additive receipt remain readable, but every newly prepared execution must persist it.

**Tech stack:** Go 1.26, SQLite-backed IssueOps state, Orca CLI adapter, shell-based dual-host smoke runner, standard `testing`, CLI/MCP response-contract goldens.

## Global constraints

- Do not add pre-v1 decoders, compatibility aliases, fallback parsers, or host-specific core branches.
- `auto` is the documented/default automated-child request. `direct` stays available only with a non-empty bounded explicit reason; it never probes Orca.
- Preview performs the same selection probe used by confirm and returns a deterministic readiness fingerprint. Confirm must receive the preview fingerprint, repeat the read-only selection, and fail before operation-ID allocation, repository mutation, worktree creation, or Orca mutation if the fingerprint differs.
- Fingerprint only normalized selection inputs and observed readiness facts. Every fingerprint includes requested mode, `probe_attempted`, `probe_available`, `probe_ready`, normalized probe code, fallback code, and normalized explicit-direct reason. Owner host/model/effort plus provider/issue identity are included only on the `auto`/`orca` probe paths that actually derive them; explicit direct does not invent or read provider identity. Do not include timestamps, absolute temporary paths, secrets, or mutable unrelated Orca inventory.
- Normalize an explicit-direct reason once in the shared domain contract with `strings.TrimSpace`; require valid UTF-8, 1–512 UTF-8 bytes after trimming, and reject every ASCII C0 control byte plus DEL. Persist and fingerprint those exact normalized bytes. CLI `next_command` uses the existing POSIX single-quote encoder and MCP carries the same normalized string, so neither adapter may reinterpret or normalize it independently.
- Persist selection time only on successful confirm. Preview is not durable state.
- The durable receipt's `resolved_mode` must equal `Execution.Mode`. Probe booleans/codes and direct reason must obey the decision matrix. Existing current-v1 records may have a nil additive receipt; new preparation write paths may not.
- Native host smoke and Orca orchestration are separate evidence lanes. A host smoke receipt must say `validation_lane=native_host`; it cannot satisfy the Orca worktree/Run/task/dispatch/claim requirements.
- Preserve current lease, intent, reconciliation, completion, publication, and response error meanings.

---

### Task 1: Define the additive selection receipt and decision matrix

**Files:**
- Modify: `internal/contract/issueops/execution.go`
- Modify: `internal/contract/issueopslease/`
- Modify: `internal/contract/issueopspreparation/prepare.go`
- Modify: `internal/domain/issueopspreparation/decision.go`
- Test: matching contract/domain tests

**Contract:**

Add a shared execution selection receipt containing:

```text
requested_mode, resolved_mode,
probe_attempted, probe_available, probe_ready, probe_code,
fallback_code, readiness_fingerprint,
selected_at, explicit_direct_reason
```

Expose the same readiness projection and fingerprint on prepare preview/confirm results. Add command inputs for `direct_reason` and `expected_readiness_fingerprint`.

- [ ] Write RED table tests for `auto` ready→Orca, `auto` unavailable/unready→direct with exact fallback code, explicit Orca failure, explicit direct with no probe, missing direct reason, invalid reason, and nil additive receipt decoding.
- [ ] Assert impossible receipt combinations fail validation: mode mismatch, available without attempted, ready without available, probe code on unattempted direct, fallback on Orca, missing fingerprint/time on a present receipt, or direct reason on non-explicit-direct.
- [ ] Implement the smallest normalized decision evidence type and deterministic SHA-256 fingerprint encoder. Keep hashing pure and stable; use an explicit ordered projection, not map JSON.
- [ ] Ensure clone/encode/decode paths deep-copy and round-trip the receipt without changing schema version 1.
- [ ] Run:

```bash
go test ./internal/contract/issueops ./internal/contract/issueopslease ./internal/contract/issueopspreparation ./internal/domain/issueopspreparation -run 'Selection|Readiness|DirectReason|Execution' -count=1
```

### Task 2: Enforce preview/confirm identity before mutation

**Files:**
- Modify: `internal/application/issueopspreparation/prepare.go`
- Modify: application ports/types as required
- Test: `internal/application/issueopspreparation/*_test.go`

- [ ] Write RED tests proving preview returns the full readiness projection/fingerprint and performs no write or external mutation.
- [ ] Write RED tests proving confirm requires the expected preview fingerprint, reruns the read-only selection, and rejects changed owner profile or provider/issue identity on `auto`/`orca` probe paths, changed probe outcome/code, changed explicit-direct reason, missing fingerprint, and malformed fingerprint before operation-ID allocation and before direct/Orca workspace calls. Explicit direct proves the inverse boundary: it performs no provider/issue read and fingerprints only the direct selection fields.
- [ ] Require the exact shared normalization contract (trimmed valid UTF-8, 1–512 bytes, no ASCII C0/DEL) whenever normalized requested mode is `direct`; do not probe Orca or derive provider identity on that path. `auto` fallback records the probe facts/fallback code and has no explicit-direct reason.
- [ ] On successful confirm, obtain one clock value, construct the durable receipt, and pass it unchanged to `CommitDirect` or `BeginIntent`.
- [ ] Preserve existing-execution status/idempotency behavior; do not overwrite or synthesize an existing durable receipt during a read.
- [ ] Run:

```bash
go test ./internal/application/issueopspreparation -run 'Selection|Fingerprint|DirectReason|Prepare' -count=1
```

### Task 3: Persist the receipt atomically through both execution paths

**Files:**
- Modify: `internal/application/issueopspreparation/ports.go` or the current equivalent
- Modify: `internal/adapter/outbound/issueopspreparation/repository.go`
- Test: `internal/adapter/outbound/issueopspreparation/repository_test.go`
- Test: `internal/adapter/outbound/issueopspreparation/repository_orca_test.go`

- [ ] Write RED tests asserting direct commit and Orca `BeginIntent` persist the exact selection receipt in the same store transaction as the first execution record.
- [ ] Add compare-before-write checks that reject a receipt whose resolved mode differs from the chosen path or whose fingerprint no longer matches its normalized fields.
- [ ] Verify failed writes leave no execution, holder index, external intent, operation record, or partial receipt.
- [ ] Verify Orca reconciliation and later lease transitions preserve the receipt byte-for-byte.
- [ ] Implement only the new contract threading and atomic persistence; do not duplicate domain decisions in the repository.
- [ ] Run:

```bash
go test ./internal/adapter/outbound/issueopspreparation -run 'Selection|Direct|Orca|Atomic' -count=1
```

### Task 4: Expose one CLI/MCP/status contract and gate unreasoned direct use

**Files:**
- Modify: `internal/core/issueops/execution_prepare.go`
- Modify: `internal/adapter/inbound/issueopspreparation/prepare.go`
- Modify: `cmd/harness/issueopscli/executioncmd/execution.go`
- Modify: `cmd/harness/mcpcli/` IssueOps execution tool schema/mapping
- Modify: `cmd/harness/harnessapp/` response mappings and goldens
- Modify: relevant lifecycle/command-policy gate only if the shared preparation boundary cannot reject the exact automation bypass
- Test: CLI/MCP/harnessapp/contract golden suites

- [ ] Add CLI/MCP inputs `direct_reason` and `expected_readiness_fingerprint`, and expose the full preview projection plus durable `execution.selection` through status.
- [ ] Update `next_command` so an `auto` preview emits the exact confirm command with the fingerprint; explicit direct next-command also carries its literal reason safely.
- [ ] Add RED→GREEN tests for missing/mismatched fingerprint and missing direct reason across CLI and MCP. Assert all fail before mutation with the same typed error code/fields.
- [ ] Freeze required response fields in response-contract goldens. Keep error/output parity between CLI and MCP.
- [ ] Ensure current issue automation docs/prompts no longer instruct a reasonless `--mode direct` escape hatch.
- [ ] Run:

```bash
go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli ./cmd/harness/harnessapp ./cmd/harness/contractgolden -run 'Selection|Preparation|Execution|Golden' -count=1
```

### Task 5: Mark native-host smoke as a distinct evidence lane

**Files:**
- Modify: `scripts/verify-child-host-smoke.sh`
- Modify: `internal/adapter/hostprobe/child_host_smoke_script_test.go`
- Modify: `.agent-harness/operations/child-host-smoke.md`
- Modify: `.agent-harness/TESTING.md`

- [ ] Write a failing receipt test requiring exact top-level `validation_lane: "native_host"`.
- [ ] Add the constant field to the strict JSON receipt and validator fixtures without changing activation/restore semantics.
- [ ] Document that this receipt proves Codex/Claude native adapters only and is never Orca Run/task/dispatch/claim evidence.
- [ ] Run:

```bash
scripts/verify-go-test-match.sh --run 'ChildHostSmoke' --expect 'ChildHostSmoke' -- ./internal/adapter/hostprobe
bash -n scripts/verify-child-host-smoke.sh
```

### Task 6: Update IssueOps operating docs and completion evidence

**Files:**
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/execution.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TESTING.md`
- Create: `.agent-harness/issueops/248-turing-report.md`

- [ ] Document the normative automated-child order: exact base/branch preparation → reviewed/staged child plan → `execution prepare --mode auto` preview → confirm using returned fingerprint → sealed owner claim.
- [ ] Document explicit direct as an exceptional, reasoned choice and list the durable selection fields operators must read back.
- [ ] Record AC-01..AC-08 mapping, RED/GREEN commands, exact-head tests, and separate Orca/native-host evidence in the Turing report.
- [ ] Remove or update only instructions made stale by this change; do not restore pre-v1 or predecessor paths.

### Task 7: Verify, review, publish, and prove real Orca ownership

- [ ] Run focused tests from Tasks 1–6, then:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
git diff --check
```

- [ ] Run legacy and architecture fitness gates required by `.agent-harness/TESTING.md`; production legacy count must remain zero and no forbidden dependency edge may be added.
- [ ] Perform independent implementation review, fix all blocker/high/medium findings, rerun affected and full verification, and create one policy-compliant atomic commit.
- [ ] Push `248-orca-ready-issueops-dogfood`, create a Korean draft PR to `228-clean-break-hexagonal-architecture`, and verify exact base/head SHA plus green CI.
- [ ] From Orca readback—not native-host smoke—prove lifecycle `io-268bd6ac6e7a` generation 4 has `mode=orca`, an exact plan digest, linked issue/parent lineage, runtime/worktree/terminal/Run/task/dispatch, one native Codex owner claim, completion, and final HEAD/PR evidence.
- [ ] Release the owner only after completion; merge only after review, exact-head CI, and the separate `validation_lane=native_host` receipt pass. Clean only this child after parent containment and IssueOps completion are verified.

## Acceptance mapping

- AC-01: Tasks 1 and 3.
- AC-02: Task 2.
- AC-03: Tasks 2, 4, and 6.
- AC-04: Task 4.
- AC-05: Task 5.
- AC-06: Task 7 live evidence.
- AC-07: Tasks 1, 2, and 4 RED→GREEN fixtures.
- AC-08: Tasks 1, 3, 4, and full verification.

## Stop conditions

Stop before mutation and report a blocker if the issue digest, plan digest, branch link/base SHA, generation, holder/pending/completion state, Orca readiness fingerprint, installed exact binary, or canonical worktree identity drifts. Never substitute direct mode merely to make the dogfood pass. Do not treat the existing native-host smoke, PR #249 sentinel bootstrap, or an Orca terminal without a successful current-generation claim/completion as AC-06 evidence.
