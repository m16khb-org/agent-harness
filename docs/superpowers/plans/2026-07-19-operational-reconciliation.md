# Operational Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `agent-harness doctor` reject cross-system Git, IssueOps, Orca, and user-state residue through one deterministic operational-health model, then use a sealed external recovery bundle to remove every currently approved non-main/stale artifact while preserving the exact current operator terminal.

**Architecture:** Add one pure `internal/core/operationalhealth` classifier and one read-only adapter collector. The existing top-level `doctor` receives the normalized snapshot and invocation-scoped preserve sets, while IssueOps stale scan reuses the classifier's cycle-authority rule without broadening automatic release eligibility. The stability audit delegates operational judgment to the built `doctor`. The live cleanup remains an explicitly approved, one-time external runner stored in a `0700` recovery bundle; it is not a product command or MCP surface.

**Tech Stack:** Go 1.26.3, Go standard library, existing SQLite `sqlstore`, existing Orca CLI adapter, Python 3 standard library for the external one-time manifest/journal runner, Git plumbing commands, existing Go/Python test suites.

## Global Constraints

- The approved design at `docs/superpowers/specs/2026-07-19-operational-reconciliation-design.md` is authoritative. If implementation pressure requires a semantic change, stop and amend/re-review the design before coding further.
- The main agent owns all cross-architecture decisions and every destructive live-state mutation. No sub-agent may close terminals, reset Orca, release IssueOps records, remove worktrees, delete refs, relocate state artifacts, push `main`, or edit the external journal.
- Do not add a cleanup/reconcile CLI command, MCP tool, persistent exemption registry, background reaper, scheduler, or generic orchestrator abstraction.
- `binding` proves ownership only. A claimed cycle is live only with exact resource identity and a fenced heartbeat no older than `15m`; heartbeat age alone never authorizes interrupt, delete, or release.
- `--preserve-cycle` and `--preserve-terminal` are repeatable, non-empty, exact, invocation-scoped flags. They never write state and never cure incomplete or ambiguous identity.
- Any incomplete list, duplicate identity, count mismatch, truncation, unsupported state, parse failure, or collection error is `operational_inventory_unknown` and makes doctor unhealthy.
- Operational-health product code remains read-only. The one-time external runner may mutate only targets sealed in its manifest, using existing exact primitives plus the fail-safe paired-digest mode on `issueops force-release`, and must stop on any identity, digest, OID, owner, or inventory drift.
- The current operator terminal is resolved at execution time from non-empty `ORCA_TERMINAL_HANDLE` and must match exactly one current Orca terminal row. No guessed handle is accepted.
- Preserve unrelated user changes. Before every commit and before creating the cleanup manifest, require a clean source checkout except for the exact files of the current task.
- Use CodeGraph before new code-location exploration. Use `apply_patch` for source and external-runner edits. Never use broad globs, unresolved deletion targets, `git branch -D`, plain `git push --delete`, or direct SQLite file copying as a backup claim.
- Every code task ends with a focused test and an atomic Conventional Commit + Lore body. Do not push until Tasks 1–7 pass the pre-cleanup verification gate.
- The final success claim requires direct output from the complete final verification window; partial/yielded test output is not success evidence.
- Uppercase `EXACT_*` tokens in command illustrations are field names, not shell placeholders. The external runner passes the already sealed manifest value as one argv element; operators never paste or interpolate those tokens manually.

## Acceptance Map

| Requirement | Primary implementation | Primary verification |
|---|---|---|
| Ownership/liveness/preservation/residue are distinct | `internal/core/operationalhealth` | classifier table tests |
| Bound cycle without fresh heartbeat is unhealthy | classifier first RED | exact named regression |
| Orca task/gate/message observations are complete and redacted | `internal/adapter/orca` | fake-runner contract tests |
| Git/IssueOps/Orca inventory failures fail closed | `internal/adapter/operationalhealth` | collector integration tests |
| Existing doctor is the only public operational-health command | doctor request projection + CLI flags | CLI tests and unchanged MCP/command catalog |
| Stale scan shares liveness but not auto-release policy | IssueOps stale scan wiring | heartbeat-only non-release regression |
| Stability audit delegates to doctor | stability audit script | Python unit tests + live final audit |
| Live deletion is exact, journaled, and recoverable | external bundle runner | synthetic runner tests + per-op readback |
| Final state has only canonical/main and current terminal | live cleanup stages | fresh full inventories |
| No regression or hidden residue remains | final gate | test/race/vet/build/goldens/self-verify/stability audit |

---

### Task 0: Version the approved implementation plan

**Files:**
- Add: `docs/superpowers/plans/2026-07-19-operational-reconciliation.md`

- [ ] **Step 1: Re-read the design and plan diff together**

Run:

```bash
git diff --check
git diff -- docs/superpowers/specs/2026-07-19-operational-reconciliation-design.md docs/superpowers/plans/2026-07-19-operational-reconciliation.md
```

Expected: no whitespace errors; the plan contains every acceptance item from design sections 7–12 and does not introduce a public cleanup surface.

- [ ] **Step 2: Run a placeholder and forbidden-scope scan**

Run:

```bash
rg -n 'TO[D]O|TB[D]|FIXM[E]|new .*cleanup command|persistent exemption|background reaper' docs/superpowers/plans/2026-07-19-operational-reconciliation.md
```

Expected: no unresolved placeholder; forbidden-scope phrases appear only in explicit prohibitions.

