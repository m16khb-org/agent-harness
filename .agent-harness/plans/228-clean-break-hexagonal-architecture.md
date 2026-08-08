# Clean-break Hexagonal Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `turing` for evidence-bound execution. Use `superpowers:subagent-driven-development` only for provider-native child cycles recorded as `isolated-worktree-work` or `task-fan-out-coordination`; otherwise the main agent executes directly. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every production legacy path and every `internal/core` package while preserving current schema-v1, CLI, MCP, hook, persistence, and process-safety behavior.

**Architecture:** Establish stable capability contracts and minimal ports first, then migrate independently reviewable child scopes into domain/application/contract/port and inbound/outbound adapters. Concrete wiring remains only in `cmd/harness/harnessapp`; each child exact HEAD must pass deterministic tests plus fresh Codex and Claude Code native smoke before parent integration.

**Tech Stack:** Go 1.26.3, standard library, `modernc.org/sqlite`, `github.com/modelcontextprotocol/go-sdk/mcp` v1.6.1, shell coordinator scripts, GitHub IssueOps, Codex CLI 0.146.0, Claude Code 2.1.220.

## Global Constraints

- Preserve only current schema v1 and current CLI/MCP/hook behavior. Do not add dual-read, dual-write, migration, feature flags, compatibility aliases, or fallback transports.
- Existing non-v1, malformed, legacy-field, key-mismatch, and byte-mismatch records return the same public `invalid state`; absent records retain `not found`.
- End state is `internal/core` package 78→0 and directory/import count 0, legacy dependency edges 100→0, production unused 147→0, and test unused 12→0.
- Do not introduce `core`, `common`, `service`, `utils`, a service locator, a mega filesystem/process port, or a second workflow authority.
- Domain, application, contract, and port packages cannot import concrete adapters, `cmd`, SQL drivers, `os/exec`, or host SDK implementations.
- Every behavior change uses a named RED test before production edits, then minimal GREEN, surface projection, cleanup, focused verification, and an atomic Conventional Commit + Lore body.
- Every child owner edits only its canonical IssueOps worktree. The source checkout remains observation/coordinator-only.
- User-scope installation and native-host activation run only in the source coordinator lane, one child at a time, with before receipt, `defer`/trap restoration, and exact readback.
- A missing hook event, timeout, truncated output, version drift, restore mismatch, zero MCP call, or multiple unexpected MCP calls is `fail`, never `inconclusive` success.
- OpenWiki is not generated or edited. Historical ADR facts remain; the new ADR entry appends a superseding decision.
- No production implementation starts until the linked plan, compatibility review, and fresh Brooks devil's-advocate verdict are all approved.

---

## TL;DR

> **Summary:** Execute one contract-first parent foundation followed by 10 provider-native child deliveries. Each child uses TDD, exact-head CI, fresh Codex/Claude smoke, parent merge, completion reflection, and child-only cleanup.
>
> **Deliverables:** current-only state/error contract; native activation separated from reset legacy; SDK-only MCP; current daemon/install/IssueOps paths; 78-package ownership migration; zero-baseline fitness/unused gates; updated shared docs and release note; 10 dual-host smoke receipts.
>
> **Effort:** XL.
>
> **Parallel:** YES — six controlled waves; live host activation is always serialized.
>
> **Critical Path:** T1 → T3/T4/T5 → T6/T7/T8 → T9/T10 → T11 → T12.

## Context

### Original Request

Verify whether the hexagonal migration and legacy deletion were actually complete, remove all remaining legacy code, create issues for the findings, and dogfood the full issue/worktree/test/PR/merge/cleanup lifecycle.

### Approved Decisions

- Clean break: preserve current v1 only.
- Strict core zero: delete every production package under `internal/core/**`.
- Generic invalid state: all invalid existing records share one public error; absent remains not-found.
- Per-child live verification: all 10 child exact HEADs must pass fresh Codex and Claude Code native smoke.
- Written design: `docs/superpowers/specs/2026-08-02-clean-break-hexagonal-architecture-design.md` at commit `4bf68a7d2cbb90c68afea53ac2d411c568b5a8d5`.

### Repo Grounding

- `go list ./internal/core/...` reports 78 production packages; production Go inventory is 308 files.
- `internal/architecture/dependency_test.go` owns the direct production import inventory, unconditional layer rules, and `internal/architecture/testdata/legacy_imports.txt` comparison.
- `internal/core/state/state_io.go`, `internal/core/state/state_migrate.go`, `internal/core/issueops/issueops_state.go`, and `internal/contract/issueopslease/record.go` accept/promote schema 0 or expose version-specific unsupported errors.
- `cmd/harness/installcli/install_native.go` currently calls `BeginLegacyResetActivation` and `SealLegacyResetActivation`; current native activation must be extracted before `internal/core/issueops/reset_legacy*.go` is deleted.
- `scripts/install-native.sh` performs same-directory staged binary replacement; the script remains a repository helper but must call the canonical `agent-harness install` command after the CLI alias is removed.
- `internal/adapter/hostprobe/codex.go`, `internal/adapter/hostprobe/claude.go`, `internal/adapter/mcp/conformance_probe.go`, and `cmd/harness/validationcli/installdryrun` already provide isolated MCP and install-dry-run building blocks.
- `internal/adapter/codex/install.go` and `internal/adapter/claude/install.go` write the current hooks/MCP surfaces; activation readback must preserve co-resident hook order and third-party entries.
- Issue #228 and children #229–#238 are open, labelled `enhancement`, assigned to `m16khb`, and linked to branch `228-clean-break-hexagonal-architecture` at sealed base `4b86dd46a454d241cdb348194754b1e1e452bc00`.

### Gap Analysis

- **Activation coupling:** deleting reset legacy as one directory sweep would remove a current safety mechanism used by install. Extract activation into the install capability first; only reset/migration semantics are legacy.
- **Cross-child dependency:** tooling packages import IssueOps/lifecycle/state, while IssueOps imports policy/preflight/state. A mechanical package-by-package move would create temporary application→core edges or aliases. T1 establishes stable contracts and ports before child migration.
- **Child parallelism:** implementation branches may start in parallel after T1, but parent integration follows the dependency matrix. No child is allowed to merge on a speculative target API.
- **Live-host gap:** existing conformance proves MCP behavior but does not alone prove installed SessionStart/PreToolUse readback and reversible temporary activation. T2 adds a coordinator-only runner that composes existing installers and host probes.
- **Public diagnostics:** collapsing invalid records reduces public detail intentionally; fixture names and internal test tables preserve the cause category without exposing it in CLI/MCP output.
- **Rollback:** code rollback is a PR revert. State bytes are never rewritten downward. Host-smoke rollback restores the exact before receipt and blocks all later work if restoration is not byte/semantic exact.

## Work Objectives

### Core Objective

Replace the retained core/facade/compatibility architecture with explicit capability ownership and zero production legacy while preserving verified current-v1 behavior.

### Definition of Done

- `test ! -d internal/core` succeeds.
- `go list ./...` contains no package/import beginning `agent-harness/internal/core`.
- `internal/architecture/testdata/legacy_imports.txt` is absent and the architecture suite reports zero legacy edges without a baseline.
- `golangci-lint run --enable-only unused --max-issues-per-linter 0 --max-same-issues 0 ./...` reports zero production and test findings.
- `rg` finds no production `reset-legacy`, `StateMigrate`, `UnsupportedSchemaError`, `unsupported_schema`, legacy MCP transport, integer PID decoder, deprecated install CLI alias, compatibility oracle, or legacy marker.
- The state/IssueOps table matrix passes for valid v1, missing/0, future, malformed, legacy field, key mismatch, byte mismatch, and absent.
- Focused, full, race, vet, build, contract golden, response golden, and deterministic full self-verify pass from one unchanged final HEAD.
- Exactly 10 child smoke receipts contain matching local/remote HEAD, both host versions, SessionStart and PreToolUse observations, one MCP call per host, activation digest, restore digest, exit codes, durations, and `verdict=pass`.
- ARCHITECTURE, CONVENTIONS, OPERATIONS, TESTING, ADR, and release notes describe the final current-only structure; OpenWiki remains untouched.
- Every child and parent has verified issue/branch/worktree/PR/CI/merge/completion/cleanup receipts according to IssueOps.

### Must NOT Have

- Temporary aliases or forwarding facades committed as the final state of any parent merge.
- Application/domain code that hides `os`, `os/exec`, `database/sql`, SQLite, `net/http`, filesystem, provider, or host calls behind package globals.
- A test that updates goldens merely to accept drift; golden changes require an intentional public current-contract change documented in the same commit.
- User credentials, transcript text, private reasoning, raw home paths, tokens, or unredacted hook/MCP configuration in evidence.
- Child cleanup before provider merge/readback, or parent cleanup before the parent PR is merged and the user separately approves deletion.

## Chosen Ownership Map

The table is normative. A source package with multiple targets is split by responsibility: pure invariant/decision to domain, stable DTO/error to contract, orchestration to application, external technology implementation to outbound adapter, and request projection to inbound adapter. The implementer does not choose a different layer name.

### Parent Contract-First Seams

| Shared concern | Exact target packages | Consumers unblocked |
| --- | --- | --- |
| Generic state error and v1 record envelope | `internal/contract/state`, `internal/domain/state` | #230, #236, #237 |
| State persistence and locking | `internal/port/state` | #236, #237, tooling audit/trace |
| IssueOps stable record and phase DTOs | `internal/contract/issueops`, `internal/domain/issueops` | #232, #235, #236 |
| Lifecycle hook DTOs and decisions | `internal/contract/lifecycle`, `internal/domain/lifecycle` | #235, #236 |
| Policy decisions and process execution | `internal/domain/policy`, `internal/port/policy` | #235, #236 |
| Current native activation | `internal/contract/nativeactivation`, `internal/application/nativeactivation`, `internal/port/nativeactivation` | #230, #231, T2 host smoke |

### #235 Tooling/Project Sources

| Source packages | Exact target packages |
| --- | --- |
| `audit` | `internal/application/audit`, `internal/adapter/outbound/audit` |
| `commandguard` | `internal/domain/commandguard` |
| `commandparse` | `internal/contract/commandparse` |
| `commitsuggest` | `internal/application/commitsuggest`, `internal/adapter/outbound/commitsuggest` |
| `contextregion` | `internal/domain/contextregion` |
| `docs`, `doctor`, `inspect`, `preflight` | `internal/application/{docs,doctor,inspect,preflight}`, `internal/adapter/outbound/{docs,doctor,preflight}` |
| `draftwiki`, `draftwiki/draftmeta`, `draftwiki/queue`, `draftwiki/suggestdraft` | `internal/application/draftwiki`, `internal/contract/draftwiki`, `internal/adapter/outbound/draftwiki` |
| `failurecause` | `internal/domain/failurecause` |
| `guard`, `guard/pattern` | `internal/application/guard`, `internal/domain/guard`, `internal/adapter/outbound/guard` |
| `hookprompt` | `internal/application/hookprompt`, `internal/contract/hookprompt` |
| `install` | `internal/application/install` |
| `judgement`, `prompt`, `skillcontract` | `internal/contract/{judgement,prompt,skillcontract}` |
| `lintdiagnose`, `lintgate` | `internal/application/{lintdiagnose,lintgate}`, `internal/adapter/outbound/lint` |
| `nextaction`, `operationalhealth`, `qualitycatalog`, `searchrouting` | `internal/domain/{nextaction,operationalhealth,qualitycatalog,searchrouting}` |
| `policy`, `policy/auditid` | `internal/domain/policy`, `internal/application/policy`, `internal/adapter/outbound/policy` |
| `projectbootstrap` | `internal/application/projectbootstrap` |
| `projectdoc`, `projectdocs`, `projectdocs/detection` | `internal/contract/projectdocs`, `internal/application/projectdocs`, `internal/adapter/outbound/projectdocs` |
| `remoteartifact`, `repopath` | `internal/domain/{remoteartifact,repopath}`, `internal/adapter/outbound/repopath` |
| `toolconformance` | `internal/domain/toolconformance`, `internal/application/toolconformance`, `internal/adapter/outbound/toolconformance` |
| `trace`, `trace/classification` | `internal/application/trace`, `internal/domain/trace`, `internal/adapter/outbound/trace` |

