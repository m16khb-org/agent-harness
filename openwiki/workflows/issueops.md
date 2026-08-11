---
type: Workflow
title: IssueOps Workflow
description: The durable issue-driven work cycle with a 9-phase state machine, fail-closed readiness gates, and a phase ledger. IssueOps is the single workflow authority in agent-harness.
tags: [issueops, workflow, phase-machine, readiness-gates]
---

# IssueOps Workflow

IssueOps is the central workflow engine of agent-harness. It moves task context out of conversation and into a durable cycle connecting issue, plan, worktree, feedback, and verification evidence. The cycle survives across sessions and hosts because state is persisted in [SQLite](../operations/state-and-storage.md) under a dedicated namespace.

IssueOps is the **single workflow authority** — adapters like Orca provide execution capability but IssueOps retains durable authority over phase, lease, and completion.

## Phase State Machine

IssueOps enforces a strict 9-phase forward-only state machine. No backward transitions are allowed; once in `done`, no transitions occur.

```mermaid
stateDiagram-v2
    [*] --> problem: issueops start
    problem --> grill: intent contract ready
    grill --> plan: issue URL + plan prep + domain review
    plan --> compatibility_review: compat review + worktree
    compatibility_review --> implement: devils advocate + execution lease
    implement --> ai_slop_clean: implementation changes
    ai_slop_clean --> feedback: slop clean fingerprint
    feedback --> pr: strict PR readiness
    pr --> done: completion + lease released
    done --> [*]

    note right of grill
        Readiness gate:
        intent_contract required
    end note

    note right of implement
        Execution lease active
        Worktree match required
    end note
```

Source: [`internal/domain/issueops/phase.go`](/internal/domain/issueops/phase.go), [`internal/adapter/issueops/issueops_phase.go`](/internal/adapter/issueops/issueops_phase.go).

### Phase Definitions

| Rank | Phase | Key Gate |
|------|-------|---------|
| 1 | `problem` | Entry — no gate (starting state) |
| 2 | `grill` | `intent_contract` (raw request + interpreted intent + success criteria) |
| 3 | `plan` | `issue_url`, `plan_prep` (4 items unless trivial), `split_decision`, `domain_review` |
| 4 | `compatibility-review` | `compatibility_review` (compat, side effects, rollback, verification, no blockers, approved), `worktree_exists`, `plan_in_worktree` |
| 5 | `implement` | All compat-review items + `devils_advocate_review` (pass or waived) + valid `execution` with active write lease + `execution_worktree_match` |
| 6 | `ai-slop-clean` | All implement items + `implementation_changes` evidence |
| 7 | `feedback` | `AISlopCleanAt` non-empty |
| 8 | `pr` | Strict PR readiness (state-root-aware gate) |
| 9 | `done` | Remote artifact verified, `execution.completion` present, `lease.status == released` |

### Worktree Expectations

Phases implement, ai-slop-clean, feedback, and pr expect a worktree (`IssueOpsPhaseExpectsWorktree`). However, only implement, ai-slop-clean, and feedback can reset on a stale worktree — pr artifacts live remotely so PR-phase records are excluded from stale-worktree reset.

## Readiness Gates

Every phase transition passes through a fail-closed readiness function. The gate checks for specific evidence fields in the `IssueOpsRecord`. Missing any required field blocks the transition.

The readiness model is layered: later phases inherit all earlier requirements. For example, entering `implement` requires everything from `compatibility-review` plus the devil's-advocate review and an active execution lease.

Source: [`internal/domain/issueops/`](/internal/domain/issueops/), [`internal/adapter/issueops/issueops_readiness.go`](/internal/adapter/issueops/issueops_readiness.go), [`internal/adapter/issueops/issueops_pr_readiness_strict.go`](/internal/adapter/issueops/issueops_pr_readiness_strict.go).

## Phase Ledger

Every forward transition is recorded in an additive audit index called the **Phase Ledger** — a `map[IssueOpsPhase]PhaseLedgerEntry` with `EnteredAt`, `CompletedAt`, `Artifacts`, `Missing`, and `Notes`. The ledger records what was observed, not what is enforced. Source-of-truth fields drive the gates; the ledger provides visibility and diagnostics.

Source: [`internal/adapter/issueops/issueops_phase_ledger.go`](/internal/adapter/issueops/issueops_phase_ledger.go).

