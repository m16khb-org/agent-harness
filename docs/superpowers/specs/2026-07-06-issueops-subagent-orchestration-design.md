# IssueOps Sub-Agent Orchestration Design

## Problem

IssueOps coordinates exactly one agent working one cycle. Two real workflows exceed that model:

1. **Hierarchical delegation (S1).** One umbrella issue with N provider-native child tasks. The main agent runs IssueOps in the umbrella issue's worktree; each child task should run as a *sub-agent* in its *own* isolated worktree, with its own IssueOps discipline, PRing back into the parent work branch. Today children exist only as flat remote links (`IssueOpsIssueLink{Type:"child"}`, `internal/core/issueops/model/types.go:19-28`) — there is no local cycle-to-cycle hierarchy, no per-child state, and no parent gate that says "all delegated work is verified and merged".
2. **Worker-pool refactoring (S2).** For a large mechanical refactor, the main agent selects candidate work items, then keeps a bounded pool of sub-agent workers busy — dispatching items as slots free up, keeping load below the pool size — while the main agent validates each result and integrates it. Today the closest primitive is `internal/core/worker`, which is a no-shell lifecycle record store with no queue semantics (no claim, no lease, no retry: `worker/worker.go`, "worker MVP records lifecycle state only").

Cross-cutting gaps found by direct audit (`.agent-harness/ISSUEOPS_AUDIT.md`, reconciled 2026-07-01):

- **Session binding is one-per-repo** (`internal/core/issueops/session/session.go:1-12`: "Each repo may have at most one active binding"). With a parent + N child cycles concurrently active in the same repo, N+1 sessions cannot each resume their own cycle through the binding.
- `issueops resume` resolves only via the per-repo binding or branch heuristics (`package.go:487-545`); a sub-agent cannot resume a specific cycle by ID.
- No aggregation readiness: nothing stops a parent cycle from reaching `pr`/`done` while delegated children are unfinished.
- Concurrency and quality must be first-class (explicit user requirement): every new shared-state surface needs the same flock discipline, lost-update tests, and `-race` coverage the existing core has (`TestIssueOpsConcurrentFeedbackNoLostUpdate` precedent).

## Goal

Add a **delegation graph** (parent/child IssueOps cycles) and a **work pool** (bounded lease-based task queue) to agent-harness as durable, fail-closed, host-neutral coordination state — integrated with the existing phase machine, phase ledger, hooks, and skills — so that the main agent can orchestrate sub-agents across worktrees while the harness guarantees:

- no lost updates or double-claims under concurrent multi-session access (verified with `-race` and adversarial concurrency tests),
- fail-closed parent completion (a parent cannot pass `pr` readiness while children are incomplete or unvalidated),
- resumability of every participant (parent, each child, each pool worker) after interruption,
- bounded load (pool never exceeds its configured size; leases expire and requeue).

## Non-Goals

- **The harness never spawns, supervises, or kills agent processes.** Verified: no `claude -p`/agent invocation exists anywhere in the codebase today (only a bounded read-only `codex exec` review helper in `cmd/harness/apidoc/api_doc_review_runner.go:24`). Sub-agents are spawned by the *main agent* through host-native mechanisms (Claude Code Agent tool/agent teams, Codex, GJC). The harness provides state, gates, and guidance only. This preserves the ARCHITECTURE.md worker principle ("local job worker는 workspace 경계, command policy, secret redaction, audit log가 준비된 뒤 도입한다") and keeps host adapters thin.
- Hooks do not perform workflow work. They observe and relay deterministic facts (children/pool summaries) and keep the existing narrow blocks; they never create cycles, claim tasks, or record heartbeats.
- No changes to the 9-phase list (`model/phase.go:5-15`) and no phase-machine fork for children. Children traverse the *same* machine; delegation is expressed as auto-recorded, parent-referencing artifacts, not as a second state machine.
- No provider work-item automation beyond what exists (`cleanup close-children`, `remote create-child` stay as-is).
- No Windows cross-process locking work (deferred item 1.2 of the audit stays deferred).
- Pool tasks are not IssueOps cycles. A pool task that grows into real design work should be promoted manually to a child cycle; automatic promotion is out of scope.

## Current Evidence