### #236 IssueOps/Lifecycle/Worker Sources

| Source packages | Exact target packages |
| --- | --- |
| `hookfailure`, `hookmetrics` | `internal/application/hooktelemetry`, `internal/adapter/outbound/hooktelemetry` |
| `issueops/model`, `issueops/stringlist`, `issueops/compatibilityreview`, `issueops/delegation` | `internal/contract/issueops`, `internal/domain/issueops` |
| `issueops` execution/lease/preparation/publication/completion files | existing `internal/{application,domain,contract}/issueops{lease,preparation,publication,completion}` plus `internal/adapter/{inbound,outbound}/issueops*` |
| `issueops/active`, `issueops/start`, `issueops/intentdesign`, `issueops/linking`, `issueops/branchprepare` | `internal/application/issueopscoordination`, `internal/adapter/outbound/issueopsstate` |
| `issueops/artifacttemplate`, `issueops/artifactverify`, `issueops/remote` | `internal/contract/issueopsartifact`, `internal/application/issueopsartifact`, `internal/adapter/outbound/issueopsprovider` |
| `issueops/benchmark` | `internal/domain/issueopsbenchmark`, `internal/application/issueopsbenchmark`, `internal/adapter/outbound/issueopsbenchmark` |
| `issueops/cleanupchildren`, `issueops/cleanupstatus`, `issueops/orphancleanup` | `internal/application/issueopscleanup`, `internal/adapter/outbound/issueopscleanup` |
| `issueops/devilsadvocate`, `issueops/implementation` | `internal/domain/issueopsreview`, `internal/application/issueopsreview` |
| `issueops/pathutil`, `issueops/readinesspaths` | `internal/adapter/outbound/issueopspath` |
| `lifecycle/model`, `lifecycle/fingerprint`, `lifecycle/worktreeguard`, `lifecycle/doctarget` | `internal/contract/lifecycle`, `internal/domain/lifecycle` |
| `lifecycle`, `lifecycle/compact`, `lifecycle/docupkeep`, `lifecycle/liveapproval`, `lifecycle/nextactionrelay`, `lifecycle/worktreepath` | `internal/application/lifecycle`, `internal/adapter/outbound/lifecycle` |
| `looprun` | `internal/contract/looprun`, `internal/application/looprun`, `internal/adapter/outbound/looprun` |
| `worker` | `internal/contract/worker`, `internal/application/worker`, `internal/adapter/outbound/worker` |

### #237 State/SQL/Network Sources

| Source package | Exact target packages |
| --- | --- |
| `sqlstore` | `internal/adapter/outbound/sqlstore` |
| `state` | `internal/contract/state`, `internal/domain/state`, `internal/application/state`, `internal/adapter/outbound/state` |
| `state/statepath` | `internal/domain/statepath`, `internal/adapter/outbound/statepath` |
| `webfetch` | `internal/contract/webfetch`, `internal/domain/webfetch`, `internal/application/webfetch`, `internal/adapter/outbound/webfetch` |

### #229 Root Source

`internal/core/*.go` is deleted after every external caller imports the owning capability package directly. No root replacement package is created.

## Public Error Contract

```go
package state

import "errors"

var ErrInvalidState = errors.New("invalid state")
```

- The sentinel lives once in `internal/contract/state` and contains no schema/parser detail.
- State and IssueOps decoders return or wrap the sentinel without a version, legacy-field name, raw parser error, or malformed bytes.
- Repository `Get` absence continues to return the existing not-found identity used by each surface.
- CLI/MCP projections compare with `errors.Is`; they emit the same current error code/message for every invalid-existing case.
- Tests may label fixtures `missing_schema`, `future_schema`, `malformed_json`, `legacy_field`, `key_mismatch`, and `byte_mismatch`, but expected public output is identical.

## Child Host-Smoke Evidence Contract

The coordinator-only command is:

```bash
/absolute/canonical/coordinator/worktree/scripts/verify-child-host-smoke.sh \
  --issue 230 \
  --source-root /Users/m16khb/Workspace/agent-harness \
  --child-root /absolute/canonical/child/worktree \
  --head FULL_40_HEX_SHA \
  --remote-ref refs/heads/230-short-slug \
  --json-out /absolute/private/evidence/230-host-smoke.json \
  --confirm-user-activation
```

The JSON result is strict-decoded against:

```go
type ChildHostSmokeReceipt struct {
    SchemaVersion     int               `json:"schema_version"`
    Issue             int               `json:"issue"`
    LocalHead         string            `json:"local_head"`
    RemoteHead        string            `json:"remote_head"`
    ChildBinarySHA256 string            `json:"child_binary_sha256"`
    Before            ActivationDigest  `json:"before"`
    Activated         ActivationDigest  `json:"activated"`
    ActivatedRootSHA256   string         `json:"activated_root_sha256"`
    ActivatedBinarySHA256 string         `json:"activated_binary_sha256"`
    Codex             HostSmokeEvidence `json:"codex"`
    Claude            HostSmokeEvidence `json:"claude"`
    Restore           ActivationDigest  `json:"restore"`
    Verdict           string            `json:"verdict"`
}

type HostSmokeEvidence struct {
    Version             string `json:"version"`
    SessionStartObserved bool   `json:"session_start_observed"`
    PreToolUseObserved   bool   `json:"pre_tool_use_observed"`
    MCPCallCount         int    `json:"mcp_call_count"`
    ResponseSHA256       string `json:"response_sha256"`
    ExitCode             int    `json:"exit_code"`
    DurationMS           int64  `json:"duration_ms"`
}
```

`verdict=pass` requires exact local/remote HEAD equality, `activated_binary_sha256 == child_binary_sha256`, activated semantic/raw readback matching the expected child-managed surfaces, one MCP call per host, both hook observations, zero exit codes, and `before == restore` for semantic and raw managed surfaces. `activated_root_sha256` identifies the activated root without emitting its absolute path. The strict validator rejects a missing/zero activated digest or identity. The output excludes command transcripts, credentials, prompt text, private reasoning, and user-home paths.

## Verification Strategy

- Test decision: strict TDD with Go `testing`, subprocess helpers, table tests, race tests, and shell smoke fixtures.
- Every child stores RED and GREEN command/exit evidence under ignored `.agent-harness/evidence/228/<issue>/`.
- Architecture changes use real `go list -json ./...` inventory; source-text scans supplement but never replace compile/test evidence.
- API documentation gate: no HTTP endpoint/OpenAPI schema changes are planned. Run `./bin/agent-harness api-doc static-check --repo "$PWD" --json` at every child preflight and the final gate; define `api_doc_result="$evidence_dir/api-doc-review-result.json"` and, if candidates are detected, require `./bin/agent-harness api-doc review --repo "$PWD" --result "$api_doc_result" --json` before publication.
- Filtered Go test gate: create `scripts/verify-go-test-match.sh` in T1. Every filtered command uses `scripts/verify-go-test-match.sh --run '<go-regexp>' --expect '<required-test-regexp>' -- <packages...>`; the helper runs `go test -json -count=1`, requires at least one matching `Action=run` top-level test event, preserves the test exit code/output, and fails on zero matches. Plain `go test -run` is not an accepted completion oracle.
- Live runtime matrix:

| Environment | Repo/config evidence | Runtime evidence | Failure path | Remediation order |
| --- | --- | --- | --- | --- |
| Child worktree | branch, full HEAD, clean status | deterministic focused/full tests | code/test failure | fix child, rerun from RED-owned gate |
| Temp HOME/CODEX_HOME | install JSON and no-write scan | two-host dry-run | planned write/host mismatch | fix installer, discard temp roots |
| User Codex | before/activated/restore receipts | fresh SessionStart, PreToolUse, MCP | event/call/readback mismatch | restore first, diagnose second |
| User Claude Code | before/activated/restore receipts | fresh stream-json hooks, MCP | scope/event/call mismatch | restore first, diagnose second |

## Execution Strategy

### Parallel Execution Waves

- **Wave 0 — parent foundation:** T1 shared contracts/ports, then T2 coordinator host-smoke runner. Both are reviewed and merged to the parent branch before child branches are cut or rebased.
- **Wave 1 — legacy surfaces:** T3 #230, T4 #231, and T5 #232 may execute in isolated child worktrees from the same Wave-0 parent HEAD.
- **Wave 2 — package ownership:** T6 #237 depends on T3; T7 #236 depends on T3 and T5; T8 #235 depends on T1. Parent merge order is T6 → T7 → T8 so tooling callers consume final state/lifecycle/IssueOps contracts without aliases.
- **Wave 3 — dependency inversion:** T9 #233 and T10 #234 start after T4, T6, T7, and T8 are in the parent branch.
- **Wave 4 — facade/unused:** T11 #229 starts after T3–T10 are integrated.
- **Wave 5 — final contract/docs:** T12 #238 starts after T11 and closes zero-baseline, docs, release, and full verification.
- Live host smoke and parent merge are serialized even when code implementation is parallel.

### Dependency Matrix

| Task | Issue | Depends On | Blocks | Parallel peer |
| --- | --- | --- | --- | --- |
| T1 | #228 | design approval | T3–T12 | T2 after interface names fixed |
| T2 | #228 | T1 native-activation contract | every child merge | none during user activation |
| T3 | #230 | T1, T2 | T6, T7, T11 | T4, T5 |
| T4 | #231 | T1, T2 | T9, T11 | T3, T5 |
| T5 | #232 | T1, T2 | T7, T11 | T3, T4 |
| T6 | #237 | T3 | T9, T10, T11 | T7, T8 implementation |
| T7 | #236 | T3, T5 | T9, T10, T11 | T6, T8 implementation |
| T8 | #235 | T1 | T9, T10, T11 | T6, T7 implementation |
| T9 | #233 | T4, T6, T7, T8 | T11 | T10 |
| T10 | #234 | T6, T7, T8 | T11 | T9 |
| T11 | #229 | T3–T10 | T12 | none |
| T12 | #238 | T11 | parent PR | none |

### Normative Child Delivery Epilogue

Tasks 3–12 use this one publication state machine. Task-specific endings add smoke assertions but cannot reorder these gates:

For each child set `evidence_dir="$child_root/.agent-harness/evidence/228/$issue"`, create it with mode `0700`, and keep it ignored/untracked. Evidence commands below write only bounded JSON/digests there.