## Brooks Regress Events

When a devil's-advocate review produces a `stop` or `revise` verdict, a regress event is recorded (`IssueOpsRegressEvent` with reason, from-phase, timestamp). A regress cap escalates to human decision after repeated rounds, preventing infinite revision loops.

Source: [`internal/adapter/issueops/issueops_regress.go`](/internal/adapter/issueops/issueops_regress.go).

## IssueOpsRecord — Root Aggregate

```mermaid
erDiagram
    IssueOpsRecord ||--|| IssueOpsIntent : has
    IssueOpsRecord ||--o| IssueOpsDesignReview : has
    IssueOpsRecord ||--o| IssueOpsPlanPrep : has
    IssueOpsRecord ||--o| IssueOpsCompatibilityReview : has
    IssueOpsRecord ||--o| IssueOpsDevilsAdvocateReview : has
    IssueOpsRecord ||--o| IssueOpsBranchPrepare : has
    IssueOpsRecord ||--o| Execution : has
    IssueOpsRecord ||--o| IssueOpsRemoteArtifact : has
    IssueOpsRecord ||--o{ IssueOpsDecision : records
    IssueOpsRecord ||--o{ IssueOpsFeedbackItem : collects
    IssueOpsRecord ||--o{ IssueOpsRegressEvent : tracks
    IssueOpsRecord ||--o| IssueOpsDelegationContract : has

    IssueOpsRecord {
        string id
        string repo
        string branch
        IssueOpsPhase phase
        int schema_version
        timestamp created_at
        timestamp updated_at
    }
    Execution {
        ExecutionMode mode
        Workspace workspace
        WriteLease lease
        OrcaBinding orca
        ExecutionCompletion completion
    }
```

The `IssueOpsRecord` is the root aggregate, persisted as a single JSON row per IssueOps ID. The ID is deterministic: `io-` + first 12 hex chars of `SHA-256(repo + "\x00" + branch)`.

Source: [`internal/domain/issueops/`](/internal/domain/issueops/), [`internal/adapter/issueops/`](/internal/adapter/issueops/).

## Intent Class

The intent class controls plan-prep gate strictness: `trivial` (skips the gate), `standard` (default), `refactoring`, `architecture`, `research`. Empty normalizes to `standard`.

Source: [`internal/domain/issueops/intent_class.go`](/internal/domain/issueops/intent_class.go).

## State Persistence

IssueOps v1 state lives in a physically separate SQLite namespace: `issueops_v1/harness.db`, bucket `issueops_v1`. Schema version is `1`. Legacy authority fields are explicitly forbidden — their presence triggers a fail-closed error. The schema version is bumped in the bucket name, not via in-place migration.

All writes require `RequireIssueOpsMutationAllowed(stateRoot)` and are serialized through [sqlstore spans](../operations/state-and-storage.md) (BEGIN IMMEDIATE on a lock database).

Source: [`internal/adapter/issueops/issueops_state.go`](/internal/adapter/issueops/issueops_state.go).

## Brooks Devil's Advocate and Regress

A devil's-advocate review (verdict: `pass | revise | stop`) is a **fail-closed precondition** for `implement` entry. A `stop` or unwaived `revise` blocks the transition. When the devil's advocate returns `stop`, the cycle regresses from `plan` or `compatibility-review` back to **`grill`** for re-investigation and re-planning.

```mermaid
stateDiagram-v2
    [*] --> problem
    problem --> grill
    grill --> plan
    plan --> compatibility_review
    compatibility_review --> implement: devil's advocate pass
    compatibility_review --> grill: devil's advocate stop (regress)
    plan --> grill: devil's advocate stop (regress)
    implement --> ai_slop_clean
    note right of compatibility_review
        Regress cap = 3
        Repeated stop→re-plan escalates
        to human decision
    end note
```

Regression does **not** delete the worktree, branch, or artifacts. It clears design approval and marks downstream ledger entries stale (retained as audit). The regress cap (`issueOpsRegressCap = 3`) prevents infinite revision loops.

Source: [`internal/adapter/issueops/issueops_regress.go`](/internal/adapter/issueops/issueops_regress.go), [`internal/adapter/issueops/devilsadvocate/devils_advocate.go`](/internal/adapter/issueops/devilsadvocate/devils_advocate.go).