- Phase machine and gates: `AdvanceIssueOpsPhase` → `validateIssueOpsPhaseTransition` (`internal/core/issueops/issueops_phase.go:24-122`) — rank-monotonic, fail-closed per-target readiness, `done` requires prior `pr` + verified remote artifact.
- Per-cycle serialization: `withIssueOpsLock` flock on `<stateRoot>/<id>.lock`, persistent inode, never deleted (`issueops_lock_unix.go:23-42`); cycle id = `io-<sha256(repo\0branch)[:12]>` (`issueops_state.go:43-46`); `StartIssueOps` locks with abs-normalized repo (`package.go:209-226`).
- Session binding: one per repo, `session.Binding{CycleID,Repo,Branch,ExpectedWorktree,BoundAt}`, per-repo flock (`session/session.go`), bind on `LinkIssueOpsWorktree`, cycle-guarded unbind on done/force paths.
- Heartbeat/liveness: `RecordIssueOpsHeartbeat`, `IssueOpsLastActiveAt` (heartbeat-preferred) feeding the stale scan (`issueops_heartbeat.go`, `stalescan/stalescan.go:60-155`).
- Phase ledger: additive completion index with entry-vs-completion discipline and stale marking on regress (`issueops_phase_ledger.go`; design: `docs/superpowers/specs/2026-06-29-issueops-phase-ledger-design.md`).
- Execution decision gate already validates sub-agent plans against the 12 SUB_AGENT_PATTERNS slugs (`IssueOpsSubAgentPlan{Pattern,Benefit,Tradeoffs,NetPositiveRationale}`, `model/types.go:187-206`); the relevant slugs here are `task-fan-out-coordination`, `isolated-worktree-work`, `background-long-running-work`.
- Worker MVP: `WorkerJob` one-file-per-job store with per-job flock and stuck-PID detection but no claim/lease/queue loop (`internal/core/worker/`).
- Hook surface: `agent-harness hook <event>`; PreToolUse worktree guard resolves expected worktree from env/flag only in hookcli (deliberate, to avoid blocking unrelated same-repo work), lifecycle MCP guard falls back env → branch-matched session binding → active cycles; UserPromptSubmit injects `activeWorktreeReminderValue` (`internal/core/hookprompt/worktree_reminder.go:8-29`); Stop relays next-action facts.

## Design Overview

Three additive state features, one binding extension, hook/skill surfacing, and a verification battery:

| # | Feature | New state | Owner commands |
|---|---------|-----------|----------------|
| D1 | Delegation graph (parent/child cycles) | `ParentCycleID`, `Delegation`, `ChildCycles` on `IssueOpsRecord` | `issueops child start/status/accept/list` |
| D2 | Scoped session bindings | per-cycle binding files | `issueops resume --id`, `issueops bind --id` |
| D3 | Work pool | `internal/core/workpool` (`<state>/workpool/`) | `workpool create/add-task/claim/heartbeat/submit/accept/reject/reap/status/close` |
| D4 | Hook surfacing | none (read-only) | UserPromptSubmit hint + Stop relay additions |
| D5 | Skill/doc integration | none | `skills/issueops` sections + references, SUB_AGENT_PATTERNS, AGENT_WORKFLOW, ADR |
| D6 | Upstream independence | none (removals) | gate/install/update/review/judge decoupling from upstream tools & services |

### Actor model

- **Main agent (parent session):** owns candidate selection, delegation decisions (recorded via the existing execution-decision gate), validation/acceptance of results, integration (merge), and all safety judgments. Spawns sub-agents host-natively.
- **Sub-agent (child/worker session):** receives a *delegation prompt* (rendered by the harness — cycle/task id, worktree path, `export HARNESS_EXPECTED_WORKTREE=...` line, owner-command contract), works only inside its worktree, records evidence through the same CLI/MCP surface, and reports by state (child cycle phase / task submit), not by side channel.
- **Harness:** durable state, fail-closed gates, lease bookkeeping, deterministic hook reminders.

## D1: Delegation Graph (S1)

### State shape

At this design's July 6 baseline, these non-ownership fields were additive `omitempty` under IssueOps schema v1, following the `phase_ledger` precedent. Issue #16 later supersedes that compatibility decision with root schema v4 for `execution_handoff`, stable terminal identity, and sealed completion authority:

```go
// Child-side: identifies the parent and carries the delegated work contract.
Delegation *IssueOpsDelegationContract `json:"delegation,omitempty"`

// Parent-side: index of delegated child cycles. The child record is
// authoritative for child phase/liveness; this index is authoritative ONLY for
// parent-owned validation verdicts. Read paths reconcile (read-repair) phase
// data from child records.
ChildCycles []IssueOpsChildCycleRef `json:"child_cycles,omitempty"`
```

```go
type IssueOpsDelegationContract struct {
	ParentCycleID      string   `json:"parent_cycle_id"`
	TaskScope          string   `json:"task_scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ParentPlanPath     string   `json:"parent_plan_path,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	DelegatedAt        string   `json:"delegated_at"`
}