1. Before committing, verify the canonical child worktree contains only task-owned tracked changes and ignored evidence, then run the child-focused, full, race, and golden gates named by the task and build `./bin/agent-harness`. Unrelated tracked/untracked files are a stop condition; the task-owned implementation diff is expected to be uncommitted here.
2. Stage only the exact task paths, inspect the staged diff, run `./bin/agent-harness api-doc static-check --repo "$PWD" --json`, and obtain a fresh implementation review with zero unresolved Critical/Important findings. If the API gate reports candidates, complete the agent review/result ingestion before continuing. Brooks is repeated only if the approved plan/design contract changes; it is not the child code reviewer. Any fix returns to step 1.
3. Create the task's named atomic commit(s) from the reviewed staging groups, verify a clean worktree, and capture the full 40-hex `child_head`. For multi-commit tasks, the listed commit sequence is implemented here; do not create a second copy of those commits.
4. Push the child branch, read it back, and require `local_head == remote_head == child_head`. Do not rewrite history after this equality is sealed.
5. Run provider CI/check readback for that exact remote head and require green before user-scope activation.
6. From the source coordinator lane, run Task 2 against the exact remote ref. A pass receipt requires `receipt.local_head == receipt.remote_head == child_head`; the smoke runner never targets an unpushed commit.
7. Create/read back the draft child PR with that head and the planned parent branch as base. Merge into the parent only after the receipt passes, review threads/checks are clear, and the authorized IssueOps child-merge gate is satisfied.
8. Record `child_head` and the distinct `parent_merge_commit` (or fast-forward parent head) in parent integration evidence. Prove the parent contains `child_head`; never require a child smoke receipt to equal the later parent merge commit.
9. Complete and clean the child only after provider merge/readback. A failed restore, stale remote head, CI failure, or integration mismatch blocks merge, cleanup, and the next child's live smoke.

## Implementation Tasks

### Task 1: Establish contract-first capability seams on parent #228

**Files:**

- Create: `internal/contract/state/errors.go`, `internal/contract/state/record.go`, `internal/contract/state/errors_test.go`
- Create: `internal/domain/state/validation.go`, `internal/domain/state/validation_test.go`
- Create: `internal/contract/issueops/types.go`, `internal/contract/lifecycle/types.go`
- Create: `internal/domain/issueops/phase.go`, `internal/domain/lifecycle/decision.go`
- Create: `internal/domain/policy/decision.go`, `internal/port/policy/policy.go`, `internal/port/state/state.go`
- Create: `internal/contract/nativeactivation/receipt.go`, `internal/application/nativeactivation/service.go`, `internal/port/nativeactivation/nativeactivation.go`
- Create: `internal/architecture/ownership_manifest_test.go`
- Create: `scripts/verify-go-test-match.sh`, `scripts/verify-go-test-match-test.sh`
- Modify: `internal/core/issueops/model/{execution.go,intent_class.go,phase.go,types.go}` consumers to import stable contracts directly
- Modify: `internal/core/lifecycle/model/*.go` consumers to import stable contracts directly
- Modify: `internal/core/policy/*.go`, `internal/core/state/state_types.go`, `cmd/harness/installcli/install_native.go`
- Test: focused tests beside every created package plus `internal/architecture/dependency_test.go`

**Interfaces:**

- Produces: `state.ErrInvalidState`, exact-v1 `state.RecordEnvelope`, stable `issueops.Record`/phase DTOs, stable lifecycle DTOs, policy decision interfaces, state store interfaces, and current-only native activation service.
- Consumes: existing schema-v1 record fields and current `port.TransactionalRecordStore`; no legacy decoder or adapter type enters the contract.

- [ ] **Step 1: Add the zero-match guard, then RED contract tests for the public state identity**

First write shell fixtures proving the match helper fails when `go test -json` contains no top-level `Action=run` event, propagates a test failure, and passes only when an event name matches `--expect`. Implement the bounded helper, run `bash scripts/verify-go-test-match-test.sh`, and use it for every later filtered test.

```go
func TestInvalidExistingStateUsesOnePublicIdentity(t *testing.T) {
    cases := []error{
        state.Invalid("missing_schema"),
        state.Invalid("future_schema"),
        state.Invalid("malformed_json"),
        state.Invalid("legacy_field"),
        state.Invalid("key_mismatch"),
        state.Invalid("byte_mismatch"),
    }
    for _, err := range cases {
        if !errors.Is(err, state.ErrInvalidState) || err.Error() != "invalid state" {
            t.Fatalf("public invalid-state drift: %q", err)
        }
    }
}
```

Run: `scripts/verify-go-test-match.sh --run '^TestInvalidExistingStateUsesOnePublicIdentity$' --expect '^TestInvalidExistingStateUsesOnePublicIdentity$' -- ./internal/contract/state`

Expected RED: package or symbols do not exist.

- [ ] **Step 2: Add RED architecture ownership fixtures**

Add synthetic import edges that reject `domain|application|contract|port -> internal/core|internal/adapter|cmd`, reject any package/import prefix `internal/core`, and allow only the target directions in the approved design.

Run: `scripts/verify-go-test-match.sh --run 'Ownership|EvaluateEdges' --expect 'Ownership|EvaluateEdges' -- ./internal/architecture`

Expected RED: the new strict ownership rules or manifest symbols are absent.

- [ ] **Step 3: Implement the minimal stable contracts**

```go
package state

import "errors"

var ErrInvalidState = errors.New("invalid state")

func Invalid(_ string) error { return ErrInvalidState }
```

The string parameter exists only so decoder tests can retain internal fixture labels; it is not stored or rendered. Define schema-v1 record envelopes without legacy/future unions. Move stable IssueOps/lifecycle DTO definitions, not filesystem/process code, into their exact contract packages and update callers directly rather than adding aliases.

- [ ] **Step 4: Define capability-minimal ports**

```go
type Reader interface {
    Get(bucket, id string) ([]byte, bool, error)
}

type TransactionalStore interface {
    Get(bucket, id string) ([]byte, bool, error)
    Mutate([]Mutation) error
}
```

Keep policy and native-activation ports separate. Do not create a generic repository containing state, process, filesystem, provider, and network methods.

- [ ] **Step 5: Extract current native activation from legacy reset naming**

Move the begin/seal/readback receipt contract used by `cmd/harness/installcli/install_native.go` into `internal/application/nativeactivation` plus its outbound persistence/readback port. The receipt retains target binary identity, four Codex/Claude hook/MCP surfaces, semantic/raw SHA-256, and write-last sealing; it contains no reset target schema or legacy-drain field.

- [ ] **Step 6: Update direct consumers and remove temporary definitions**

Update compilation units that consume moved DTOs/interfaces. Delete the old definition immediately after each consumer imports the exact target package; no production type alias or forwarding function is permitted.

- [ ] **Step 7: Verify the foundation**

Run:

```bash
go test ./internal/contract/state ./internal/domain/state ./internal/contract/issueops ./internal/domain/issueops ./internal/contract/lifecycle ./internal/domain/lifecycle -count=1
go test ./internal/application/nativeactivation ./internal/architecture -count=1
go test -race ./internal/application/nativeactivation ./internal/domain/... -count=1
go test ./cmd/harness/installcli -count=1
```

Expected GREEN: all pass; `rg -n 'type .* = .*internal/core|func .*\{ return core\.' internal/{contract,domain,application,port}` returns no compatibility facade.

**QA Scenarios:**

```text
Scenario: valid current contracts compile through real consumers
  Channel: bash
  Steps: run the four focused commands above from the exact parent worktree HEAD
  Expected: exit 0; no contract/application import of adapter/cmd/core
  Evidence: .agent-harness/evidence/228/task-1-contract-foundation.json

Scenario: forbidden compatibility seam
  Channel: bash
  Steps: run ownership synthetic test with application -> internal/core and contract -> adapter fixtures
  Expected: exact rule plus importer -> imported diagnostic; test exits 0 because rejection is asserted
  Evidence: .agent-harness/evidence/228/task-1-contract-foundation-error.json
```

**Commit:** `refactor(architecture): establish capability contracts`

Stage only the created contract/domain/port/nativeactivation files and the exact consumers/tests changed by this task. Lore must state that these are final ownership seams, not compatibility aliases.

### Task 2: Build the serialized child dual-host smoke runner on parent #228

**Files:**

- Create: `scripts/verify-child-host-smoke.sh`
- Create: `internal/adapter/hostprobe/child_host_smoke_script_test.go`
- Create: `internal/adapter/hostprobe/testdata/child-host-smoke/{codex-stream.jsonl,claude-stream.jsonl,invalid-stream.jsonl}`
- Create: `.agent-harness/operations/child-host-smoke.md`
- Modify: `internal/adapter/hostprobe/{codex.go,claude.go,runner.go}` only to expose bounded event observations already present in native output
- Modify: `internal/port/tool_conformance.go` to add SessionStart/PreToolUse booleans and response digest without transcript fields
- Modify: `.agent-harness/TESTING.md` live-smoke section only if the new runner contract differs from the existing approved text

**Interfaces:**

- Consumes: Task 1 native-activation service, existing `scripts/install-native.sh`, `contract conformance live`, Codex/Claude installers, hostprobe runners, and exact Git refs.
- Produces: strict `ChildHostSmokeReceipt` JSON and a nonzero exit on every fail/inconclusive condition.

- [ ] **Step 1: Write RED parser and restoration tests**

Tests must inject fake `git`, `codex`, `claude`, install, and receipt readers. Cover success, local/remote SHA mismatch, Codex version drift, Claude version drift, missing SessionStart, missing PreToolUse, MCP call count 0 and 2, missing/zero activated digest, activated binary mismatch, activation failure, child-session failure, restore failure, and signal/early-exit cleanup. Every drift case returns nonzero with `verdict=fail`, still restores once, and cannot invoke a later merge/cleanup command.

```go
func TestChildHostSmokeAlwaysRestoresBeforeReturningFailure(t *testing.T) {
    result := runScriptFixture(t, fixture{ClaudeExit: 9})
    if result.ExitCode == 0 || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 {
        t.Fatalf("unsafe failure receipt: %+v", result)
    }
}
```

Run: `scripts/verify-go-test-match.sh --run 'ChildHostSmoke' --expect 'ChildHostSmoke' -- ./internal/adapter/hostprobe`

Expected RED: runner script/fixtures do not exist.

- [ ] **Step 2: Implement strict argument and authority checks**

The script requires absolute canonical source/child/output paths, decimal issue number, 40-hex HEAD, `refs/heads/` remote ref, clean child worktree, local HEAD equality, singleton `git ls-remote` equality, and literal `--confirm-user-activation`. Reject symlink evidence/output paths and mode other than private `0600` file in an existing private directory.

- [ ] **Step 3: Seal before-state and single-flight ownership**

```bash
lock_dir="${HARNESS_STATE_DIR:-$HOME/.local/state/agent-harness}/child-host-smoke.lock"
if ! mkdir "$lock_dir" 2>/dev/null; then
  echo 'child_host_smoke_already_running' >&2
  exit 1
fi
restore_required=0
cleanup() {
  rc=$?
  if [[ "$restore_required" == 1 ]]; then
    restore_previous_activation || rc=1
  fi
  rmdir "$lock_dir" 2>/dev/null || rc=1
  exit "$rc"
}
trap cleanup EXIT INT TERM
```

Before-state includes the active root, binary SHA-256, host versions, and semantic/raw digests for Codex hooks/MCP and Claude hooks/MCP. It never copies credential stores or unrelated home files.

- [ ] **Step 4: Build and verify the exact child binary**

Build into the disposable child checkout, verify `version`, record SHA-256, then run canonical `install --dry-run --project-local --json` under temporary HOME/CODEX_HOME/HARNESS_ROOT and assert exact two-host/no-write output.

- [ ] **Step 5: Activate, run fresh hosts, and restore**

Activate via the child checkout's staged install helper with `--skip-build --path-mode=skip --json`. Start fresh nonpersistent Codex and Claude sessions using the verified argv from `internal/adapter/hostprobe`; observe SessionStart and PreToolUse, make exactly one capture-only MCP tool call, and perform host-native MCP registration readback. Set `restore_required=1` before the first user-scope write and clear it only after exact restore readback succeeds.