## Cleanup and Post-Merge

Post-merge cleanup is a separate human-authorized operation, not part of phase completion. It has multiple paths:

| Command | Target | Key behavior |
|---------|--------|-------------|
| `cleanup finish` | Post-merge record-backed cycle | Preview → fingerprint CAS → resumable destructive steps |
| `cleanup orphan` | Recordless worktree (merged artifact) | Preview → fingerprint CAS → worktree+branch deletion |
| `cleanup abandon` | Cycle with Orca residue | Preview → fingerprint CAS → Orca cleanup |
| `cleanup remote-branch` | Remote branch | Ancestry-checked deletion |
| `cleanup close-children` | Provider-native child work items | Post-merge close |
| `prune` | Bulk stale cycles | Time-based pruning |

### Cleanup Status and Missing Polarity

`cleanupstatus.ForRecord` computes a `Missing` list that must be empty for cleanup readiness. Items are written in **requirement form** — `worktree_clean` means "cleanliness is required and not met," not "the worktree is dirty." This polarity convention was unified in [issue #185](https://github.com/m16khb/agent-harness/issues/185) to eliminate ambiguity.

Required items include: `pr_phase`, `remote_artifact` (with sub-fields for provider/kind/url/labels/assignees), `remote_artifact_merged`, `child_tasks_closed`, `worktree_path`, `worktree_exists`, `worktree_git_status`, `worktree_clean`, `branch`, `branch_match`, and remote branch checks.

### Cleanup Finish Protocol

The `cleanup finish` protocol uses a TOCTOU-safe compare-and-swap:

1. **Preview** — all gates must pass; compute fingerprint
2. **Apply** — `--apply --confirm --fingerprint <hash>` must match freshly recomputed fingerprint
3. **Destructive steps** (resumable on failure — failure point recorded in `CleanupFinishFailure`):
   - Orca worktree removal (`force=false`, absence = success)
   - `git worktree remove` (idempotent)
   - `git update-ref -d refs/heads/<branch> <head-OID>` (HEAD CAS, absence = skip)
   - Best-effort audit line reflection (completion payload snapshot taken **before** destruction)
   - Record deletion

Workspace process quiescence (`inspectWorkspaceProcesses`) blocks if processes hold the worktree (excluding requester ancestry).

### Orphan Cleanup

Orphan cleanup targets **recordless** worktrees whose remote artifacts are already merged — cycles that never completed formal phases. It requires: `repo_root_match`, `inventory_complete`, `canonical_repo_root`, exactly one target worktree, valid worktree HEAD, `record_absent`, no lease-holder authority, no Orca worktree authority. The remote branch is intentionally untouched — deletion requires separate explicit approval.

Source: [`internal/adapter/issueops/issueops_cleanup_finish.go`](/internal/adapter/issueops/issueops_cleanup_finish.go), [`internal/adapter/issueops/cleanupstatus/cleanup_status.go`](/internal/adapter/issueops/cleanupstatus/cleanup_status.go), [`internal/adapter/issueops/orphancleanup/orphan_cleanup.go`](/internal/adapter/issueops/orphancleanup/orphan_cleanup.go), [`internal/adapter/issueops/issueops_cleanup_abandon.go`](/internal/adapter/issueops/issueops_cleanup_abandon.go).

### Superseding Artifact Verification

When a completed record's original PR was closed-unmerged but a replacement PR that explicitly includes those changes was merged, the normal cleanup paths dead-end — `cleanup finish` rejects "artifact not merged" and `cleanup abandon` rejects "phase is done" ([issue #283](https://github.com/m16khb/agent-harness/issues/283)). The `--superseded-by` flag on `cleanup finish` accepts a replacement artifact URL and verifies it via provider readback (`ValidateSupersedingArtifact`). The check is fail-closed: the replacement must be merged, must belong to the same provider and repo, and its body must explicitly declare it supersedes the original URL on the same line. Cross-repo replacements are not accepted, and mere URL mentions in issue listings or quotes do not qualify.

Source: [`internal/domain/issueops/superseding_artifact.go`](/internal/domain/issueops/superseding_artifact.go), [`internal/adapter/issueops/issueops_cleanup_finish.go`](/internal/adapter/issueops/issueops_cleanup_finish.go).

## Delegation and Child Cycles

IssueOps supports parent-child cycle delegation via `IssueOpsDelegationContract` (parent cycle ID, task scope, acceptance criteria) and `IssueOpsChildCycleRef` entries. This enables umbrella/topology workflows where a parent issue spawns child implementation cycles.

### Handoff Sealing and Child Recovery

Handoff generation is sealed to a deterministic token path derived from `tokenSHA256(record.ID)[:16]`, not an arbitrary user-supplied path. Claim tokens use `O_CREAT|O_EXCL` with `0600` permissions and reject symlinks and oversized files. MCP audit responses are drained before pipe closure.

When a child cycle is cleaned and restarted, the parent's child reference is reset **only** when the child's incarnation is strictly newer (`issueOpsChildRefHasNewerIncarnation` compares `CreatedAt` timestamps). If the child record is missing (exact cleanup removed it), the parent records an audited receipt against the indexed missing child after confirming absence under the held parent span.

Only well-formed receipts — `accepted` with evidence, or `dropped` with a reason of at least 10 characters — count as terminal for status and PR gates.

### Unresolved Children Count

`cleanup abandon` blocks when unresolved child cycles exist. A child counts as resolved only when its record is observed with `phase == done` — record absence is **not** evidence of resolution (the child could be lost, not finished). To clean an absent child, the parent must first record an audited receipt via `issueops cleanup close-children` (which performs remote observation). This unified both code paths (IssueLinks and ChildCycles) to the same standard, fixing a split where one path checked `CloseVerifiedAt` and the other blocked on record existence alone ([issue #437](https://github.com/m16khb/agent-harness/issues/437)). The diagnostic returns specific child IDs, not just a count, so the user knows exactly what to resolve first.

### Ghost Owner Replacement

A released Orca owner can leave a terminal row whose pane is no longer live. `PlanResume` previously treated this as an identity conflict, blocking replacement. Now, when the terminal ID is present but `TerminalLive == false`, and the terminal inventory is complete with a settled dispatch status matching the sealed binding, the disposition is `ResumeCreateTerminal` instead of denial. This admits only the exact prior PTY identity.

### Lease Recovery for Absent Worktrees

If a lease's canonical worktree directory no longer exists, the lease is transitioned directly to `Released` (not `Claimable`) during finalize. This opens the `abandon` path — `Claimable` would be false because no workspace means no token file can be read. The absence is sealed to a fingerprint bound to exact path and branch, so a resurrected worktree produces a different fingerprint.

### Reconcile Intent Clearing

When a mutation's invocation result is unknown but the external inventory returns an authoritative zero (confirmed absence via actual Orca query), `PlanReconcileStage` returns `ReconcileStageClear` with reason `"invocation-left-no-resource"`. This clears the pending intent without recording a failure. Non-authoritative zeros still preserve the intent.

### Orphan Linked Branch Recovery

GitHub's `createLinkedBranch` can return `linkedBranch: null` without error, leaving a ref-null orphan record. The typed cleanup control plane classifies observation into `StateAbsent`, `StateHealthy`, `StateOrphan`, `StateMismatched`, or `StateAmbiguous`. Only `StateOrphan` (ref-null) is deletable, by node ID only (not branch name). The flow is preview, classify, fingerprint-bind, re-observe (stale check), delete, and record all outcomes as durable audit.

Source: [`internal/adapter/issueops/`](/internal/adapter/issueops/), [`internal/domain/issueopslinkedbranch/`](/internal/domain/issueopslinkedbranch/), [`internal/domain/issueopslease/reconcile.go`](/internal/domain/issueopslease/reconcile.go).

## IssueOps Skill

The `skills/issueops/` skill provides the operational reference for the workflow, with phase-specific reference files:

| Reference | Covers |
|-----------|--------|
| `operational-start.md` | Starting a cycle |
| `execution.md` | Orca execution mode details |
| `worktree-context.md` | Worktree provisioning |
| `remote-issue.md` | Remote issue creation and linking |
| `review-feedback.md` | Review and feedback phases |
| `ai-slop-clean.md` | AI slop clean phase |
| `cleanup-state.md` | Post-merge cleanup |
| `evidence-contract.md` | Evidence requirements |
| `orchestration.md` | Cross-phase orchestration |

The [Execution Model](execution-model.md) page covers the execution lease, generation fence, and completion gate in depth.