type IssueOpsChildCycleRef struct {
	CycleID            string   `json:"cycle_id"`
	Branch             string   `json:"branch"`
	Title              string   `json:"title,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	CreatedAt          string   `json:"created_at"`
	// Parent-owned validation verdict (main agent judgement), recorded by
	// `issueops child accept/reject/drop`. accepted and dropped unblock the
	// parent gate; rejected keeps it blocked pending redo.
	ValidationVerdict  string   `json:"validation_verdict,omitempty"` // accepted | rejected | dropped
	ValidationEvidence []string `json:"validation_evidence,omitempty"`
	ValidatedAt        string   `json:"validated_at,omitempty"`
}
```

A child cycle is a **full IssueOpsRecord** with its own id derived from `(repo, child-branch)` by the existing `newIssueOpsID` — child branches are distinct, so ids are distinct and the existing per-id flock serializes each child independently.

### Child start (fail-closed)

`issueops child start --parent <id> --branch <child-branch> --title ... --scope ... --acceptance ... [--child-issue-url ...] --json` / MCP `issueops_child_start`:

1. Read parent under parent lock; **refuse** unless ALL hold (fail-closed):
   - parent `Phase == implement` (delegation is an implement-phase execution move),
   - parent `DesignReview.Approved`, `CompatibilityReview.Approved` with no blockers, `DevilsAdvocateReview` pass-or-waived (same conditions as `IssueOpsImplementationReadiness`),
   - parent `ExecutionDecision.SubagentPlans` contains a plan whose `Pattern` is one of `task-fan-out-coordination`, `isolated-worktree-work`, `background-long-running-work` (delegation without a recorded sub-agent decision is rejected; missing key `execution_decision_subagent_plan`),
   - `--branch` differs from the parent branch and passes `branchprepare.ValidateBranch` when a child issue URL is provided.
2. Create the child record via the existing `StartIssueOps` path (its abs-normalized start lock), then under the *child* lock stamp `Delegation` and auto-record delegated artifacts (below). Locks are strictly sequential — **never hold two cycle locks at once** (lock-ordering invariant, see Concurrency).
3. Under the *parent* lock, append the `IssueOpsChildCycleRef` (dedupe by CycleID). If this step fails after the child was created, the child's `Delegation.ParentCycleID` still points at the parent; `issueops child list/status` performs read-repair by scanning child records (authoritative direction: child → parent pointer wins).
4. If `--child-issue-url` is provided, also record the existing remote link (`LinkIssueOpsChild`) so the provider graph and local graph stay associated.

### Delegated artifact profile (no phase-machine fork)

Children traverse the same 9 phases, but `child start` auto-records the analysis artifacts the parent already produced, so a child begins **plan-entry-ready** (it can enter `plan` immediately) without re-grilling:

- `Intent` — derived from the delegation contract (`RawRequest` = parent intent + task scope; `InterpretedIntent` = the task scope statement — required by `issueOpsIntentMissing`; `SuccessCriteria` = acceptance criteria; `IntentClass = "delegated-child"`).
- `DomainReview` — reference entry (`ModelFit = "delegated: inherits parent <parent-id> domain review"`).
- `PlanPrep` — all three items `waived` with reason `delegated:<parent-id>` (uses the existing waive mechanics, `readiness.go:60-69`).
- `split_decision` — a `scope` decision "delegated child of <parent-id>" (satisfies `issueOpsSplitDecisionMissing`).
- `IssueURL` — the child issue URL when given, else the parent issue URL (umbrella).
- `DesignReview` / `CompatibilityReview` — delegated reference reviews with `Approved=true` AND every subfield the approved-state readiness demands populated from the parent's artifacts (`RefactorPlan`, `Alternatives`, `Risks`, `Verification` carrying design-review evidence — `issueOpsDesignReviewMissing` requires `refactor_plan`/`alternatives`/`risks`/`design_review_evidence` when `Approved`, `readiness.go:248-267`). Recorded only because the parent's are verified-approved in step 1.
- `DevilsAdvocateReview` — `Waived=true` with `WaiverRationale = "delegated:<parent-id> parent DA verdict pass"` (waiver path exists in `issueOpsDevilsAdvocateMissing`, `readiness.go:140-149`). A child whose scope drifts beyond the contract must NOT be force-fit — the sub-agent stops and reports; the parent either resolves children and re-plans or records a fresh child-level review.

**What the child must still EARN itself (fail-closed, unchanged gates):** entering `compatibility-review` requires `branch_prepare` + `plan_path` (`issueOpsBaseImplementationMissing`) and `worktree_path`/`worktree_exists`/`plan_in_worktree` (`IssueOpsCompatibilityReviewReadiness`, `readiness.go:83-105`) — i.e. the child prepares its OWN provider-linked branch, isolated worktree, and in-worktree plan during its `plan` phase. Entering `implement` additionally requires the child's own `worktree_tools` and `execution_decision`. This is deliberate: worktree isolation is exactly the property delegation exists to guarantee, so it is never inherited. The delegated reference reviews make the review-content gates instant once those setup artifacts exist, nothing more.

The child's `BranchPrepare.BaseBranch` is the **parent work branch** — so the existing `target_branch_match` strict-PR check proves each child PR targets the parent branch, and `cleanup close-children` (Merged evidence) keeps working unchanged.

### Parent aggregation gate

New completion inputs, fail-closed at the parent's `pr` entry (extends `IssueOpsStrictPRReadiness`; the phase ledger indexes them in the `pr` artifact set alongside `strict_pr_readiness` — NOT on `implement`, whose completion helper has no child check and would otherwise derive-complete falsely):

- `child_incomplete:<cycle-id>` — a delegated child that is non-terminal. **Non-terminal is defined as `(phase != done OR ForceReleasedAt != "") AND verdict != dropped`** — a force-released child has `Phase=done` stamped by the force path, so the `ForceReleasedAt` marker keeps it incomplete until the parent records `dropped` (or the child is legitimately re-driven to a verified `done`).
- `child_unvalidated:<cycle-id>` — a `done` child without a parent-recorded terminal verdict.
- `child_rejected_unresolved:<cycle-id>` — a child whose latest verdict is `rejected`; the gate stays blocked until the child is re-driven to `done` and re-validated `accepted`, or the parent records `dropped`.

Verdict model (parent-owned, terminal-vs-blocking explicit):

| Verdict | Meaning | Parent gate effect |
|---------|---------|--------------------|
| `accepted` | validated with evidence (≥1 entry) | unblocks that child |
| `rejected` | redo expected; reason ≥ 10 chars | keeps blocking (`child_rejected_unresolved`) |
| `dropped` | work deliberately abandoned; reason ≥ 10 chars | unblocks with audit trail |

Owner commands: `issueops child status --parent <id> --json` (aggregated table: child id, phase, heartbeat age, worktree, verdict; read-repairs the index) and `issueops child accept|reject|drop --parent <id> --child <cycle-id> [--evidence ...|--reason ...] --json` (parent lock; a rejected child's redo is driven by a new delegation prompt, never by mutating the child record from the parent).

A parent with zero `ChildCycles` is unaffected (gate vacuously satisfied) — fully backward compatible.

### Parent regress with active children (fail-closed)

`RegressIssueOpsForReplan` invalidates the parent plan the children were delegated from. A NEW guard, evaluated **after** the existing preconditions (phase within plan..compatibility-review, DA `stop` reflected, regress cap — `issueops_regress.go:44-61`), refuses regress with reason `children_active` while any delegated child is non-terminal. The parent must first resolve every child (accept, drop, or let it finish) before regressing.

Reachability note (verified against the current gates): a parent normally delegates only from `implement`, where regress is ALREADY refused by the existing phase precondition — that refusal is pinned by a test in delegation context. The `children_active` guard becomes reachable through the stale-reset corner: a worktree-deleted parent is reset to `problem` **with `ChildCycles` preserved** (see Lifecycle interactions), re-advances to `plan`, records a DA `stop` — and must still not regress past its live children. This prevents silently orphaning in-flight sub-agents against a plan that no longer exists. Owner command: `issueops child status --parent <id>`.

### Lifecycle interactions (stale reset, force paths)

- **Stale reset preserves the delegation graph.** `resumeOrReset` (`start/start.go:80-133`) rebuilds a reset record from an explicit field allow-list; `Delegation` and `ChildCycles` MUST be added to that allow-list (alongside `IssueURL`/`IssueLinks`/`Decisions` it already preserves). Otherwise a worktree-deleted CHILD would lose `Delegation.ParentCycleID` (orphaned from the read-repair scan) and a reset PARENT would lose `ChildCycles` including validation verdicts. A reset child re-enters at `problem` (not `done`), so it correctly remains `child_incomplete` on the parent.
- **Parent force-done/force-release with active children is allowed but audited, never silent.** Force paths set `Phase=done` directly (`issueops_force_release.go:38`, `issueops_force_done.go`), bypassing strict PR readiness — they are deliberate human escape hatches and refusing them could deadlock recovery. Instead: the force result and the parent record's force reason capture the list of then-active child cycle ids, and `issueops child status` (read-repair) marks children whose parent is `done` as `parent_closed` so their sub-agent sessions surface the orphan state. The Stop-hook relay names active children when a bound parent is force-closed.
- **Child-link partial success.** `child start` step 4 (`LinkIssueOpsChild` for `--child-issue-url`) can fail after the child cycle and parent ref were durably created — e.g. `remote.ValidateChildMatchesParent` requires the child issue to live in the SAME provider project as the umbrella issue (cross-project child issues are unsupported). The command then returns the created child with a `child_link_warning` instead of failing the whole operation; the remote link can be retried with the existing `issueops link-child`.
- **Pools cannot permanently deadlock a parent.** `pool_incomplete` is computed from live pool manifests; an abandoned pool is always resolvable by `workpool close --force --reason ...` (audited), so the parent gate has a human escape hatch with a trail.

### Depth limit

`child start` refuses when the parent itself has a non-empty `Delegation` (max depth 1). Sub-agent nesting is a documented net-negative (SUB_AGENT_PATTERNS forbidden table); missing key `delegation_depth_exceeded`.

## D2: Scoped Session Bindings

Problem: `session.Bind` overwrites the single per-repo binding — parent binds, then child 1 binds, parent's binding is gone.

Design (backward compatible):

- Keep the legacy per-repo binding file as the **primary** binding (main agent's cycle).
- Add per-cycle **scoped bindings**: `issueops-session-<repoHash>-<cycleID>.json` (cycleID is already filesystem-safe `io-[0-9a-f]{12}`). Written by `LinkIssueOpsWorktree` for delegated children (a cycle with `Delegation != nil` writes a scoped binding INSTEAD of overwriting the primary) and by explicit `issueops bind --id <cycle> --json`. Removed cycle-guarded on done/force paths (same compare-and-delete discipline, under the same per-repo session flock — one flock for all binding files of a repo keeps bind/unbind linearized and avoids a lock-file-per-cycle sprawl).
- `issueops resume` gains `--id <cycle>`: resolves that cycle directly, returns the same `IssueOpsResumeResult` (+ `export HARNESS_EXPECTED_WORKTREE=` guidance), and with `--bind` writes the scoped (child) or primary (non-delegated) binding.
- Guard resolution order (lifecycle MCP guard only; hookcli PreToolUse stays env/flag-only by design): env → **branch-matched scoped binding** → branch-matched primary binding → active cycles. Each sub-agent session runs with its own `HARNESS_EXPECTED_WORKTREE` export (per-process env, no cross-talk), so the strong guard works per-worker today; scoped bindings restore context after compaction/restart.

## D3: Work Pool (S2)

New package `internal/core/workpool` + state namespace `<state.StateDir()>/workpool/`.

### State shape

```go
const WorkPoolCurrentSchemaVersion = 1

type WorkPool struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`   // wp-<sha256(repo\0name)[:12]>
	Repo          string `json:"repo"`
	Name          string `json:"name"`
	ParentCycleID string `json:"parent_cycle_id,omitempty"` // umbrella IssueOps cycle
	Size          int    `json:"size"`            // max concurrent leases, 1..16
	LeaseTTL      string `json:"lease_ttl"`       // Go duration, default "15m"
	MaxAttempts   int    `json:"max_attempts"`    // default 2
	Status        string `json:"status"`          // active | draining | closed
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type WorkTask struct {
	OK                 bool     `json:"ok"`
	SchemaVersion      int      `json:"schema_version"`
	ID                 string   `json:"id"`      // task-<seq>-<sha256(title)[:8]>
	PoolID             string   `json:"pool_id"`
	Title              string   `json:"title"`
	Instructions       string   `json:"instructions"`        // redacted on write
	Scope              []string `json:"scope,omitempty"`      // target paths/globs
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Status             string   `json:"status"` // pending | leased | submitted | accepted | rejected | dropped
	WorkerID           string   `json:"worker_id,omitempty"`
	LeaseExpiresAt     string   `json:"lease_expires_at,omitempty"`
	LastHeartbeatAt    string   `json:"last_heartbeat_at,omitempty"`
	Attempts           int      `json:"attempts"`
	Branch             string   `json:"branch,omitempty"`
	WorktreePath       string   `json:"worktree_path,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`   // worker-submitted verification evidence
	SubmittedAt        string   `json:"submitted_at,omitempty"`
	RejectReason       string   `json:"reject_reason,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}
```

Layout: `<state>/workpool/<pool-id>.json` (manifest), `<state>/workpool/<pool-id>/<task-id>.json` (one file per task), `.lock` siblings following the persistent-inode flock pattern verbatim (`issueops_lock_unix.go` discipline; non-unix in-process fallback mirrors the existing ones).

### Task lifecycle

```
pending ──claim──▶ leased ──submit──▶ submitted ──accept──▶ accepted (terminal)
   ▲                 │                     │
   │            lease expiry          reject (attempts < max)
   │            (reap)                     │
   └──────attempts++ ──────────────────────┘
                └── attempts ≥ max ──▶ dropped (terminal)
```

- **claim** (`workpool claim --pool <id> --worker <worker-id> --json`): under the **pool lock** (single short critical section): reap expired leases (below), count `leased` tasks; if count ≥ `Size` → refuse with `pool_saturated` (this is the load-control contract); else pick the oldest `pending` task, stamp `leased/WorkerID/LeaseExpiresAt = now+TTL`, and return the task + a rendered delegation prompt (recommended branch name `pool/<pool-name>/<task-id>`, worktree guidance, `export HARNESS_EXPECTED_WORKTREE=` once the worker prepares it, owner-command contract: heartbeat/submit). Claims are pool-lock-serialized; task mutations after claim use the per-task lock only.
- **heartbeat** (`workpool heartbeat --pool --task --worker --json`): per-task lock; refuse if `WorkerID` mismatch or lease expired (worker learns it lost the lease and must stop); extends `LeaseExpiresAt = now+TTL`.
- **submit** (`workpool submit --pool --task --worker --evidence ... --branch --worktree --json`): per-task lock; requires live lease + worker match + ≥1 evidence entry; → `submitted`.
- **accept / reject** (`workpool accept|reject ...` — main-agent commands): per-task lock; `accept` requires ≥1 validation evidence entry recorded with the verdict; `reject --reason` (≥10 chars) requeues (`pending`, attempts++) or drops at `MaxAttempts`.
- **reap** (`workpool reap --pool --json`, also run inline at the start of every claim/status under the pool lock): `leased` tasks whose lease is expired → `pending`, attempts++ (→ `dropped` at cap). **Expiry boundary is defined as `now >= LeaseExpiresAt`** (the expiry instant itself is expired) and is pinned by boundary tests on both sides. Reaping is timestamp-based (sub-agents are not PIDs the harness tracks); TTL + heartbeat is the liveness contract.
- **close / draining**: `draining` refuses new claims but allows submit/accept; `close` requires all tasks terminal or `--force` with reason; a pool with zero tasks closes trivially; a `closed` pool refuses ALL task mutations (claim/heartbeat/submit/accept/reject).

### Pool ↔ IssueOps integration

- `workpool create` with `--parent-cycle <id>` requires the same parent preconditions as `child start` step 1 (implement phase, approved reviews, recorded sub-agent plan with slug `task-fan-out-coordination`).
- Parent strict-PR readiness gains `pool_incomplete:<pool-id>` while a linked pool has non-terminal tasks or `Status != closed`.
- Pool workers do NOT get IssueOps cycles; the parent cycle owns integration evidence. Worker branches PR into (or are merged by the main agent onto) the parent work branch; `accept` evidence records the verification (tests/diff review) per task.

### Sizing and performance

- `Size` clamped to 1..16 (matches host concurrency reality; Claude Code caps concurrent subagents ≈ min(16, cores−2)). Default 4.
- Claim cost: O(tasks in pool) directory scan under the pool lock. Budget: ≤ 4096 tasks per pool (explicit cap on `add-task`), scan is one `ReadDir` + per-file decode of only non-terminal tasks (terminal tasks are skipped by a status suffix in the filename? — no: keep filenames stable, decode all; at 4096 files × ~1KB this is single-digit ms and only under claim/status, not on any hook hot path). A Go benchmark pins claim latency at 1000 tasks.
- Hook hint reads the pool manifest + one `ReadDir` count only (no per-task decode) and only when a pool is linked to a bound/active cycle — hooks stay cheap.

## D6: Upstream Independence (standalone operation)

User decision (2026-07-07): agent-harness must operate fully standalone — no upstream library/service MCPs are used by harness features, none are set up during install/update, and every harness code path that depends on an upstream tool or external service is removed. This REVERSES the documented "바퀴를 재발명하지 않는다 / companion tool" policy in `.agent-harness/ARCHITECTURE.md` and the AGENTS.md invariants, so it must be recorded as an ADR with the rejected alternative (keeping opt-in upstream wiring) and rationale (independence, reproducibility, no external keys/network on core paths).

Verified dependency inventory (all to be removed):

| # | Dependency | Evidence | Replacement |
|---|-----------|----------|-------------|
| U1 | `implement` entry gate hard-requires CodeGraph (`codegraph_ready` when `!prep.CodeGraphChecked \|\| !prep.CodeGraphReady`) | `internal/core/issueops/issueops_readiness.go:193-195` | drop `codegraph_ready` from the gate; keep `WorktreeTools.CodeGraph*` fields as OPTIONAL informational evidence (JSON compat preserved); `prepare-tools` skips CodeGraph silently when absent |
| U2 | External LLM service calls: Z.AI Coding Plan HTTP API (`api.z.ai`, `$Z_AI_API_KEY`, glm-5-turbo) | `internal/core/externalllm/print.go:15-21`; consumers: `draftwiki` suggest/queue-process, `issueops/remote` judge (remote score), `issueops/benchmark` judges + self-consistency, `lintgate`, `qualitycatalog`, `external_llm_usage` | delete `internal/core/externalllm` and the Z.AI client entirely; every consumer moves to the **host-agent judgement contract**: the harness RENDERS the judge/suggest prompt (+ JSON schema) and ACCEPTS the result as a file/JSON input recorded into state (the existing judge-file input path is the template), consistent with this design's actor model — the harness never performs or purchases intelligence itself |
| U3 | draft-wiki promote writes into the upstream `m16khb/llm-wiki` hub (config/registry resolution) | `internal/core/draftwiki/llmpromote/config.go` | promote exports approved drafts to a repo-local export directory (`.agent-harness/draft-wiki/exported/`); users move them anywhere themselves; `llmpromote` removed |
| U4 | Install/update wire upstream tools (llm-wiki, CodeGraph, claude-mem, LazyCodex, Ponytail); `agent-harness update` defaults `--with-upstream-tools=true` | `scripts/install-native.sh:381-484`, `cmd/harness/updatecli/update_bootstrap.go:23-54` | delete `install_upstream_tools` and all wiring; update never touches upstream tools; the flags remain for ONE release as deprecated no-ops that print a warning (script/CLI compat), then are removed |
| U5 | `api-doc review` spawns the `codex` CLI | `cmd/harness/apidoc/api_doc_review_runner.go:24` | `api-doc review` renders the review prompt + output schema and accepts the host agent's result via a `--result <file>` input recorded as the review evidence; no process spawn |

Rules:

- Removal is fail-graceful, never fail-broken: a feature whose external step was removed either works standalone (U1, U3, U4) or converts to a render-prompt → record-result contract (U2, U5) with a clear owner command; no feature silently pretends the external step ran.
- No harness feature may add a hard dependency on CodeGraph, llm-wiki, claude-mem, or any external LLM API. This includes D1-D5 above: the orchestration features use only harness core state and gates (they already do; U1 removes the one inherited hard edge via `worktree_tools`).
- Independence is verified mechanically: after implementation, `grep -r "api.z.ai" internal cmd` returns nothing, no `codegraph_ready` key exists in any readiness gate, `install-native.sh` contains no upstream installer, and `go test ./... -count=1` passes on a machine with none of the upstream tools installed and no `$Z_AI_API_KEY`.
- Existing state records keep their JSON shape (`WorktreeTools.CodeGraph*` fields stay readable); this is a behavior removal, not a schema break.

Interaction with D1: the delegated child's `worktree_tools` gate (spec above) no longer includes `codegraph_ready` once U1 lands — the child earns dependency readiness and worktree match only.

## D4: Hooks (observe/relay only)

- **UserPromptSubmit** — extend the existing `[agent-harness]` dynamic hints (pattern: `activeWorktreeReminderValue`): when the resolved repo has a bound cycle with children or a linked active pool, add one line each: `children: 2/5 done, 1 unvalidated — issueops child status --parent <id>` and `pool <name>: 3 leased / 4 pending / 1 expired — workpool status`. Deterministic reads, bounded cost (parent record + child refs read-repair capped at N=16 children displayed; pool manifest + dir count).
- **Stop** — the next-action relay gains deterministic facts: if the bound cycle has `child_incomplete`/`child_unvalidated`/`pool_incomplete` missing keys, the relay names them (it does not judge). This prevents "declared done while children run" without making the hook a workflow actor.
- **PreToolUse** — unchanged. The per-process `HARNESS_EXPECTED_WORKTREE` env guard already gives each sub-agent session its own strong worktree fence; scoped bindings (D2) only improve post-compaction fallback in the lifecycle MCP guard chain.

## D5: Skills and docs

- `skills/issueops/SKILL.md`: new **Delegated Child Cycles** and **Worker Pool** sections — owner-command map additions (`children_complete` → `issueops child status/accept`, `pool_incomplete` → `workpool status/accept`), delegation preconditions, and the sub-agent prompt contract. Contract strings pinned by extending `issueops_skill_contract_test.go`.
- New reference `skills/issueops/references/orchestration.md`: the delegation prompt template (child: cycle id, worktree, export line, phase contract, "stop and report on scope drift"; pool worker: claim→prepare worktree→heartbeat→submit loop), validation rubric for `accept`, and the S1/S2 walkthroughs.
- `.agent-harness/SUB_AGENT_PATTERNS.md`: map D1→patterns #2/#7, D3→#7/#8 with the recorded-execution-decision requirement.
- `.agent-harness/AGENT_WORKFLOW.md`: resume contract additions (`issueops resume --id`, scoped bindings, pool worker loop).
- `.agent-harness/ADR.md`: record the decision (harness-as-coordinator, host-spawns-agents; delegated-artifact profile instead of a phase-machine fork; lease-TTL pool instead of a process-supervising executor) and rejected alternatives (harness-side process spawning — violates worker preconditions; per-child reduced phase enum — forks the machine; single shared pool file — lock contention and lost updates).
- Golden/contract surfaces: `cmd/harness/testdata/mcp_tools.golden.json`, `response_contracts.golden.json`, `cmd/harness/contractgolden`, usage goldens.

## Concurrency Model (explicit contract)

1. **Single-entity locking only — including same-entity re-entry.** No code path holds two entity locks simultaneously (two cycle locks, or a cycle lock + pool lock, or pool + task), AND no `with*Lock` callback may call another `with*Lock`-wrapped function even for the SAME entity: each call opens a fresh fd, and a second exclusive `flock` on the same lock file from a new fd blocks in-process — self-deadlock (e.g. `LinkIssueOpsChild` internally takes the parent lock, so `child start` must invoke it as a separate sequential step after its own parent-lock step releases, never inside it). Multi-entity operations are sequences of single-locked steps with read-repair for the cross-entity index (`ChildCycles`). This makes deadlock structurally impossible rather than order-disciplined. Enforced by convention + a CAUTIONS entry + review.
2. **Authoritative direction.** Child record (`Delegation.ParentCycleID`) is authoritative for membership and child phase; the parent's `ChildCycles` index is authoritative only for parent-owned validation verdicts. `child status/list` reconciles: children found by scan but missing from the index are surfaced (and appended under the parent lock on `--repair`); index entries whose child record is gone are marked orphaned.
3. **Lease correctness.** Claim/reap run under the pool lock (serialized); heartbeat/submit under the per-task lock re-check `WorkerID` + expiry after acquiring the lock (a reaped-and-reclaimed task refuses the old worker: fencing by worker-id + lease timestamp comparison). Lock files are persistent inodes, never deleted (audit lesson: deleting broke flock mutual exclusion, 35-38/50 lost updates).
4. **Scoped-binding linearization.** All binding files of a repo share the existing per-repo session flock; bind/unbind stay compare-and-delete under that lock.
5. **Race verification battery (required, not optional).** Every race test asserts CONTENT integrity, not just counts — each surviving ref/task/binding must carry internally consistent fields matching its true origin (CycleID↔Branch, WorkerID↔lease, evidence↔task), so a torn or interleaved write cannot pass as a correct cardinality:
   - concurrent `child start` for the same child branch from 2 goroutines → exactly one child record, one index entry, fields consistent;
   - concurrent index appends (5 children started in parallel) → 5 refs, each ref's CycleID/Branch/Title matching its originating request (modeled on `TestIssueOpsConcurrentFeedbackNoLostUpdate`);
   - 10 concurrent `claim` against `Size=3` → exactly 3 leases, 7 `pool_saturated` refusals, each leased task's WorkerID equal to exactly one claimant;
   - claim vs reap race on an expiring lease → task is either re-leased or reaped, never double-leased; stale worker's heartbeat/submit after reap → refused; **and after another worker reclaims, the ORIGINAL worker's submit/heartbeat is refused by worker-id fencing while the new worker's succeeds**;
   - concurrent bind/unbind of primary + scoped bindings → no binding loss for the surviving cycle;
   - all of the above under `go test -race`.
6. **Fail-closed gates are pinned through the real command surface, not only core functions.** The parent `pr` gate (children and pool) has CLI-path tests that drive the actual `issueops phase --to pr` dispatch and assert blocked-then-unblocked-after-remedy; the delegated artifact profile is verified by calling `AdvanceIssueOpsPhase` through the child's phases (readiness output alone is not acceptance).
7. **Performance guards are deterministic tests, not benchmarks.** `go test` skips `-bench`; therefore claim latency at 1000 tasks is enforced by a normal test with a generous fixed wall-clock budget (fails on pathological regressions, tolerant of CI noise), alongside an informational benchmark.

## Backward Compatibility

- At this design's July 6 baseline, the delegation additions were `omitempty` fields under IssueOps schema v1. Issue #16 supersedes the root version with schema v4 for supervised ownership, stable terminal identity, and sealed completion authority; records without delegation fields and parents without children still preserve the same behavior.
- The historical mixed-binary risk was that an older writer could drop unknown delegation fields. Current schema-v4 writers instead require v1 to reject v2+, v2 to reject v3, and v3 to reject v4 before rewrite; the single central binary update model remains operationally required.
- The workpool namespace is separate and remains at `schema_version=1`; its records apply the same fail-closed-on-future-version principle, not the IssueOps root version number.
- Legacy per-repo session binding keeps working unchanged for the single-cycle workflow.

## CLI and MCP Surface

New CLI: `issueops child start|status|list|accept|reject|drop`, `issueops bind --id`, `issueops resume --id`, `issueops heartbeat --id` (expose the existing core heartbeat for sub-agent liveness), `workpool create|add-task|claim|heartbeat|submit|accept|reject|reap|status|close`.
New MCP tools (same daemon, both server identities): `issueops_child_start`, `issueops_child_status`, `issueops_child_accept`, `issueops_child_reject`, `issueops_child_drop`, `issueops_resume` (extended arg `id`), `issueops_heartbeat`, `workpool_create`, `workpool_add_task`, `workpool_claim`, `workpool_heartbeat`, `workpool_submit`, `workpool_accept`, `workpool_reject`, `workpool_status`, `workpool_reap`, `workpool_close`.
Every readiness missing key maps to exactly one owner command in `issueops status` output (ledger convention).

## Testing

Core: the race battery above; fail-closed child-start preconditions BOTH directions (each blocked case AND per-condition remedy: fix only that field → succeeds); delegated artifact profile proven by actual `AdvanceIssueOpsPhase` calls (not readiness output alone): problem→grill→plan succeed immediately, `compatibility-review` entry FAILS with the child's own missing `branch_prepare`/`worktree_path`/`plan_in_worktree`, succeeds after the child earns them, and `implement` entry still requires child `worktree_tools`/`execution_decision`; stale reset preserves `Delegation`/`ChildCycles` in both directions (reset parent keeps refs+verdicts; reset child keeps parent pointer and stays `child_incomplete`); parent regress in delegation context: implement-phase regress refused by the EXISTING phase precondition (pinned), and the `children_active` guard exercised via the stale-reset corner (reset parent re-advanced to plan + DA stop → refused while children live → allowed after resolution); parent force-done/force-release with active children records the audit list and children surface `parent_closed`; child-link partial success returns `child_link_warning` with the child cycle intact; reject success path records the verdict and keeps the gate blocked; force-released child remains `child_incomplete` (ForceReleasedAt predicate) and is resumable via `resume --id --bind` to completion; read-repair both directions; lease lifecycle incl. attempts cap and exact expiry boundary (`now == LeaseExpiresAt` expired, one tick before not); worker-id fencing incl. reclaimed-by-another-worker; pool clamp table (0/1/16/17/-1) and TTL edge table ("", "0s", "-5m", "15m") with exact expected outcomes; zero-task pool close; closed pool refuses all mutations; reap idempotence; scoped binding resume after simulated compaction (env cleared) and env-beats-binding precedence.
CLI/MCP: lifecycle tests per new command/tool; **CLI-path gate tests driving real `issueops phase --to pr` dispatch (children and pool variants, blocked→remedy→unblocked)**; goldens; skill-contract test extensions; `issueops status --json` shows children/pool summaries + owner commands.
Performance: deterministic budget test for claim at 1000 tasks (normal `go test` path) + informational `BenchmarkClaimAt1000Tasks`; `child status` at 16 children budget test.
Verification commands:

```bash
go test ./internal/core/issueops/... -count=1
go test ./internal/core/workpool/... -count=1
go test -race ./internal/core/issueops/... ./internal/core/workpool/... -count=1
go test ./cmd/harness/... -count=1
go test ./cmd/harness/... -run Golden -count=1
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

Dogfood scenarios (documented in the plan; run before declaring done):
- **B1 (S1):** umbrella cycle + 2 child cycles in this repo, two sub-agent sessions resuming by `--id`, parent `pr` blocked until both children `done`+accepted.
- **B2 (S2):** pool of size 2 with 5 tasks; 3 claims → third refused; one worker abandoned → lease expires → reaped → reclaimed; accept/reject paths; parent gate clears on close.

Independence verification (D6):

```bash
grep -rn "api.z.ai" internal cmd            # expect: no matches
grep -rn "codegraph_ready" internal/core/issueops/issueops_readiness.go   # expect: no matches
grep -n "install_upstream_tools" scripts/install-native.sh                # expect: no matches
Z_AI_API_KEY= go test ./... -count=1        # green without any upstream tool installed
```

## Rollout

Additive state first, no destructive migration. Recommended order: **U1 (CodeGraph gate removal) first** — it simplifies every worktree_tools fixture the delegation tests touch — then D1+D2 (hierarchical delegation), then D3 (pool), with the remaining D6 removals (U2-U5) as an independent parallel track. S1 is fully useful alone and exercises the binding/gate machinery D3 reuses. After tests pass: `agent-harness update --path-mode=skip --json`, then verify `daemon status --json`, `codex mcp get agent_harness`, `claude mcp list`, run B1/B2, and run the D6 independence verification block above.