- [ ] **Step 6: Emit bounded receipt last**

Write a temporary `0600` file, `fsync`, atomic rename, and parent-directory `fsync`. Hash only bounded/redacted event projections. `verdict=pass` is impossible until restore verification finishes.

- [ ] **Step 7: Verify deterministic and live opt-in boundaries**

Run:

```bash
scripts/verify-go-test-match.sh --run 'ChildHostSmoke' --expect 'ChildHostSmoke' -- ./internal/adapter/hostprobe
go test ./internal/adapter/hostprobe ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/mcp -count=1
go test -race ./internal/adapter/hostprobe -count=1
bash -n scripts/verify-child-host-smoke.sh
```

Expected GREEN: fixture tests pass without touching user config; the real script refuses to activate without the literal confirmation flag.

**QA Scenarios:**

```text
Scenario: complete fake two-host pass
  Channel: bash
  Steps: invoke the script test harness with matching local/remote SHA, two hook events per host, one MCP call per host, and matching before/restore digests
  Expected: exit 0, verdict=pass, private atomic JSON receipt, restore_calls=1
  Evidence: .agent-harness/evidence/228/task-2-host-smoke.json

Scenario: Claude succeeds but restore digest drifts
  Channel: bash
  Steps: inject one-byte raw Codex hook drift after restore
  Expected: nonzero exit, verdict=fail, lock released, no next-child/merge command executed
  Evidence: .agent-harness/evidence/228/task-2-host-smoke-error.json
```

**Commit:** `test(host): add reversible child smoke runner`

Lore must state the user-scope mutation boundary, exact restoration rule, host versions, and why the runner is coordinator-only.

### Task 3: Complete #230 current-only state and delete reset/migration legacy

**Files:**

- Delete: `internal/core/issueops/reset_legacy.go`, `reset_legacy_activation.go`, `reset_legacy_confirm.go`, `reset_legacy_drain.go`, `reset_legacy_process.go` after current activation ownership is in Task 1
- Delete: `internal/core/issueops/reset_legacy*_test.go` except negative current-only fixtures moved below
- Delete: `internal/core/state/state_migrate.go`, `internal/core/state/state_migrate_test.go`
- Delete: `cmd/harness/issueopscli/issueops_reset_legacy_cli.go`, `issueops_reset_legacy_cli_test.go`
- Modify: `internal/core/state/{state_io.go,state_types.go,state_doctor*.go,state_test.go}`
- Modify: `internal/core/issueops/issueops_state.go` and current record decoder tests
- Modify: `internal/contract/issueopslease/{record.go,record_test.go}`
- Modify: `cmd/harness/statecli/{state_cli_router.go,state_cli_maintenance.go,state_cli_test.go,exports.go}`
- Modify: `cmd/harness/mcpcli/{mcp_tool_policy_state.go,mcp_tool_policy_state_test.go}`
- Modify: `cmd/harness/validationcli/stateroundtrip/*`, `cmd/harness/selfworkflow/rerun/*`, `internal/adapter/cli/usage.go`, contract/response goldens
- Create: `internal/contract/state/invalid_matrix_test.go`, `internal/contract/issueopslease/invalid_matrix_test.go`

**Interfaces:**

- Consumes: Task 1 `state.ErrInvalidState` and native activation service.
- Produces: exact-v1-only decoders, no migration/reset CLI/MCP surface, and identical invalid-existing projection.

- [ ] **Step 1: Write the RED state/IssueOps matrix before deleting code**