- [ ] **Step 3: Commit the plan**

Use `atomic-commit-push` for an atomic local commit with subject:

```text
docs(operations): plan operational reconciliation
```

Do not push yet.

---

### Task 1: Establish the cycle-authority invariant with the mandatory first RED

**Files:**
- Create: `internal/core/operationalhealth/types.go`
- Create: `internal/core/operationalhealth/classifier.go`
- Create: `internal/core/operationalhealth/classifier_test.go`

- [ ] **Step 1: Add only the first failing regression**

Define normalized, I/O-free inputs. The core types use parsed `time.Time` values and exact strings, not raw JSON or implicit wall-clock reads:

```go
const HeartbeatTTL = 15 * time.Minute

type Cycle struct {
	ID, Repo, Branch, Phase, HandoffState string
	Attempt                               int
	OwnershipEpoch, ContextSHA256         string
	WorkerSessionID, WorkerAgentID        string
	WorktreePath, OrcaWorktreeID          string
	OrcaWorktreeInstanceID                string
	TerminalHandle, PTYID                 string
	TaskID, DispatchID                    string
	LastHeartbeatAt                       time.Time
}

type Binding struct {
	CycleID, Repo, Branch, ExpectedWorktree string
}

type Options struct {
	Now                     time.Time
	PreserveCycleIDs        []string
	PreserveTerminalHandles []string
}
```

Add the exact test:

```go
func TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat(t *testing.T)
```

Fixture: one non-done cycle, exact binding, exact ready task, complete task inventory, no heartbeat. Assert `Healthy == false` and one `operational_dead_owner` finding for that cycle.

- [ ] **Step 2: Run the named test and retain the RED exit**

Run:

```bash
go test ./internal/core/operationalhealth -run '^TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat$' -count=1
```

Expected: FAIL for the missing classifier behavior, not for a malformed fixture or unrelated compile error.

- [ ] **Step 3: Implement the smallest cycle-authority evaluator**

Add:

```go
type CycleAuthority string

const (
	AuthorityLive      CycleAuthority = "live"
	AuthorityPreserved CycleAuthority = "preserved"
	AuthorityDead      CycleAuthority = "dead"
	AuthorityUnknown   CycleAuthority = "unknown"
)

func EvaluateCycleAuthority(c Cycle, opts Options) CycleAuthority
func Classify(snapshot Snapshot, opts Options) Result
```

Rules for this first pass only:

- `phase=done` is terminal and produces no dead-owner finding by itself; only a remaining owned resource becomes residue.
- `handoff_state=claimed` requires complete attempt/epoch/context/worker identity and a heartbeat where `0 <= opts.Now.Sub(heartbeat) <= HeartbeatTTL`.
- A non-claimed, non-done cycle is preserved only when its exact ID appears in `PreserveCycleIDs` and its durable identity is internally complete.
- Binding/resource presence cannot upgrade a dead owner.
- An empty or zero `Now`, unknown phase/state, future heartbeat, or incomplete claimed authority is unknown/dead and unhealthy.

- [ ] **Step 4: Re-run the named test, then the package**

Run:

```bash
go test ./internal/core/operationalhealth -run '^TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat$' -count=1
go test ./internal/core/operationalhealth -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the authority invariant**

Commit subject:

```text
feat(doctor): classify dead operational owners
```

---

### Task 2: Complete the pure cross-resource classifier

**Files:**
- Modify: `internal/core/operationalhealth/types.go`
- Modify: `internal/core/operationalhealth/classifier.go`
- Modify: `internal/core/operationalhealth/classifier_test.go`

- [ ] **Step 1: Add the remaining normalized resource types and finding contract**

Use bounded, secret-free projections:

```go
type GitWorktree struct { Path, Branch, Head string; Clean bool; Canonical bool }
type GitRef struct { Name, OID, Location string }
type OrcaWorktree struct { RuntimeID, ID, InstanceID, Repo, Path, Branch, Head string }
type OrcaTerminal struct { RuntimeID, Handle, PTYID, WorktreeID, WorktreePath string; Connected, Writable bool }
type OrcaTask struct { ID, Status, DispatchID string; CompletedAt time.Time; HasResult bool }
type OrcaGate struct { ID, TaskID, Status string }
type MessagePresence struct { Count int; Empty bool; CompleteAbsence bool }
type InventoryProblem struct { Source, Code, Detail string }
type StateArtifact struct { Path, Code string }

type Finding struct {
	Code, ResourceKind, ResourceID, Summary, Path string
}

type Result struct {
	Healthy  bool
	Findings []Finding
}
```

`Snapshot` owns canonical repo/branch/source identity plus cycles, bindings, Git refs/worktrees, Orca resources, message presence, state artifacts, and inventory problems. Sort and deduplicate findings deterministically by `(code, resource_kind, resource_id, path, summary)`.

- [ ] **Step 2: Add table-driven RED cases before implementation**

Cover exactly:

- fresh claimed heartbeat plus exact unique worktree/terminal/task/dispatch is live and healthy;
- stale and missing heartbeat remain dead despite exact binding/resources;
- exact preserved planning/dispatched/submitted/recovery/no-handoff cycle is healthy only for that invocation;
- persistent binding without preserve is dead;
- duplicate owner/resource ID and incomplete inventory are unknown;
- ready task with `CompletedAt` or `HasResult` is `operational_task_residue`;
- done cycle with any exact external resource produces the matching residue finding;
- unmatched terminal/worktree/task/gate/message produces the matching stable code;
- local and remote non-canonical refs aggregate under only `operational_non_main_branch_residue`;
- unexpected state files/directories produce `operational_state_artifact_residue`;
- unknown phase/task/handoff/gate status fails closed;
- heartbeat exactly at 15 minutes is accepted and one nanosecond beyond is rejected;
- duplicate/blank preserve values cannot hide ambiguity.
- an exact preserved terminal is healthy only when that handle occurs once; a missing or duplicate preserved handle is unknown.

- [ ] **Step 3: Run the new table and verify RED**

Run:

```bash
go test ./internal/core/operationalhealth -run 'TestClassifyOperationalHealth' -count=1
```

Expected: FAIL on the newly asserted resource rules.

- [ ] **Step 4: Implement exact ownership matching and residue aggregation**

Use maps only as indexes; preserve deterministic sorted output. A resource is healthy only when it has exactly one live/preserved exact owner. A cycle claiming a missing or duplicate resource is `operational_inventory_unknown`; unmatched resources receive their specific residue code. Never treat a binding as heartbeat or liveness evidence.

The only public finding constants are:

```text
operational_inventory_unknown
operational_dead_owner
operational_worktree_residue
operational_terminal_residue
operational_task_residue
operational_gate_residue
operational_message_residue
operational_non_main_branch_residue
operational_state_artifact_residue
```

- [ ] **Step 5: Verify the package and race behavior**

Run:

```bash
go test ./internal/core/operationalhealth -count=1
go test -race ./internal/core/operationalhealth -count=1
```

Expected: PASS with no data race and no clock/global-state dependency.

- [ ] **Step 6: Commit the classifier**

Commit subject:

```text
feat(doctor): detect cross-system residue
```

---

### Task 3: Add complete, redacted Orca inventory reads

**Files:**
- Modify: `internal/port/orca.go`
- Modify: `internal/adapter/orca/client.go`
- Modify: `internal/adapter/orca/client_test.go`
- Add: `internal/adapter/orca/testdata/task_list_all.json`

- [ ] **Step 1: Write fake-runner RED tests for the installed CLI contracts**

Add tests for:

```go
func TestClientListAllTasksProjectsCompletionSemanticsWithoutRawResult(t *testing.T)
func TestClientListAllTasksRejectsCountMismatch(t *testing.T)
func TestClientListGatesRequiresCountEquality(t *testing.T)
func TestClientInboxPresenceProvesOnlyBoundedZero(t *testing.T)
func TestClientResolveRepoReturnsCanonicalRegistration(t *testing.T)
```

Pin these commands exactly:

```text
orca orchestration task-list --brief --json
orca orchestration gate-list --json
orca orchestration inbox --limit 1 --json
orca repo show --repo path:/absolute/repo --json
```

The task DTO adds only `CompletedAt string` and `HasResult bool`; it never retains raw `spec`, `result`, message `body`, message `payload`, or subject. Gate rows retain only identity/status metadata. Inbox retains only returned count, row length, and `CompleteAbsence = count == 0 && len(messages) == 0`.

- [ ] **Step 2: Run the focused adapter tests and verify RED**

Run:

```bash
go test ./internal/adapter/orca -run 'TestClient(ListAllTasks|ListGates|InboxPresence|ResolveRepo)' -count=1
```

Expected: FAIL because the new methods and DTO fields do not exist.

- [ ] **Step 3: Add narrow methods without changing `port.OrcaClient`**

Add concrete `*orca.Client` methods and additive port DTOs:

```go
func (c *Client) Available() bool
func (c *Client) ResolveRepo(ctx context.Context, repo string) (port.OrcaRepo, error)
func (c *Client) ListAllTasks(ctx context.Context) ([]port.OrcaTask, error)
func (c *Client) ListGates(ctx context.Context) ([]port.OrcaGate, error)
func (c *Client) InboxPresence(ctx context.Context) (port.OrcaInboxPresence, error)
```

Do not add these methods to the broad `port.OrcaClient`; the operational collector declares its own narrow read interface. Preserve existing ready/dispatched task behavior.

- [ ] **Step 4: Enforce completeness and semantic projection**

- Task and gate `count` must be present and equal returned rows.
- Inbox absence is proven only by the installed `limit=1` response with both zero count and zero rows. Any row is presence/residue, not truncation success.
- `result` is decoded only to determine non-empty presence and discarded immediately.
- Any malformed row identity becomes an adapter error so the collector emits unknown.

- [ ] **Step 5: Verify old and new adapter contracts**

Run:

```bash
go test ./internal/adapter/orca -count=1
go test ./internal/core/issueops -run 'Handoff|Dispatch' -count=1
```

Expected: PASS; existing IssueOps fakes need no new broad-interface methods.

- [ ] **Step 6: Commit the Orca observation surface**

Commit subject:

```text
feat(orca): expose complete health inventory
```

---

### Task 4: Collect Git, IssueOps, binding, and optional Orca state fail-closed

**Files:**
- Create: `internal/adapter/operationalhealth/collector.go`
- Create: `internal/adapter/operationalhealth/collector_test.go`
- Modify: `internal/core/operationalhealth/types.go`

- [ ] **Step 1: Define one narrow collector and injected command boundary**

Use these signatures:

```go
type OrcaInventory interface {
	Available() bool
	Status(context.Context) (port.OrcaStatus, error)
	ResolveRepo(context.Context, string) (port.OrcaRepo, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	ListTerminals(context.Context, string) ([]port.OrcaTerminal, error)
	ListAllTasks(context.Context) ([]port.OrcaTask, error)
	ListDispatchedTasks(context.Context) ([]port.OrcaTask, error)
	ShowDispatch(context.Context, string) (port.OrcaDispatch, error)
	ListGates(context.Context) ([]port.OrcaGate, error)
	InboxPresence(context.Context) (port.OrcaInboxPresence, error)
}

type GitRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type Collector struct { Git GitRunner; Orca OrcaInventory }
func (c Collector) Collect(ctx context.Context, repo string) operationalhealth.Snapshot
```

The collector returns a snapshot even on failure and appends bounded `InventoryProblem` rows instead of turning failures into empty healthy lists.

- [ ] **Step 2: Add RED integration cases**

Use a real temporary Git repository, an isolated `HARNESS_STATE_DIR`, real IssueOps SQLite records/bindings, and a fake narrow Orca reader. Cover:

- canonical source branch/HEAD/clean state;
- `git worktree list --porcelain -z` parsing;
- local refs from `refs/heads` and remote server refs from `git ls-remote --heads origin` with full OIDs;
- every IssueOps ID is read; one unreadable row produces unknown instead of being skipped;
- bindings from every distinct record repo are loaded and deduplicated;
- no Orca binary plus no Orca-owned record is optional/healthy;
- no Orca binary plus an Orca-owned record is unknown;
- registered Orca state collects global terminals/tasks/gates/messages and repo worktrees;
- unresolved registered repo or any Orca list failure is unknown;
- duplicate IDs from any source remain visible for classifier rejection.

- [ ] **Step 3: Run the collector tests and verify RED**

Run:

```bash
go test ./internal/adapter/operationalhealth -count=1
```

Expected: FAIL before the collector exists.

- [ ] **Step 4: Implement exact read commands and normalization**

Use argv execution, never shell concatenation:

```text
git symbolic-ref --quiet --short HEAD
git rev-parse --verify HEAD
git status --porcelain=v1 -z
git worktree list --porcelain -z
git for-each-ref --format=%(refname)%00%(objectname)%00 refs/heads
git ls-remote --heads origin
```

Canonicalize repo paths with existing repo/path helpers. For every claimed task, call `ShowDispatch` and require exact task/dispatch identity. Parse persisted timestamps once into `time.Time`; invalid timestamps become inventory problems.

Load all IssueOps records, not only non-done rows, so done-owner residue and global Orca ownership are visible. Filter Git branch/worktree rules to the requested repo, but allow live cycles from another repo to own their exact global Orca task/terminal.

- [ ] **Step 5: Verify collector determinism and package boundaries**

Run:

```bash
go test ./internal/adapter/operationalhealth ./internal/core/operationalhealth -count=1
go test -race ./internal/adapter/operationalhealth ./internal/core/operationalhealth -count=1
go list -deps ./internal/core/operationalhealth | rg 'internal/adapter'
```

Expected: the first two commands pass and the last command prints nothing. A printed adapter dependency is a failure even though `rg` exits nonzero on the desired empty result.

- [ ] **Step 6: Commit the collector**

Commit subject:

```text
feat(doctor): collect operational inventory
```

---

### Task 5: Project operational health through the existing doctor only

**Files:**
- Modify: `internal/core/doctor/doctor_test.go`
- Modify: `internal/core/doctor/doctor.go`
- Modify: `internal/core/utility_facade.go`
- Modify: `cmd/harness/basiccli/dependencies.go`
- Modify: `cmd/harness/basiccli/inspect_doctor_cli.go`
- Modify: `cmd/harness/basiccli/inspect_doctor_cli_test.go`
- Modify: `cmd/harness/basiccli/test_helpers_test.go`
- Modify: `cmd/harness/harnessapp/cli_facade.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`

- [ ] **Step 1: Add doctor-core RED tests for projection and state-artifact de-duplication**

Add tests proving:

- an injected classifier finding adds one `operational_state` check and one matching doctor issue;
- an inventory problem makes `Healthy=false` with `operational_inventory_unknown`;
- unexpected state files/directories are projected as `operational_state_artifact_residue` when an operational snapshot is present, without duplicate `state_unexpected_file`/`state_unexpected_directory` issues;
- a nil operational snapshot preserves direct core callers' legacy behavior and does not pretend collection succeeded;
- doctor does not mutate Git, IssueOps, Orca, or state.

Extend `HarnessDoctorRequest` only with non-JSON execution inputs:

```go
OperationalSnapshot *operationalhealth.Snapshot `json:"-"`
OperationalOptions  operationalhealth.Options   `json:"-"`
```

Do not add a new top-level response field; project into existing `checks` and `issues`.

- [ ] **Step 2: Run the focused doctor tests and verify RED**

Run:

```bash
go test ./internal/core/doctor -run 'Operational|StateArtifact' -count=1
```

Expected: FAIL before projection exists.

- [ ] **Step 3: Implement core projection**

After the existing state doctor read, deep-copy the injected snapshot, copy its `unexpected_file` and `unexpected_directory` paths into that copy, classify with injected `Now`, and add:

```go
result.addCheck("operational_state", op.Healthy, summary)
```

Map finding codes verbatim, severity `warning` for concrete residue/dead owner and `error` for inventory unknown. Fix text directs inspection/reconciliation; it never contains an auto-delete command or sets `Destructive=true`.

- [ ] **Step 4: Add repeated CLI flags and a fake collector**

Parse:

```text
--preserve-cycle ID
--preserve-terminal HANDLE
```

Trim, reject empty entries, sort/deduplicate, and pass them only in `OperationalOptions`, with `Now` set once from `time.Now().UTC()` for the invocation. Add `CollectOperationalHealth func(context.Context, string) operationalhealth.Snapshot` to `basiccli.Deps`; configure the real adapter in `harnessapp` and deterministic fakes in CLI tests.

`status` remains an aggregator, not a second operational-health authority; do not add separate status semantics or flags. The top-level `doctor` is the only public command that always supplies the operational snapshot.

- [ ] **Step 5: Add CLI RED/PASS cases**

Test repeatable flags, blank rejection, collection-failure unknown, JSON/text code parity, no state writes, and help text. Run:

```bash
go test ./cmd/harness/basiccli -run 'TestRunDoctor.*(Operational|Preserve|Inventory)' -count=1
go test ./internal/core/doctor ./cmd/harness/basiccli -count=1
```

Expected: PASS.

- [ ] **Step 6: Update only the intended usage golden**

Run:

```bash
go test ./cmd/harness/contractgolden -run Golden -update -count=1
git diff -- cmd/harness/testdata/usage.golden.txt cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
```

Expected: doctor help gains the two flags; command list, MCP tool list, and response required-field schema do not gain a cleanup/reconcile surface.

- [ ] **Step 7: Commit doctor wiring**

Commit subject:

```text
feat(doctor): gate operational residue
```

---

### Task 6: Reuse cycle authority in stale scan without widening release eligibility

**Files:**
- Modify: `internal/core/issueops/issueops_stale_scan.go`
- Modify: `internal/core/issueops/issueops_stale_scan_apply_test.go`
- Add: `internal/core/issueops/issueops_stale_scan_operational_test.go`

- [ ] **Step 1: Add the policy-separation RED**

Create a claimed, exactly bound non-done record with missing/stale heartbeat but without existing strong releasable signals. Assert scan returns a `needs-review` finding whose reasons include `operational_dead_owner`, `Releasable == false`, and `Apply=true` leaves the record non-done. Keep an existing strong `confirmed-stale` fixture releasable.

- [ ] **Step 2: Run the focused stale tests and verify RED**

Run:

```bash
go test ./internal/core/issueops -run 'TestStaleScan.*Operational|TestStaleScan.*Heartbeat' -count=1
```

Expected: FAIL because stale scan has not consumed cycle authority.

- [ ] **Step 3: Wire only the shared authority result**

Normalize each record/binding into `operationalhealth.Cycle`, call `EvaluateCycleAuthority` with the same injected `Now`, and:

- append `operational_dead_owner` to an existing stale finding; or
- synthesize a report-only `needs-review` finding when the legacy classifier returns no row;
- never set `Releasable` from operational authority;
- retain the lock, fresh read, external probes, and legacy strong-signal reclassification immediately before any release.

Do not query Orca or duplicate the full cross-resource classifier in stale scan.

- [ ] **Step 4: Run all stale/IssueOps tests**

Run:

```bash
go test ./internal/core/issueops -run 'StaleScan|ForceRelease|Session' -count=1
go test -race ./internal/core/issueops -run 'StaleScan|ForceRelease|Session' -count=1
```

Expected: PASS; heartbeat-only evidence remains report-only.

- [ ] **Step 5: Commit stale-scan reuse**

Commit subject:

```text
fix(issueops): share dead-owner diagnosis
```

---

### Task 7: Gate the stability audit on top-level doctor and document operations

**Files:**
- Modify: `skills/stability-audit/scripts/e2e_stability_audit.py`
- Modify: `skills/stability-audit/scripts/test_e2e_stability_audit.py`
- Modify: `skills/stability-audit/SKILL.md`
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/ADR.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/CAUTIONS.md`

- [ ] **Step 1: Read `skills/stability-audit/SKILL.md` completely before editing**

This skill action is mandatory because the script and its operational contract change. Follow its narrower verification steps.

- [ ] **Step 2: Add Python RED tests for delegation**

Add tests for:

```python
def operational_doctor(report: dict[str, Any]) -> None:
    ...
```

Assert it invokes `bin/agent-harness doctor --repo /Users/m16khb/Workspace/agent-harness --json` and appends `--preserve-terminal` plus exact non-empty `ORCA_TERMINAL_HANDLE` when present. It passes only when exit is zero and parsed `ok` and `healthy` are true. Failure details retain only bounded issue codes/summaries.

Also assert the doctor call inherits the outer live harness environment, while ordinary/race Go regression subprocesses receive the exact audited source checkout as `HARNESS_ROOT` and their own temporary `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`, and `HARNESS_WORKER_DIR`. Simulate a regression write and prove the outer IssueOps DB/session projection is byte-for-byte unchanged.

- [ ] **Step 3: Run Python tests and verify RED**

Run:

```bash
python3 -m unittest skills/stability-audit/scripts/test_e2e_stability_audit.py
```

Expected: FAIL before the function exists.

- [ ] **Step 4: Implement and call the doctor gate immediately after build**

The stability script does not reimplement ownership or residue logic. Add one report step named `operational_doctor`; unknown/unhealthy fails the audit. Keep that step on the inherited live environment. For ordinary/race Go regression subprocesses, explicitly pin `HARNESS_ROOT` to the audited checkout and isolate the other three harness paths so the final gate cannot recreate the residue it just removed.

- [ ] **Step 5: Update narrow docs**

Document the invocation-only preserves, 15-minute diagnostic boundary, non-auto-release rule, Orca optionality, archival-only snapshot/reset race, stability delegation, recovery-bundle location, and forward-recovery rule. Add only the new core/adapter boundary and the accepted decision/roadmap state to `ARCHITECTURE.md` and `ADR.md`; do not duplicate the whole design.

- [ ] **Step 6: Run skill/script/document checks**

Run:

```bash
python3 -m unittest skills/stability-audit/scripts/test_e2e_stability_audit.py
python3 scripts/validate-skill.py skills/stability-audit
git diff --check
```

Expected: PASS.

- [ ] **Step 7: Commit audit/docs wiring**

Commit subject:

```text
test(operations): gate stability on doctor
```

---

### Task 8: Complete pre-cleanup source verification, review, and safe `main` publication

**Files:**
- Review all files changed in Tasks 0–7
- Add no product file unless a failing gate requires a directly scoped fix

- [ ] **Step 1: Format and check module hygiene**

Run:

```bash
gofmt -w internal/core/operationalhealth/*.go internal/adapter/operationalhealth/*.go internal/adapter/orca/client.go internal/adapter/orca/client_test.go internal/port/orca.go internal/core/doctor/doctor.go internal/core/doctor/doctor_test.go cmd/harness/basiccli/dependencies.go cmd/harness/basiccli/inspect_doctor_cli.go cmd/harness/basiccli/inspect_doctor_cli_test.go cmd/harness/basiccli/test_helpers_test.go cmd/harness/harnessapp/cli_facade.go internal/core/utility_facade.go internal/core/issueops/issueops_stale_scan.go internal/core/issueops/issueops_stale_scan_apply_test.go internal/core/issueops/issueops_stale_scan_operational_test.go
go mod tidy
git diff --check
```

Expected: only intended formatting; no unexplained dependency churn.

- [ ] **Step 2: Run the pre-cleanup gate to terminal exit**

Run:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness api-doc static-check --json
```

Expected: all PASS; the API-doc command reports pass or an explicit no-candidate skip. Do not run the newly gated live stability audit yet because the approved residue intentionally remains until Tasks 9–11.

- [ ] **Step 3: Review requirements and diff**

Verify no new root command/MCP tool, no persistent preservation, no raw task/message payload, fail-closed collection, no heartbeat auto-release, read-only doctor, and pure classifier. Run:

```bash
git status --short --branch
git diff origin/main...HEAD --stat
git diff origin/main...HEAD -- cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
```

- [ ] **Step 4: Fix only demonstrated findings and repeat Step 2**

Every fix gets focused RED/PASS evidence and an atomic commit.

- [ ] **Step 5: Publish `main` safely**

Use `atomic-commit-push`, push current `main` without force, then run:

```bash
git fetch --prune origin
git rev-parse HEAD
git rev-parse refs/remotes/origin/main
git status --short --branch
```

Expected: clean source and exact OID equality. If remote main advanced independently, stop; never force.

---

### Task 9: Build, test, and seal the external recovery bundle

**Files outside the repository:**
- Create: `/Users/m16khb/.local/state/agent-harness-backups/<repo-fingerprint>/<UTC-timestamp>/reconcile.py`
- Create: same directory `test_reconcile.py`
- Create through the runner: `manifest.json`, `manifest.sha256`, `journal.jsonl`, `git/`, `issueops/`, `state/`, `orca/`

The angle-bracket path components are generated values, not operator-entered deletion targets. The runner resolves the final absolute path once, rejects symlinks/non-owned parents, creates directories `0700`, writes files `0600`, and stores the resolved path in the sealed manifest. Later stages accept only that manifest path.

- [ ] **Step 1: Resolve and validate immutable execution identities**

Run independently:

```bash
pwd -P
git rev-parse --show-toplevel
git status --porcelain=v1
git rev-parse HEAD
git rev-parse refs/remotes/origin/main
printenv ORCA_TERMINAL_HANDLE
orca terminal list --limit 512 --json
```

Expected: exact canonical repo, clean source, equal main OIDs, non-empty environment handle, and exactly one matching terminal row.

- [ ] **Step 2: Create the external runner with explicit stages**

Implement only:

```python
def collect_manifest(repo_root: Path, current_terminal: str, bundle_root: Path) -> dict: ...
def stable_projection(manifest: dict) -> dict: ...
def append_journal(bundle_root: Path, row: dict) -> None: ...
def validate_bundle(bundle_root: Path) -> None: ...
def apply_operation(bundle_root: Path, operation_id: str) -> None: ...
def verify_final(bundle_root: Path) -> None: ...
```

Journal appends and fsyncs the file and parent directory. Each operation advances only `planned -> started -> completed -> verified`; recovery inspects current state before continuing. Orca reset is never automatically retried after an ambiguous invocation.

- [ ] **Step 3: Test the runner against synthetic resources**

Test stable digest volatility, identity drift, duplicate/symlink rejection, journal order/fsync, started recovery, planned-exception and started reset recovery against Git-only worktree drift, locked force-release CAS proof, hostile inherited environment overrides, bundle-executor tampering, singleton/equal fetch-push authority, exact sealed fetch/prune argv, operation-order phase projections across every Orca/Git/IssueOps/state inventory, pre-stop new-task rejection, post-reset new-owner rejection before ref deletion, blank/mismatched environment-terminal rejection before observation, reset ambiguity, and redaction of task/message content.

Run:

```bash
python3 "$BUNDLE_ROOT/test_reconcile.py"
```

Expected: PASS before live observation.

After the implementation commit is clean and equals `refs/remotes/origin/main`, run `simulate_copy.py`. It builds its own `0700` `-trimpath` executor, verifies SHA-256 plus VCS revision and `modified=false`, then exercises the exact binding deletion, every force-release CAS, and the approved test-session transaction against an online backup of the live SQLite database. It must never invoke the ignored repository `bin/agent-harness`.

- [ ] **Step 4: Collect the live manifest and backups without deletion**

The runner:

- requires an explicit-URL, heads-only, `--no-tags --no-write-fetch-head` fetch/prune dry-run to be empty, records direct server ref OIDs, and proves each remote OID is already present in the local object database without mutating the source checkout;
- records every Git worktree path/branch/HEAD/clean state;
- creates/verifies `git/all-refs.bundle`;
- uses `sqlite3.Connection.backup()` and verifies backup `PRAGMA integrity_check == ok`;
- pins exact raw-byte SHA-256 and key-sorted compact UTF-8 canonical SHA-256 (without HTML escaping) for every IssueOps/session row;
- writes redacted Orca projections and `limitations.json`;
- copies/hashes unexpected state artifacts into `state/backup/`;
- requires the caller terminal argument to equal a non-empty valid `ORCA_TERMINAL_HANDLE` before observation, then pins that exact current terminal and all approved targets; every later live-validation/apply/final CLI entry rechecks the same environment-to-seal equality before mutation or readback;
- builds a private executor from the clean sealed HEAD and pins its SHA-256 plus Go VCS revision alongside runner/test script hashes.

- [ ] **Step 5: Seal and independently validate**

Run:

```bash
python3 "$BUNDLE_ROOT/reconcile.py" validate --bundle "$BUNDLE_ROOT"
git bundle verify "$BUNDLE_ROOT/git/all-refs.bundle"
sqlite3 "$BUNDLE_ROOT/issueops/harness.sqlite" 'PRAGMA integrity_check;'
shasum -a 256 "$BUNDLE_ROOT/manifest.json"
```

Expected: all pass; manifest hash matches, modes are `0700`/`0600`, and no raw task result/spec or message body/payload/subject is retained. Stop if a resource appears during sealing.

---

### Task 10: Quiesce Orca and remove exact terminal/worktree/orchestration residue

**Files:**
- Append only: sealed bundle `journal.jsonl`
- No repository source edits

- [ ] **Step 1: Revalidate manifest identity immediately before mutation**

Run `reconcile.py validate-live --bundle "$BUNDLE_ROOT"`. Compare source HEAD/clean state, target identities, record digests, server OIDs, runtime ID, and current terminal. Any drift stops.

- [ ] **Step 2: Close every non-current terminal by exact handle**

Invoke `orca terminal close --terminal EXACT_HANDLE --json` per manifest target. After each call, verify target absence and the exact preserved handle's unique presence, then journal `verified`.

- [ ] **Step 3: Stop the active coordinator run once**

Before invoking anything, require the exact journal-derived Orca/Git/IssueOps/state phase projection; a new task/terminal/dispatch/worktree or any other unsealed resource prevents the stop call. Then invoke `orca orchestration run-stop --json`, journal bounded result, and verify the same exact post-operation projection. On ambiguity, stop for readback instead of repeating.

- [ ] **Step 4: Remove each non-main Orca worktree by exact ID**

Invoke `orca worktree rm --worktree id:EXACT_WORKTREE_ID --force --json`. Verify absence in Orca and Git worktree inventories. Preserve source/main and current terminal's worktree.

- [ ] **Step 5: Require two identical quiescent full digests**

Two consecutive stable projections must equal each other and the manifest's expected post-quiescence projection. Any difference stops before reset.

- [ ] **Step 6: Reset orchestration once and verify zero**

Invoke `orca orchestration reset --all --json` exactly once. Verify all-task `0`, dispatched `0`, gate `0`, inbox count/rows `0`, the exact current terminal, and exact singleton canonical Git/Orca source worktrees. Every later fence requires that same exact projection. Ambiguous reset is read back and never auto-retried.

---

### Task 11: Release sealed IssueOps records, remove bindings/refs, and relocate state artifacts

**Files:**
- Append only: sealed bundle `journal.jsonl`
- Move only manifest-pinned state artifacts into bundle `state/relocated/`
- No repository source edits

- [ ] **Step 1: Force-release every pinned non-done record after digest CAS**

Delete only the manifest-pinned canonical-repository bindings with the exact SQLite transaction CAS. For each record, pass its sealed raw and canonical digests to the bundle-private clean-HEAD executor; the core re-reads both digests and proves zero repository bindings under the same state-root span lock before mutating:

```text
BUNDLE_ROOT/bin/agent-harness issueops force-release --id EXACT_ID --reason operational-reconciliation-2026-07-19 --expected-raw-sha256 RAW_SHA256 --expected-canonical-sha256 CANONICAL_SHA256 --json
```

Pin `HARNESS_STATE_DIR`, `HARNESS_ROOT`, `HARNESS_DAEMON_DIR`, and `HARNESS_WORKER_DIR` for every live executor call. Journal the locked before/after digests, binding counts, executor digest, `phase=done`, exact reason, timestamp, and expected cleanup/orphan stamps separately. New/drifted records or bindings stop the stage.

Before and after every post-reset operation, recompute the journal-order phase projection. Sealed resources may be absent only after their own verified removal; all record IDs, session keys, other SQLite rows, local/remote refs and OIDs, and state artifact paths/hashes must otherwise remain exact. This fence runs before branch/ref/state mutation, so a newly created IssueOps owner cannot be discovered only after its branch was deleted.

- [ ] **Step 2: Delete only the approved stale test-session set by exact transaction**

Require every manifest-pinned test-session row to be present with its exact raw/canonical digest, delete the whole approved set under the shared SQLite span lock, and prove all are absent with unrelated rows unchanged. Partial presence, a new canonical-repository binding, or any digest drift stops without broad stale cleanup or done-record pruning.

- [ ] **Step 3: Delete non-canonical local refs with expected-old OIDs**

For each sealed target invoke only `git update-ref -d refs/heads/EXACT_BRANCH EXPECTED_FULL_OID`, verify absence, and never target `refs/heads/main`.

- [ ] **Step 4: Delete non-canonical remote refs with leases**

Re-read each server OID through the sealed explicit URL, require manifest equality, then invoke only `git push --force-with-lease=refs/heads/EXACT_BRANCH:EXPECTED_FULL_OID SEALED_PUSH_URL :refs/heads/EXACT_BRANCH`. Collection and every fence require exactly one fetch URL and one push URL with the same credential-free canonical authority. Verify server absence and never target `main`.

- [ ] **Step 5: Relocate each state artifact after double hash verification**

Verify original identity/hash and backup hash, require empty destination, atomically rename when possible or copy+fsync+hash+unlink exact original, then prove original absence and both bundle hashes. Never whitelist artifacts.

- [ ] **Step 6: Prune registrations and run final runner invariant check**

Run `git worktree prune`, then fetch/prune only `+refs/heads/*:refs/remotes/origin/*` from `SEALED_FETCH_URL` with `--no-tags --no-write-fetch-head`, then run `reconcile.py verify-final`. The journaled mutation and its dry-run readback must use the exact same sealed URL/refspec; require no new unaccounted resource.

---

### Task 12: Run the final all-or-nothing verification window and seal evidence

**Files:**
- Append only: sealed bundle `journal.jsonl`
- Write: sealed bundle `final-verification.json`
- Update source only for a directly reproduced defect; if that occurs, re-push main and repeat Tasks 9–12 with a new bundle

- [ ] **Step 1: Prove final external inventories from scratch**

Run standalone:

```bash
git status --short --branch
git worktree list --porcelain
git for-each-ref --format='%(refname) %(objectname)' refs/heads
git ls-remote --heads origin
orca status --json
orca worktree list --repo path:/Users/m16khb/Workspace/agent-harness --limit 512 --json
orca terminal list --limit 512 --json
orca orchestration task-list --brief --json
orca orchestration task-list --status dispatched --json
orca orchestration gate-list --json
orca orchestration inbox --limit 1 --json
./bin/agent-harness issueops cleanup stale --repo /Users/m16khb/Workspace/agent-harness --prune-done 0s --json
./bin/agent-harness state doctor --json
```

Expected: one source/main Git and Orca worktree; only local/remote main; only exact current terminal; task/dispatched/gate/message all zero; all pinned records done, non-done zero, bindings zero; no unexpected state artifact.

- [ ] **Step 2: Run top-level doctor with exact preserved terminal**

Run:

```bash
./bin/agent-harness doctor --repo /Users/m16khb/Workspace/agent-harness --preserve-terminal "$ORCA_TERMINAL_HANDLE" --json
```

Expected: `ok=true`, `healthy=true`, healthy `operational_state`, and no `operational_*` issue.

- [ ] **Step 3: Run complete code verification to terminal exit**

Run:

```bash
go mod tidy
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

Expected: every exit `0`; both self-verifications are `ok=true`, score at least `95`, termination eligible.

- [ ] **Step 4: Run the full stability audit with live doctor gate**

Run:

```bash
python3 skills/stability-audit/scripts/e2e_stability_audit.py --cleanup-stale --json
```

Expected: `ok=true`, `operational_doctor.ok=true`, no stale daemon/socket/zombie/orphan or leaked test process.

- [ ] **Step 5: Recheck Git publication and cleanup invariants in the same window**

Run:

```bash
git status --short --branch
git rev-parse HEAD
git rev-parse refs/remotes/origin/main
git ls-remote --heads origin
```

Expected: clean main, equal local/remote-tracking OIDs, only server `refs/heads/main`, and no verification-created residue.

- [ ] **Step 6: Seal final evidence**

Write command argv, exit codes, stable output digests, observed counts, final OID, doctor result, self-verify scores, stability result, and bundle/journal digest to `final-verification.json`. Append `reconciliation/verified`, fsync, and revalidate the bundle.

- [ ] **Step 7: Report completion with direct evidence**

Cite source file/lines, final matching OID, absolute bundle path and hashes, before/after counts, verification exits/scores, and the approved Orca archival-only/reset-race limitation. Do not claim completion if any item is inferred, skipped, or from a different verification window.