```go
var invalidCases = []struct {
    name string
    raw  string
}{
    {"missing_schema", `{"value":1}`},
    {"schema_zero", `{"schema_version":0}`},
    {"future_schema", `{"schema_version":2}`},
    {"malformed_json", `{`},
    {"legacy_field", `{"schema_version":1,"legacy_authority":"x"}`},
    {"key_mismatch", validV1WithWrongKey},
    {"byte_mismatch", validV1WithWrongDigest},
}
```

Assert `errors.Is(err, state.ErrInvalidState)` and exact public text `invalid state` for every row; assert absent storage returns the existing not-found identity.

Run: `scripts/verify-go-test-match.sh --run 'InvalidMatrix|Absent' --expect 'InvalidMatrix|Absent' -- ./internal/contract/state ./internal/contract/issueopslease`

Expected RED: schema zero currently migrates/promotes and future/malformed cases leak different details.

- [ ] **Step 2: Make decoders exact-v1 and strict**

Decode one JSON object, reject unknown legacy authority fields, require schema version exactly 1, validate key and canonical persisted bytes, and map every existing-invalid result to the sentinel. Do not expose `json.SyntaxError`, schema integer, or field name outside internal fixture assertions.

- [ ] **Step 3: Remove schema promotion and unsupported error types**

Delete `UnsupportedSchemaError`, `unsupported_schema`, zero→v1 normalization, and version-specific error mapping. Update CLI/MCP/application projections to use `errors.Is(ErrInvalidState)`.

- [ ] **Step 4: Remove reset and migrate surfaces**

Delete production reset/migrate handlers, CLI router entries, MCP tools, usage lines, self-verify rerun suggestions, and validation paths. Keep state doctor current-v1 inspection; invalid records are reported as invalid rather than migrated.

- [ ] **Step 5: Preserve current native activation**

Update canonical `install` to call Task 1 current activation service. Verify no remaining activation symbol, error, or receipt contains `legacy`, `reset`, or `target_schema`.

- [ ] **Step 6: Run exact absence and focused gates**

```bash
! rg -n 'reset-legacy|PreviewLegacyReset|ConfirmLegacyReset|StateMigrate|UnsupportedSchemaError|unsupported_schema|unsupported (state|issueops) schema' cmd internal --glob '*.go'
go test ./internal/contract/state ./internal/contract/issueopslease ./cmd/harness/statecli ./cmd/harness/issueopscli ./cmd/harness/mcpcli -count=1
go test -race ./internal/application/nativeactivation ./internal/contract/issueopslease -count=1
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden ./cmd/harness/harnessapp
```

- [ ] **Step 7: Commit, push, and execute exact-head child smoke**

After focused/full verification, execute the Normative Child Delivery Epilogue for issue 230. Its implementation review must confirm the invalid-state projection and preserved activation boundary; parent merge is blocked until the private receipt strict-decodes with `verdict=pass`.

**QA Scenarios:**

```text
Scenario: current-v1 read/write
  Channel: bash
  Steps: run focused state and IssueOps round-trip tests with schema_version=1
  Expected: persisted bytes and current response contract remain byte-equivalent
  Evidence: .agent-harness/evidence/228/230/current-v1.json

Scenario: existing invalid versus absent
  Channel: bash
  Steps: execute all seven invalid fixtures plus an absent key through application, CLI, and MCP projections
  Expected: first seven return identical invalid state; absent alone returns not found
  Evidence: .agent-harness/evidence/228/230/invalid-matrix.json
```

**Commit:** `refactor(state): remove pre-v1 compatibility`

Lore must name the breaking clean break, generic invalid-state contract, preserved not-found/current activation, commands, and no-migration rollback boundary.

### Task 4: Complete #231 SDK-only MCP, structured daemon identity, and canonical install CLI

**Files:**

- Modify/Delete: `cmd/harness/mcpcli/{mcp_transport.go,mcp_sdk_server.go,mcp_tools.go,mcp_tool_*.go,mcp_transport_test.go}`
- Modify: `cmd/harness/harnessapp/{mcp_facade.go,root_command_facade.go,facade_wrappers_test.go}`
- Modify: `cmd/harness/daemoncli/{daemon.go,daemon_status.go,daemon_identity.go,daemon_identity_test.go}`
- Modify: `cmd/harness/daemoncli/daemonpaths/{instance.go,instance_test.go}`
- Delete: `cmd/harness/installcli/install_native.go` only after canonical install implementation is renamed to `install.go`; remove `runInstallNative`
- Modify: `internal/adapter/cli/usage.go`, `scripts/install-native.sh`, `scripts/release-repro-smoke.sh`
- Modify: `cmd/harness/updatecli/{update_bootstrap.go,update_bootstrap_test.go,update_bootstrap_edges_test.go}`
- Modify: `cmd/harness/validationcli/installdryrun/*`, `cmd/harness/selfworkflow/{rerun,summary,candidateexport,augmentcatalog}/*`
- Modify: `internal/adapter/install_contract_matrix_test.go`, `cmd/harness/installcli/install_command_test.go`

**Interfaces:**

- Consumes: go-sdk `mcp.IOTransport`, current daemon `daemonInstance`, canonical `agent-harness install`, and Task 1 native activation.
- Produces: one SDK transport path for stdio and daemon `net.Conn`, structured-only daemon instance files, and no `install-native` CLI command.

- [ ] **Step 1: Add RED SDK transport parity tests**

Construct one `mcp.IOTransport` round trip for separate stdin/stdout and one for a single replaying daemon connection. Assert initialization, tools/list, one read-only tool call, cancellation, and EOF behavior; assert `HARNESS_MCP_DIRECT=1` selects the current direct path without legacy parser types.

```go
func TestSDKTransportOwnsBothStdioAndDaemonConnections(t *testing.T) {
    for _, mode := range []string{"stdio", "daemon_conn"} {
        t.Run(mode, func(t *testing.T) {
            session := startSDKSession(t, mode)
            tools := listTools(t, session)
            if !containsTool(tools, "issueops_execution") { t.Fatal("missing current tool") }
        })
    }
}
```

Run: `scripts/verify-go-test-match.sh --run 'SDKTransportOwns' --expect 'SDKTransportOwns' -- ./cmd/harness/mcpcli`

Expected RED: daemon/split stream selection still references the compatibility transport.

- [ ] **Step 2: Delete hand-rolled MCP request/error/parser surface**

Route both stream shapes through `mcp.Server.Run` and `mcp.IOTransport`. Delete `serveMCPStreamLegacy`, `RPCRequest`, `RPCError`, custom JSON-RPC dispatch/validation, and fallback selection. Keep the current MCP tool/resource schemas and tool handlers unchanged.

- [ ] **Step 3: Add RED daemon instance rejection tests**

Write a PID file containing only `12345\n`; assert status refuses it as invalid structured state. Write a current JSON `daemonInstance`; assert status/identity succeeds and stale process identity is diagnosed through existing fields.

Run: `scripts/verify-go-test-match.sh --run 'LegacyPID|StructuredInstance' --expect 'LegacyPID|StructuredInstance' -- ./cmd/harness/daemoncli/...`

Expected RED: integer PID is currently accepted as legacy.

- [ ] **Step 4: Remove daemon integer-PID compatibility**

Make `daemonpaths.ReadInstance` strict-decode the current object only. Remove the `legacy bool` return, retry branches conditioned on legacy, and legacy status prose. Do not change socket protocol or current identity fields.

- [ ] **Step 5: Add RED canonical-install routing tests**

Assert root usage contains `install` and not `install-native`; invoking `install-native` returns unknown command; `scripts/install-native.sh --dry-run --json` invokes the binary with `install --dry-run --json`; update/bootstrap retain their documented current behavior.

- [ ] **Step 6: Remove only the CLI alias**

Rename the canonical implementation file/function to `install.go`/`runInstall`. Remove the alias router and usage entry. Keep `scripts/install-native.sh` as the named build-and-install helper, but change its binary invocation and all deterministic validation fixtures to `agent-harness install`.

- [ ] **Step 7: Run focused and public-contract gates**

```bash
! rg -n 'serveMCPStreamLegacy|type RPCRequest|type RPCError|compatibility alias for install|"install-native": run' cmd internal --glob '*.go'
go test ./cmd/harness/mcpcli ./cmd/harness/daemoncli/... ./cmd/harness/installcli ./cmd/harness/updatecli -count=1
go test -race ./cmd/harness/mcpcli ./cmd/harness/daemoncli/... -count=1
scripts/verify-go-test-match.sh --run '^TestNativeInstallAdapterContractMatrix$' --expect '^TestNativeInstallAdapterContractMatrix$' -- ./internal/adapter
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden ./cmd/harness/harnessapp
```

- [ ] **Step 8: Commit, push, and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 231. The smoke must prove the canonical install command, SessionStart/PreToolUse hooks, direct MCP mode, daemon-backed MCP mode, and exact restore.

**QA Scenarios:**

```text
Scenario: current SDK MCP through both transports
  Channel: bash
  Steps: run SDK transport table and HARNESS_MCP_DIRECT=1 contract tests
  Expected: initialization and one current tool call pass with no custom JSON-RPC type in production
  Evidence: .agent-harness/evidence/228/231/mcp-sdk.json

Scenario: retired inputs
  Channel: bash
  Steps: feed integer PID file and invoke agent-harness install-native --dry-run
  Expected: both fail deterministically; structured daemon file and agent-harness install succeed
  Evidence: .agent-harness/evidence/228/231/legacy-rejection.json
```

**Commit:** `refactor(runtime): remove transport compatibility`

Lore names SDK transport unification, structured daemon identity, removed CLI alias, retained helper script, and both native-host receipts.

### Task 5: Complete #232 current IssueOps vertical and delete compatibility oracles/markers

**Files:**

- Modify: `internal/core/issueops/execution_lease.go`, `execution_complete.go`, `execution_prepare.go`, `execution_remote.go`, `execution_resume.go`, `execution_reconcile.go`
- Delete/replace tests: `execution_*_legacy_oracle_test.go`, `execution_lease_reseed_compatibility_{test,export_test}.go`, `execution_complete_legacy_oracle_test.go`
- Modify: `internal/core/issueops/execution_orca_marker.go`, `execution_orca_intent.go`, and their tests
- Modify: `internal/adapter/orca/{execution.go,execution_test.go}`
- Modify: `internal/adapter/provider/github/provider_test.go` only where a legacy self-workflow placeholder is distinguished from current `@me`
- Modify: current `internal/{application,domain,contract}/issueops{lease,preparation,publication,completion}` and inbound/outbound adapters
- Modify: `internal/architecture/dependency_test.go` compatibility-oracle detector

**Interfaces:**

- Consumes: Task 1 stable IssueOps/lifecycle contracts and existing current application verticals.
- Produces: one production entry per release, completion, preparation, publication, resume, reconcile, and current Orca marker identity.

- [ ] **Step 1: Write RED negative tests for retired forms**

Add tests asserting a two-argument release/completion facade is not registered, legacy Orca marker syntax is rejected without mutation, and retired self-workflow kind aliases do not normalize to current kinds.

```go
func TestCurrentMarkerParserRejectsLegacyMarkerWithoutMutation(t *testing.T) {
    before := snapshotState(t)
    _, err := ParseCurrentMarker("legacy:orca:task")
    if err == nil || !bytes.Equal(before, snapshotState(t)) {
        t.Fatal("legacy marker was accepted or mutated state")
    }
}
```

Run: `scripts/verify-go-test-match.sh --run 'RejectsLegacy|CurrentVerticalOnly' --expect 'RejectsLegacy|CurrentVerticalOnly' -- ./internal/core/issueops ./internal/adapter/orca`

Expected RED: legacy parser/oracle remains callable.

- [ ] **Step 2: Route production callers to current application/inbound verticals**

Update CLI/MCP/hook wiring to construct the current service once in `harnessapp` and call typed inbound handlers. Remove the public two-argument `ReleaseExecution`/`CompleteExecution` compatibility path after all callers use the service contract.

- [ ] **Step 3: Delete compatibility oracle implementations and differential tests**

Delete `releaseExecutionCompatibilityOracle`, `reseedExecutionCompatibilityOracle`, preparation/remote/reconcile/resume legacy oracles, exported test oracle shims, and architecture allow checks. Retain current vertical tests as the behavioral authority; convert useful negative fixtures to rejection tests against current handlers.

- [ ] **Step 4: Make marker parsing current-only**

Accept exactly the current lifecycle/generation/operation marker format defined by the stable contract. Delete `LegacyOrca*`, legacy intent render/parse, and legacy self-workflow aliases. Unknown/retired inputs return a bounded current error and perform no external or durable mutation.

- [ ] **Step 5: Verify generation/lease/publication behavior**

```bash
! rg -n 'CompatibilityOracle|LegacyOrca|LegacySelf|parseLegacy|renderLegacy|compatibilityExport' cmd internal --glob '*.go'
go test ./internal/application/issueopslease ./internal/application/issueopspreparation ./internal/application/issueopspublication ./internal/application/issueopscompletion -count=1
go test ./internal/adapter/inbound/... ./internal/adapter/outbound/... ./internal/adapter/orca -count=1
go test -race ./internal/application/issueopslease ./internal/adapter/orca -count=1
scripts/verify-go-test-match.sh --run 'Lease|Generation|Preparation|Publication|Completion|Resume|Reconcile' --expect 'Lease|Generation|Preparation|Publication|Completion|Resume|Reconcile' -- ./internal/core/issueops
```

- [ ] **Step 6: Commit, push, and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 232. The MCP call must exercise current `issueops_execution` preview/status only; no remote mutation occurs during smoke.

**QA Scenarios:**

```text
Scenario: current execution lifecycle
  Channel: bash
  Steps: prepare direct execution, release exact generation, reseed through current application service, and verify generation fencing in temp state
  Expected: current path passes; stale actor/generation fails without mutation
  Evidence: .agent-harness/evidence/228/232/current-lifecycle.json

Scenario: legacy oracle and marker rejection
  Channel: bash
  Steps: run retired two-argument and legacy marker fixtures against current handlers
  Expected: bounded rejection, byte-identical state, zero provider/Orca calls
  Evidence: .agent-harness/evidence/228/232/legacy-rejection.json
```

**Commit:** `refactor(issueops): remove compatibility oracles`

Lore states the exact current verticals retained and the negative fixtures replacing legacy differential approval.

### Task 6: Complete #237 state, SQLite, path, and web-fetch ownership migration

**Files:**

- Move/split: `internal/core/sqlstore/*.go` → `internal/adapter/outbound/sqlstore/*.go`
- Move/split: `internal/core/state/*.go` → `internal/{contract,domain,application}/state/*.go` and `internal/adapter/outbound/state/*.go`
- Move/split: `internal/core/state/statepath/*.go` → `internal/domain/statepath/*.go`, `internal/adapter/outbound/statepath/*.go`
- Move/split: `internal/core/webfetch/*.go` → `internal/{contract,domain,application}/webfetch/*.go`, `internal/adapter/outbound/webfetch/*.go`
- Modify: all production/test importers returned by `go list -json ./...` for these four source prefixes
- Modify: `internal/architecture/dependency_test.go` and add exact source-prefix absence fixtures

**Interfaces:**

- Consumes: Task 3 exact-v1/generic-invalid contract and Task 1 state ports.
- Produces: SQL/filesystem/HTTP outbound adapters and technology-free state/web-fetch domain/application packages.

- [ ] **Step 1: Write RED package ownership and behavior-preservation tests**

Add an architecture test that fails while any of the four source prefixes exists or is imported. Add differential tables for state current-v1 bytes/not-found/invalid, sqlstore transaction rollback/locking, statepath canonicalization, and web-fetch timeout/status/body bounds.

Run: `go test ./internal/architecture ./internal/contract/state ./internal/application/state ./internal/application/webfetch -count=1`

Expected RED: target application packages or source-prefix rejection is absent.

- [ ] **Step 2: Move SQL implementation without changing transaction semantics**

Move SQLite driver, `database/sql`, file permission, process mutex, maintenance, and atomic transaction code to `internal/adapter/outbound/sqlstore`. Keep transaction ordering and conflict behavior byte-equivalent. The adapter implements the narrow Task 1 state store ports.

- [ ] **Step 3: Split state responsibility**

Place envelopes/errors in contract, invariant/validation in domain, doctor/maintain/prune/read-write orchestration in application, and file/lock/SQL implementation in outbound state adapters. Do not recreate migration, schema promotion, or unsupported-schema error.

- [ ] **Step 4: Split statepath and web-fetch**

Pure path validation and URL/request/result rules move to domain/contract; `os`, `filepath` identity, `net/http`, DNS, process, and body I/O move to outbound adapters. Application owns timeout/order/retry decisions already present; it does not add a new retry policy.

- [ ] **Step 5: Update all importers directly and remove source directories**

Use the production import inventory to update every importer. Delete each old package as soon as its callers compile against the target; no alias bridge is committed.

- [ ] **Step 6: Verify focused/race/absence gates**

```bash
test ! -d internal/core/sqlstore
test ! -d internal/core/state
test ! -d internal/core/webfetch
! go list -json ./... | rg 'agent-harness/internal/core/(sqlstore|state|webfetch)'
go test ./internal/contract/state ./internal/domain/state ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore -count=1
go test ./internal/contract/webfetch ./internal/domain/webfetch ./internal/application/webfetch ./internal/adapter/outbound/webfetch -count=1
go test -race ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore ./internal/application/webfetch -count=1
```

- [ ] **Step 7: Commit, push, and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 237. The smoke MCP call and hooks must use the moved state adapter through current composition without exposing SQL/path errors.

**QA Scenarios:**

```text
Scenario: state/SQL current-v1 parity
  Channel: bash
  Steps: write/read v1 bytes, exercise transaction commit/rollback and same-root lock under race
  Expected: identical bytes, atomic rollback, no race, absent stays not found
  Evidence: .agent-harness/evidence/228/237/state-sql.json

Scenario: invalid state and bounded HTTP failure
  Channel: bash
  Steps: feed future schema and an oversized/timeout HTTP fixture
  Expected: state returns generic invalid state; web-fetch returns existing bounded timeout/body classification
  Evidence: .agent-harness/evidence/228/237/failure-paths.json
```

**Commit:** `refactor(storage): move state and network adapters`

Lore records unchanged SQL/HTTP semantics, exact source prefixes removed, and generic invalid-state/not-found evidence.

### Task 7: Complete #236 IssueOps, lifecycle, loop, worker, and hook-telemetry migration

**Files:**

- Move/split: `internal/core/hookfailure/*.go`, `internal/core/hookmetrics/*.go`
- Move/split: `internal/core/issueops/*.go` and all `internal/core/issueops/*/*.go` except legacy files already deleted by Tasks 3/5
- Move/split: `internal/core/lifecycle/*.go` and all `internal/core/lifecycle/*/*.go`
- Move/split: `internal/core/looprun/*.go`, `internal/core/worker/*.go`
- Target: the exact #236 packages in the Chosen Ownership Map, including existing issueops lease/preparation/publication/completion verticals
- Modify: `cmd/harness/issueopscli/**`, `cmd/harness/hookcli/**`, `cmd/harness/loopcli/**`, `cmd/harness/workercli/**`, `cmd/harness/harnessapp/issueops_*`, lifecycle hook wiring, MCP issueops tool
- Modify: `internal/adapter/orca/**`, provider adapters, IssueOps outbound state/provider/process adapters
- Modify: all production/test importers from `go list -json ./...`

**Interfaces:**

- Consumes: Tasks 1/3/5 contracts and current application vertical behavior.
- Produces: no production package under the 35 #236 source paths; generation fence, lock, atomicity, actor/cwd validation, append-only phase ledger, and current CLI/MCP/hooks remain equivalent.

- [ ] **Step 1: Freeze the current-v1 differential matrix as RED ownership tests**

Add target-package tests that run current and moved services against the same deterministic dependencies for prepare, release, replace/reseed, resume, reconcile, remote publication, completion, child start/accept, phase transition, hook routing, loop lifecycle, and worker queue.

```go
func TestMovedIssueOpsServicePreservesGenerationFence(t *testing.T) {
    current, moved := newExecutionPair(t)
    stale := requestWithGeneration(3)
    assertSameResultAndState(t, current.Release(stale), moved.Release(stale))
}
```

Run: `scripts/verify-go-test-match.sh --run 'Moved|SourcePrefix' --expect 'Moved|SourcePrefix' -- ./internal/application/issueops... ./internal/architecture`

Expected RED: target packages/source-prefix gates are incomplete.

- [ ] **Step 2: Move stable record/model and pure decisions**

Finish moving record/phase/lifecycle/worker DTOs to contracts and pure validation, readiness, next-action, delegation, review, and fingerprint decisions to domain packages. Remove the original files immediately; do not retain `model` aliases.

- [ ] **Step 3: Move IssueOps application orchestration by vertical**

Keep the four existing issueops execution verticals and create explicit coordination/artifact/benchmark/cleanup/review application packages named in the ownership map. Inject clock, record store, process inspector, Git/worktree, Orca, and provider ports. No application package imports `os`, `os/exec`, adapter/provider, adapter/orca, or `cmd`.

- [ ] **Step 4: Move persistence/provider/process implementations to outbound adapters**

Move record file/SQLite operations, lock files, path identity, process receipts, Git/provider calls, Orca inventory/actions, artifact file access, and cleanup effects to the exact outbound packages. Preserve current atomic transaction and pending-external-intent semantics.

- [ ] **Step 5: Move lifecycle, loop, worker, and telemetry**

Application packages own lifecycle orchestration, capsule/next-action coordination, loop state transitions, worker enqueue/status/list/cancel, and hook metric aggregation. Outbound adapters own files, locks, PIDs, JSONL, clocks, and process probes. Keep worker no-shell behavior; do not add network/background execution.

- [ ] **Step 6: Rewire inbound CLI/MCP/hooks and harnessapp**

CLI/MCP/hook packages consume typed application/contract interfaces. `harnessapp` constructs outbound implementations and injects them; CLI packages do not instantiate adapters. Preserve command/tool names and required response fields for current surfaces.

- [ ] **Step 7: Delete all 35 source packages and reject their return**

```bash
for pkg in hookfailure hookmetrics issueops lifecycle looprun worker; do
  test ! -d "internal/core/$pkg"
done
! go list -json ./... | rg 'agent-harness/internal/core/(hookfailure|hookmetrics|issueops|lifecycle|looprun|worker)'
```

Add the exact prefix set to architecture fitness with importer→imported diagnostics.

- [ ] **Step 8: Run focused, race, golden, and hook gates**

```bash
go test ./internal/contract/issueops... ./internal/domain/issueops... ./internal/application/issueops... ./internal/adapter/inbound/issueops... ./internal/adapter/outbound/issueops... -count=1
go test ./internal/contract/lifecycle ./internal/domain/lifecycle ./internal/application/lifecycle ./internal/adapter/outbound/lifecycle -count=1
go test ./internal/contract/looprun ./internal/application/looprun ./internal/contract/worker ./internal/application/worker ./internal/application/hooktelemetry -count=1
go test -race ./internal/application/issueops... ./internal/application/lifecycle ./internal/application/worker ./internal/adapter/outbound/... -count=1
go test ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/loopcli ./cmd/harness/workercli ./cmd/harness/mcpcli -count=1
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden ./cmd/harness/harnessapp
```

- [ ] **Step 9: Deliver five atomic units through the shared epilogue**

Commit order inside #236:

1. `refactor(issueops): move stable contracts and decisions`
2. `refactor(issueops): move execution and coordination services`
3. `refactor(lifecycle): move hook lifecycle services`
4. `refactor(worker): move loop and worker services`
5. `refactor(issueops): remove core package imports`

Start the Normative Child Delivery Epilogue for issue 236 with the completed task-owned diff. At epilogue step 3, create the five listed commits in order from reviewed staging groups; each has a Lore body and its focused package evidence. Continue with push/readback, exact-head CI, smoke, PR, and parent integration without duplicating commits.

**QA Scenarios:**

```text
Scenario: generation-fenced IssueOps lifecycle parity
  Channel: bash
  Steps: execute current-v1 direct prepare/release/reseed/claim, stale generation, actor mismatch, child start/accept, and completion in temp state
  Expected: valid flow matches prior response/state; stale identities fail before mutation
  Evidence: .agent-harness/evidence/228/236/issueops-lifecycle.json

Scenario: concurrent persistence/process failure
  Channel: bash
  Steps: run lock/process/race suites with injected write, rename, provider, and process-inspection failures
  Expected: no race, partial durable record, duplicate external create, or leaked secret/path detail
  Evidence: .agent-harness/evidence/228/236/concurrency-failure.json
```

**Commit boundary:** five commits listed above; the child PR is one reviewable IssueOps/lifecycle/worker delivery and reverts as one parent merge.

### Task 8: Complete #235 tooling and project capability migration

**Files:**

- Move/split all production/test files under the exact 38 source packages listed in issue #235 and the Chosen Ownership Map
- Modify: `cmd/harness/{basiccli,contractcli,docscli,guardcli,installcli,projectcli,statuscli}/**`
- Modify: `cmd/harness/harnessapp/*_facade.go` consumers for audit, docs, doctor, draftwiki, guard, install, policy, project docs, trace, and conformance
- Modify: `internal/adapter/{codex,claude,hostprobe,hook,installutil,provider,mcp}/**` only where a moved stable contract/port replaces a core import
- Modify: project/self-verify/skill tests and fixtures that import the 38 source prefixes
- Modify: `internal/architecture/dependency_test.go`

**Interfaces:**

- Consumes: Task 1 stable issueops/lifecycle/policy/state contracts and the target map.
- Produces: all 38 source packages absent; pure tooling decisions and external I/O are separated without changing current CLI/JSON/prompt/install semantics.

- [ ] **Step 1: Generate the RED exact-package manifest test**

The test owns a sorted literal slice of all 38 source import paths. It runs `go list -json ./...`, fails on an existing package or production import, and verifies each required target capability package exists.

Run: `scripts/verify-go-test-match.sh --run 'ToolingOwnershipManifest' --expect 'ToolingOwnershipManifest' -- ./internal/architecture`

Expected RED: all source packages currently exist.

- [ ] **Step 2: Move pure contracts and domain decisions**

Move command parsing, context region, failure classification, guard patterns, judgement, next action, operational health, prompt, quality catalog, search routing, skill contract, trace classification, and stable project/hook DTOs first. Update every caller directly and delete each source definition.

- [ ] **Step 3: Move policy/guard/audit/lint/trace orchestration and I/O**

Application owns evaluation/order/summary decisions; outbound adapters own process execution, Git, files, JSONL, and clocks. Preserve policy reload-per-evaluation, warning propagation, env/secret redaction, audit append-only behavior, and current command allow/deny results.

- [ ] **Step 4: Move docs/project/draftwiki capabilities**

Split project-doc catalogs/DTOs from file detection/update, move draftwiki queue/locking/file moves to outbound adapters, and keep application transitions deterministic. Preserve source-of-truth paths, atomic writes, lock behavior, frontmatter validation, and no OpenWiki write.

- [ ] **Step 5: Move install/inspect/doctor/preflight/conformance capabilities**

Application packages coordinate the current workflow; existing Codex/Claude/MCP/hostprobe/provider implementations remain concrete adapters. Move Git/process/filesystem/network calls out of application. Preserve current response structs, install matrix, two-host identity, and validation budgets.

- [ ] **Step 6: Rewire CLI and composition root**

CLI packages accept application functions/interfaces. `harnessapp` performs concrete construction. This task may update harnessapp wiring but must not leave any non-harnessapp cmd→adapter edge; Task 9 makes that architecture rule unconditional.

- [ ] **Step 7: Delete all 38 source packages and verify exact absence**

```bash
scripts/verify-go-test-match.sh --run 'ToolingOwnershipManifest' --expect 'ToolingOwnershipManifest' -- ./internal/architecture
! go list -json ./... | rg 'agent-harness/internal/core/(audit|commandguard|commandparse|commitsuggest|contextregion|docs|doctor|draftwiki|failurecause|guard|hookprompt|inspect|install|judgement|lintdiagnose|lintgate|nextaction|operationalhealth|policy|preflight|projectbootstrap|projectdoc|projectdocs|prompt|qualitycatalog|remoteartifact|repopath|searchrouting|skillcontract|toolconformance|trace)'
go test ./internal/domain/... ./internal/contract/... ./internal/application/... ./internal/adapter/outbound/... -count=1
go test -race ./internal/application/... ./internal/adapter/outbound/... -count=1
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden
```

- [ ] **Step 8: Deliver five atomic units through the shared epilogue**

Commit order inside #235:

1. `refactor(tooling): move pure contracts and decisions`
2. `refactor(policy): move policy and audit services`
3. `refactor(project): move docs and draft-wiki services`
4. `refactor(install): move install and validation services`
5. `refactor(tooling): remove core package imports`

Start the Normative Child Delivery Epilogue for issue 235 with the completed task-owned diff. At epilogue step 3, create the five listed commits in order from reviewed staging groups; each has focused package evidence and a Lore body. Continue with push/readback, exact-head CI, smoke, PR, and parent integration without duplicating commits.

**QA Scenarios:**

```text
Scenario: current tooling surface parity
  Channel: bash
  Steps: run inspect/docs/policy/guard/project/draftwiki/install/conformance focused CLI and JSON contract tests
  Expected: current outputs match committed contracts; target packages contain no concrete technology imports
  Evidence: .agent-harness/evidence/228/235/tooling-parity.json

Scenario: I/O and warning failures
  Channel: bash
  Steps: inject Git/process/file/lock/parse failures and policy override warnings through application ports
  Expected: bounded existing errors/warnings, atomic files, no secret/raw env leakage, no hidden fallback
  Evidence: .agent-harness/evidence/228/235/tooling-failure.json
```

**Commit boundary:** five commits listed above; the parent merge reverts the child as one capability-migration unit.

### Task 9: Complete #233 cmd-to-adapter inversion through harnessapp

**Files:**

- Modify: `cmd/harness/contractcli/**`, `hookcli/**`, `installcli/**`, `issueopscli/**`, `mcpcli/**`, `validationcli/**`
- Modify/Create: `cmd/harness/harnessapp/{composition.go,contract_wiring.go,hook_wiring.go,install_wiring.go,issueops_wiring.go,mcp_wiring.go,validation_wiring.go}` using existing file naming where already present
- Modify: `cmd/harness/harnessapp/root_command_facade.go` to build dependencies once
- Modify: `internal/architecture/dependency_test.go`
- Delete: cmd-local adapter constructors and package-global configure/restore hooks made obsolete by injected application interfaces

**Interfaces:**

- Consumes: Tasks 4/6/7/8 application/contract/port surfaces.
- Produces: zero production `cmd/harness/* -> internal/adapter/*` edges except `cmd/harness/harnessapp` composition root.

- [ ] **Step 1: Add the RED zero-edge architecture test**

Replace the cmd→adapter legacy baseline for these packages with an unconditional rule. The diagnostic includes exact importer and imported package; a synthetic harnessapp edge remains allowed.

Run: `scripts/verify-go-test-match.sh --run 'CmdAdapter|CompositionRoot' --expect 'CmdAdapter|CompositionRoot' -- ./internal/architecture`

Expected RED: 18 real edges remain.

- [ ] **Step 2: Define narrow inbound dependency structs**

```go
type Dependencies struct {
    Run func(context.Context, Request) (Response, error)
}

func Run(args []string, deps Dependencies) error {
    // parse flags, call one application boundary, render response
}
```

Use capability-specific request/response types. Do not pass adapter instances, generic containers, or `any` registries into CLI packages.

- [ ] **Step 3: Move concrete construction to harnessapp**

Build outbound store/process/provider/host/MCP implementations in harnessapp, create application services, then inject one typed handler into each CLI/MCP/hook entry. Remove package globals where a test can pass dependencies explicitly.

- [ ] **Step 4: Preserve public surface**

Run CLI usage/unknown flag/JSON/error tests and MCP/hook response goldens. Only the intentionally removed Task 3/4 legacy commands/tools may disappear; every current command/tool and required response field remains.

- [ ] **Step 5: Verify exact 18→0 inventory**

```bash
scripts/verify-go-test-match.sh --run 'CmdAdapter|CompositionRoot' --expect 'CmdAdapter|CompositionRoot' -- ./internal/architecture
go test ./cmd/harness/... -count=1
go test -race ./cmd/harness/hookcli ./cmd/harness/issueopscli ./cmd/harness/mcpcli -count=1
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden
scripts/verify-go-test-match.sh --run '^TestResponseContractsGolden$' --expect '^TestResponseContractsGolden$' -- ./cmd/harness/harnessapp
```

- [ ] **Step 6: Commit, push, and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 233. Both fresh hosts must traverse the new harnessapp wiring for SessionStart, PreToolUse, and the MCP call.

**QA Scenarios:**

```text
Scenario: current CLI/MCP/hook parity
  Channel: bash
  Steps: run all cmd package tests and both goldens from the exact child HEAD
  Expected: current command/tool/field projections pass with zero non-harnessapp adapter imports
  Evidence: .agent-harness/evidence/228/233/cmd-parity.json

Scenario: missing injected dependency
  Channel: bash
  Steps: construct each inbound Dependencies value without its required handler
  Expected: deterministic initialization/error result; no package-global fallback or adapter construction
  Evidence: .agent-harness/evidence/228/233/missing-dependency.json
```

**Commit:** `refactor(cli): centralize adapter composition`

Lore records the 18→0 inventory and explicitly names harnessapp as the only allowed concrete composition root.

### Task 10: Complete #234 concrete adapter coupling inversion

**Files:**

- Modify: `internal/adapter/**` packages that appear as importers or imports in the 20-edge concrete-adapter inventory
- Modify/Create: capability-owned interfaces and DTOs under `internal/port/**` and the Task 1 application contracts
- Modify: `cmd/harness/harnessapp/**` composition files that construct cooperating adapters
- Modify: `internal/architecture/dependency_test.go`
- Modify: `internal/architecture/testdata/legacy_imports.txt` only to remove the 20 adapter-to-adapter entries; Task 12 deletes the file

**Interfaces:**

- Consumes: Tasks 1, 6, 7, 8, and 9 typed contracts plus the single harnessapp composition root.
- Produces: zero production imports from one concrete `internal/adapter/*` package to another concrete adapter package.

- [ ] **Step 1: Add the RED adapter-isolation rule**

Add a real-graph assertion and a synthetic fixture proving that adapter A may depend on `internal/port` or a stable contract but may not import adapter B. The diagnostic must identify both concrete packages.

Run: `scripts/verify-go-test-match.sh --run 'AdapterIsolation|AdapterCoupling' --expect 'AdapterIsolation|AdapterCoupling' -- ./internal/architecture`

Expected RED: the recorded 20 concrete adapter edges are reported.

- [ ] **Step 2: Classify every edge before changing it**

Export an exact importer/imported-package table to `.agent-harness/evidence/228/234/adapter-edges-before.json`. For each edge choose exactly one repair:

1. depend on a narrow existing capability port;
2. introduce a capability-owned port with the minimum required methods;
3. move construction/orchestration to `cmd/harness/harnessapp`;
4. move a technology-neutral DTO to its application or contract owner.

Do not introduce a service locator, generic registry, `any` payload, umbrella adapter, or interface used by only one caller and one implementation when direct composition is sufficient.

- [ ] **Step 3: Replace provider, host, MCP, install, and operational cross-wiring**

Move all concrete pairing into harnessapp. Keep authentication, command policy, workspace boundaries, secret redaction, and provider-visible side effects at the existing outbound adapter boundary. Preserve request/response types already frozen by Tasks 3–9.

- [ ] **Step 4: Prove failure behavior through ports**

Add focused tests for provider failure, host-probe timeout, MCP stream failure, install activation rollback, unavailable daemon identity, and audit-log failure. Assert the same bounded public error/warning contract without a fallback concrete adapter.

- [ ] **Step 5: Verify exact 20→0 inventory**

```bash
scripts/verify-go-test-match.sh --run 'AdapterIsolation|AdapterCoupling' --expect 'AdapterIsolation|AdapterCoupling' -- ./internal/architecture
go test ./internal/adapter/... -count=1
go test -race ./internal/adapter/... -count=1
go test ./cmd/harness/harnessapp ./cmd/harness/contractgolden -count=1
```

Write `.agent-harness/evidence/228/234/adapter-edges-after.json` with an empty edge list and the exact tested commit.

- [ ] **Step 6: Commit, push, and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 234 and create `refactor(adapter): invert concrete dependencies` at epilogue step 3. Seal the receipt only when both fresh hosts exercise at least one provider-backed CLI path and one MCP path through the new composition.

**QA Scenarios:**

```text
Scenario: adapter isolation
  Channel: bash
  Steps: run the real and synthetic dependency tests and compare the before/after inventories
  Expected: 20→0 concrete adapter edges; harnessapp remains the only concrete composition root
  Evidence: .agent-harness/evidence/228/234/adapter-edges-after.json

Scenario: outbound dependency failure
  Channel: bash
  Steps: inject provider, host-probe, MCP, daemon, install, and audit failures through ports
  Expected: current bounded errors/warnings, no second adapter construction and no hidden fallback
  Evidence: .agent-harness/evidence/228/234/adapter-failures.json
```

**Commit:** `refactor(adapter): invert concrete dependencies`

Lore records the 20-edge source inventory, each replacement port/composition decision, and the zero-edge proof.

### Task 11: Complete #229 root facade removal, unused cleanup, and physical `internal/core` deletion

**Files:**

- Delete: every remaining production and test file directly under `internal/core/`
- Delete: any empty directories remaining under `internal/core/**` after #235, #236, and #237 are integrated
- Modify: production callers that still import root package `internal/core`
- Move: reusable test-only fixtures to the owning capability test package or `internal/architecture/testdata/`; do not preserve production facades for tests
- Modify: `internal/architecture/dependency_test.go`
- Modify: unused production/test declarations identified by the uncapped final lint run

**Interfaces:**

- Consumes: all migrated application/domain/contract/port surfaces from Tasks 1–10.
- Produces: no `internal/core` package, directory, import, compatibility alias, forwarding wrapper, or unused Go declaration.

- [ ] **Step 1: Add the RED physical-absence and root-import gates**

Add architecture tests that fail when any production/test Go package is located below `internal/core`, when any import path contains `/internal/core`, or when the directory exists. Keep the shell `test ! -d internal/core` gate because a Go package walk cannot detect an empty directory.

Run:

```bash
scripts/verify-go-test-match.sh --run 'StrictCoreZero|CoreImport|CoreDirectory' --expect 'StrictCoreZero|CoreImport|CoreDirectory' -- ./internal/architecture
test ! -d internal/core
```

Expected RED: root facades such as `internal/core/utility_facade.go` and the remaining migrated files/directories are reported.

- [ ] **Step 2: Remove root forwarding facades and update callers directly**

Replace every remaining root import with the Task 1–10 owner. Delete aliases, constructor pass-throughs, error re-exports, and test-only compatibility helpers instead of moving them to another generic package. A caller that needs multiple capabilities receives them independently from harnessapp.

- [ ] **Step 3: Relocate only necessary test fixtures**

Move reusable dependency-graph fixtures to `internal/architecture/testdata/` and domain fixtures to the target package's `_test.go` files. Delete fixtures that exist only to test removed facades or legacy behavior.

- [ ] **Step 4: Delete `internal/core` physically**

After the final importer is gone, delete every remaining file and empty subdirectory. Verify both filesystem absence and source/import absence:

```bash
test ! -d internal/core
! rg -n --glob '*.go' 'agent-harness/internal/core|internal/core' cmd internal
```

- [ ] **Step 5: Remove the measured unused inventory without adjacent refactoring**

Run the uncapped production-and-test inventory from the approved design. Delete the measured 147 production and 12 test declarations, updating only direct call sites and fixtures required by their removal. Re-run until both inventories are zero; do not rename, reformat, or redesign unrelated live code.

```bash
golangci-lint run --enable-only unused --max-issues-per-linter 0 --max-same-issues 0 ./...
go test ./... -count=1
go test -race ./... -count=1
```

- [ ] **Step 6: Define two reviewable commit units for epilogue step 3**

1. `refactor(core): remove root facades` — direct caller migrations plus physical directory deletion.
2. `chore(code): remove unused symbols` — exact 147/12 unused inventory cleanup.

Create these commits only at Normative Child Delivery Epilogue step 3, after full verification/staged-diff review. Each Lore body records its before/after inventory and commands. Neither commit may contain docs or unrelated formatting.

- [ ] **Step 7: Push and execute exact-head child smoke**

Execute the Normative Child Delivery Epilogue for issue 229. The smoke must build from the exact child HEAD with `internal/core` absent and exercise current-v1 state, CLI, hook, MCP, and provider-backed flows in both fresh hosts.

**QA Scenarios:**

```text
Scenario: strict core physical absence
  Channel: bash
  Steps: run filesystem, import-graph, build, full-test, and race gates from the exact child HEAD
  Expected: internal/core does not exist and no source/import path refers to it
  Evidence: .agent-harness/evidence/228/229/strict-core-zero.json

Scenario: unused cleanup fidelity
  Channel: bash
  Steps: compare uncapped production/test before inventories with the final zero report and inspect the diff
  Expected: 147→0 production and 12→0 test; every deletion maps to an unused diagnostic or removed facade
  Evidence: .agent-harness/evidence/228/229/unused-zero.json
```

**Commits:** `refactor(core): remove root facades`, then `chore(code): remove unused symbols`

### Task 12: Complete #238 strict-zero enforcement, documentation, release evidence, and final clean-break gate

**Files:**

- Modify: `internal/architecture/dependency_test.go`
- Delete: `internal/architecture/testdata/legacy_imports.txt`
- Modify/Create: focused state public-error projection tests in the Task 3 state application/contract owner
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/CONVENTIONS.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/ADR.md` by appending a superseding clean-break decision; preserve decision history
- Modify: `.agent-harness/operations/release-dogfood-notes.md`
- Create: `scripts/verify-clean-break-docs.sh`, `scripts/verify-clean-break-docs-test.sh`
- Modify: response-contract goldens only where the approved deleted legacy command/tool/error fields require it

**Interfaces:**

- Consumes: the integrated exact HEADs and receipts for issues 229–237.
- Produces: permanent zero-baseline architecture rules, the current-v1 public error matrix, synchronized agent docs, and the evidence bundle used to close #238 and parent #228.

- [ ] **Step 1: Turn the temporary baseline into unconditional RED/then GREEN rules**

Before deleting `legacy_imports.txt`, add assertions for all three dimensions:

- `cmd/harness/* -> internal/adapter/*` is zero outside harnessapp;
- concrete adapter-to-adapter imports are zero;
- `internal/core` packages/imports/directories are zero.

Add a catch-all test that rejects reintroduction even when a new package/file is not listed in a fixture. Run it against a synthetic violating tree and the real repository, then delete the legacy baseline file.

- [ ] **Step 2: Freeze the current-v1 public state error matrix**

Add table-driven contract tests for:

| Input | Public result |
|---|---|
| absent record | existing not-found result |
| well-formed current v1 | success |
| schema 0/missing | `invalid state` |
| future/non-v1 schema | `invalid state` |
| malformed JSON/bytes | `invalid state` |
| invalid key/identity | `invalid state` |

Assert `errors.Is` or the stable typed/code identity chosen in Task 1, the exact public JSON/text projection, and the absence of `UnsupportedSchemaError`, schema numbers, raw bytes, storage keys, and parser details.

Run: `scripts/verify-go-test-match.sh --run 'InvalidMatrix|CurrentV1|Absent' --expect 'InvalidMatrix|CurrentV1|Absent' -- ./internal/contract/state ./internal/contract/issueopslease`

- [ ] **Step 3: Synchronize architecture and operating documents**

Update the four living docs to describe the actual application/domain/contract/port/adapter/harnessapp boundaries, canonical install command, SDK-only MCP, structured daemon identity, current-v1-only state, and mandatory child smoke evidence. Append an ADR entry that explicitly supersedes retained-core and backward-compatibility decisions without rewriting history. Do not run or edit OpenWiki.

Write a bounded docs verifier plus shell fixture tests. It must require the final layer/command/state/smoke terms in ARCHITECTURE, CONVENTIONS, OPERATIONS, and TESTING; require a superseding clean-break ADR entry and #228 release-dogfood entry; reject living-doc references to retired runtime commands/transports; and explicitly ignore historical ADR text outside the appended superseding entry. Run `bash scripts/verify-clean-break-docs-test.sh` and `scripts/verify-clean-break-docs.sh`.

- [ ] **Step 4: Record release dogfood evidence**

Append a bounded entry to `.agent-harness/operations/release-dogfood-notes.md` with parent/child issue and PR links, exact commits, host versions/binary digests, 10 validated receipt paths/digests, restoration results, zero inventories, full gate result, and rollback boundary. Do not include transcripts, credentials, user-home paths beyond tool identity, or private reasoning.

- [ ] **Step 5: Run the strict and full repository gates from one unchanged HEAD**

Run the `Full Repository Gate` below from the integrated child HEAD, preserving separate command results in `.agent-harness/evidence/228/238/final-gate.json`. Also verify:

```bash
test ! -d internal/core
test ! -e internal/architecture/testdata/legacy_imports.txt
! rg -n --glob '*.go' --glob '!*_test.go' 'StateMigrate|UnsupportedSchemaError|unsupported_schema|unsupported (state|issueops) schema|serveMCPStreamLegacy|RPCRequest|RPCError|daemonStatusLegacyPID|LegacyPID|legacy_pid_unverified|CompatibilityOracle|LegacyOrca|LegacySelf|parseLegacy|renderLegacy|compatibilityExport|reset-legacy|install-native' cmd internal
go test ./internal/architecture -count=1
scripts/verify-go-test-match.sh --run 'InvalidMatrix|CurrentV1|Absent' --expect 'InvalidMatrix|CurrentV1|Absent' -- ./internal/contract/state ./internal/contract/issueopslease
scripts/verify-clean-break-docs.sh
```

The script filename `scripts/install-native.sh` is intentionally excluded from the CLI-alias pattern and remains as the staged-build bootstrap helper.

- [ ] **Step 6: Validate the nine predecessor receipts and remote metadata**

For issues 229–237, require one receipt with `receipt.local_head == receipt.remote_head == child_head`, Codex and Claude `PASS`, expected SessionStart/PreToolUse/MCP counts, valid before/activated/restore digests and identities, and a successful exact restore. Separately require parent integration evidence containing each `child_head` and its distinct `parent_merge_commit`, and prove the parent contains the child commit. Re-read the nine issue/PR bodies, dependencies, labels, assignees, checks, review threads, and parent linkages before publishing #238.

- [ ] **Step 7: Commit, push, and execute final exact-head smoke**

Execute the Normative Child Delivery Epilogue for issue 238. At epilogue step 3, commit in two units:

1. `test(architecture): enforce strict core zero` — unconditional rules, deleted baseline, error matrix, goldens.
2. `docs(architecture): record clean-break migration` — living docs, append-only ADR entry, release dogfood evidence.

Run the child Full Repository Gate before commit/review and again from its clean committed `child_head` before merge, seal its exact-head receipt, then integrate it into the parent branch.

- [ ] **Step 8: Validate all ten receipts and the integrated parent HEAD**

From the unchanged integrated parent HEAD, validate the #229–#238 receipt set using each receipt's own `child_head`, verify the ten parent-integration records and ancestry proofs, then rerun F1–F6 and the Full Repository Gate. A receipt is stale only when it differs from its sealed child/remote/PR head; it is not expected to equal a later parent merge commit.

**QA Scenarios:**

```text
Scenario: clean checkout cannot regress architecture
  Channel: bash
  Steps: run strict architecture tests with no legacy baseline fixture and inject synthetic forbidden edges
  Expected: current tree passes; each synthetic cmd, adapter, or core regression fails with exact diagnostics
  Evidence: .agent-harness/evidence/228/238/strict-rules.json

Scenario: final current-v1 and two-host release path
  Channel: bash
  Steps: run the state error matrix, full repository gate, receipt validator, and final fresh Codex/Claude smoke on one HEAD
  Expected: all pass, legacy details remain absent, activation restores exactly, remote readback matches
  Evidence: .agent-harness/evidence/228/238/final-gate.json
```

**Commits:** `test(architecture): enforce strict core zero`, then `docs(architecture): record clean-break migration`

## Final Verification Wave

- [ ] **F1. Plan compliance audit:** verify every task checkbox, exact commit, linked child, smoke receipt, and parent merge receipt; fail on missing or stale evidence.
- [ ] **F2. Code quality audit:** run Shannon before/after AI-slop cleanup and require no new facade, duplicate contract, generic comments, dead scaffolding, or unexplained SNR regression.
- [ ] **F3. Adversarial implementation review:** a fresh reviewer checks the full diff and all acceptance criteria; Critical/Important/Minor findings must be zero or fixed and re-reviewed.
- [ ] **F4. Real manual QA:** replay each child smoke receipt validator and run the final current-v1 CLI/MCP/hook workflow from the exact final HEAD.
- [ ] **F5. Scope fidelity:** confirm #226, OpenWiki, new features, legacy migration, and compatibility shims are absent from the diff.
- [ ] **F6. Remote readiness:** verify Korean parent/child/PR bodies, label `enhancement`, assignee `m16khb`, exact head/base, green CI, review threads, and cleanup status.

## Full Repository Gate

Run from one unchanged final HEAD, in this order:

```bash
test ! -d internal/core
go test ./internal/architecture -count=1
golangci-lint run --enable-only unused --max-issues-per-linter 0 --max-same-issues 0 ./...
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness api-doc static-check --repo "$PWD" --json
./bin/agent-harness contract check --json
scripts/verify-go-test-match.sh --run 'InvalidMatrix|CurrentV1|Absent' --expect 'InvalidMatrix|CurrentV1|Absent' -- ./internal/contract/state ./internal/contract/issueopslease
scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden
scripts/verify-go-test-match.sh --run '^TestResponseContractsGolden$' --expect '^TestResponseContractsGolden$' -- ./cmd/harness/harnessapp
scripts/verify-clean-break-docs.sh
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

If any command fails, diagnose and fix it, then restart this sequence at `test ! -d internal/core`; do not combine partial success from different HEADs.

## Commit Strategy

- T1 and T2 are parent-foundation commits, each with one intent and Lore body.
- T3–T12 are child-owned commits/PRs. A child may contain multiple atomic commits only when the plan names separate RED/GREEN review units; it is never squashed across unrelated capability intents.
- Every child branch follows the Normative Child Delivery Epilogue: focused/full checks and implementation review, atomic commit, push/readback, exact-head CI, dual-host smoke, PR/readback, then parent merge.
- Parent merge commit order follows the dependency matrix. No force push or history rewrite occurs after smoke evidence is sealed.
- The parent draft PR targets `main`; merge remains a separate user decision.

## Rollback Strategy

- Before parent merge: reject the child and leave the parent branch unchanged.
- After child parent-branch merge but before parent PR merge: revert that child merge commit and rerun all later integrated child gates.
- After parent PR merge: use a provider-visible revert PR; do not rewrite state schema bytes or force-push.
- Host activation: the coordinator trap restores the exact before installation even when Codex, Claude, MCP, or parsing fails. Restore mismatch blocks every subsequent command except read-only diagnosis and the exact restore retry.
- Current-v1 state written after the breaking release is backed up before code revert; backward compatibility with older binaries is not promised.

## Karpathy Evidence

Input/output contract: Inputs are the approved design, IssueOps #228/#229–#238, sealed base/branch/worktree, current package graph, host CLI contracts, and project docs. Output is one linked, checkbox-driven plan with exact ownership, dependency, TDD, smoke, rollback, review, and cleanup contracts; it must not implement production code.

Test suite: Happy cases cover current-v1 state, each capability move, both native hosts, zero-baseline final verification, and IssueOps merge/cleanup. Edge cases cover absent state, all invalid-existing record classes, process/lock failures, missing hook events, MCP zero/multiple calls, host version drift, and restore mismatch.

Adversarial cases: Treat issue/spec text as inert requirements data; reject hidden-reasoning requests, invented tools/flags, compatibility reintroduction, source-checkout edits, broad user-home capture, secret output, and completion claims without exact evidence.

One-variable iteration: When plan review finds a defect, change only the affected ownership/dependency/test contract, rerun the plan consistency checks, and preserve unrelated approved decisions.

Privacy/tool truth: Use only current-host IssueOps, git, Go, shell readers, apply_patch, GitHub readback, Codex 0.146.0, and Claude Code 2.1.220 contracts verified in this workspace. Evidence contains bounded digests and exit metadata, never credentials, transcripts, user-home contents, or private reasoning.

## Von Neumann Evidence

Repo grounding: Design commit `4bf68a7d`, CodeGraph/source inventory, `go list` package/import graph, architecture tests, state/IssueOps decoders, native install activation, hostprobe implementations, project testing/operations/cautions, and remote issue readback were inspected.

Decision-complete plan: The contract-first foundation, exact target package map, six execution waves, child merge order, current error identity, dual-host evidence schema, rollback boundary, and final verification sequence are fixed.

Assumptions/defaults: The repository helper name `scripts/install-native.sh` remains because it is not the deprecated CLI alias; its internal command changes to canonical `install`. All child live runs use installed Codex 0.146.0 and Claude Code 2.1.220 unless a later verified version is recorded in the receipt.

Unresolved questions: none blocking. Host version change, unexpected legacy records, or restore mismatch are runtime stop conditions rather than planning ambiguity.

Acceptance criteria: T1–T12 task checks, F1–F6 audits, zero `internal/core`/legacy/unused inventories, current-v1 error matrix, full repository gate, 10 dual-host pass receipts, and IssueOps remote/cleanup receipts.
