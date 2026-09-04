# IssueOps Phase Ledger Design

## Problem

IssueOps already has durable fields for many gates, including intent, plan prep, branch preparation, linked worktree, design review, worktree tool preparation, compatibility review, execution decision, AI slop cleanup, feedback, PR readiness, and remote artifact verification. The missing piece is a phase-level ledger that says which phase has entered, which phase has completed, and which concrete artifacts satisfied that phase.

Without that ledger, adjacent steps can be confused. For example, `issueops worktree prepare` only reports the worktree path and next command, while `issueops worktree prepare-tools` records dependency and CodeGraph readiness. Both are legitimate commands, but the current state shape makes it too easy for an agent to treat "worktree exists" as "worktree tools are prepared".

## Goal

Add a backward-compatible `phase_ledger` to IssueOps records and use it as a phase completion index. The ledger must make phase progress resumable, auditable, and fail-closed: a phase cannot be entered until the previous phase's required artifacts are complete, and completion evidence must point back to a single source-of-truth field rather than duplicating truth.

"Single source-of-truth field" means existing fields wherever one already backs the artifact, and a newly declared field where none exists yet. Declaring one new authoritative field is not duplication; copying an existing value into a second field is. The mapping in [Source-of-Truth Mapping](#source-of-truth-mapping) enumerates which artifacts are already backed, which are gated today at a *different* phase than this design assigns them, and which require a net-new field or check. The artifact matrix below was written against the intended phase model, not the current gate locations, so those deltas are real prerequisites, not descriptions of existing behavior.

## Non-Goals

- Do not replace the current IssueOps phase list.
- Do not remove or rename existing JSON fields.
- Do not make hooks perform workflow work. Hooks may observe and block deterministic violations only.
- Do not store issue bodies, secrets, raw command output, or provider tokens in the ledger.
- Do not force migration of old state files before they can be read.

## Current Evidence

- `internal/core/issueops/issueops_phase.go` already blocks several phase transitions through readiness checks.
- `internal/core/issueops/issueops_readiness.go` already computes missing keys for plan, compatibility review, implementation, and AI slop cleanup; PR readiness lives in `issueops_pr_readiness.go` and `issueops_pr_readiness_strict.go`.
- `internal/core/issueops/model/types.go` stores each major artifact as a top-level IssueOps field.
- `issueops worktree prepare-tools` persists `worktree_tools`, including exact worktree path, dependency readiness, CodeGraph path, and CodeGraph readiness.
- The current gap is not absence of every gate; it is absence of a single phase ledger that proves each phase has completed and records which fields satisfied it.

## State Shape

Add these model types:

```go
type IssueOpsPhaseLedgerEntry struct {
	Phase       IssueOpsPhase `json:"phase"`
	EnteredAt   string        `json:"entered_at,omitempty"`
	CompletedAt string        `json:"completed_at,omitempty"`
	Artifacts   []string      `json:"artifacts,omitempty"`
	Missing     []string      `json:"missing,omitempty"`
	Notes       []string      `json:"notes,omitempty"`
}

type IssueOpsPhaseLedger map[IssueOpsPhase]IssueOpsPhaseLedgerEntry
```

The map key is the authoritative phase identity. `IssueOpsPhaseLedgerEntry.Phase` is a self-describing copy that must equal its key; it is validated on read/backfill and the key wins on any divergence, so the redundancy cannot create two sources of truth.

Add this field to `IssueOpsRecord`:

```go
PhaseLedger IssueOpsPhaseLedger `json:"phase_ledger,omitempty"`
```

### Net-new backing fields (prerequisites)

These fields do not exist today and must be added before the ledger can require their artifacts. They are authoritative source-of-truth, not copies of other fields. All are additive and `omitempty`, preserving backward compatibility.

```go
// grill: domain grilling currently produces no record state.
type IssueOpsDomainReview struct {
	Terminology       []string `json:"terminology,omitempty"`
	ModelFit          string   `json:"model_fit,omitempty"`
	Risks             []string `json:"risks,omitempty"`
	OpenUncertainties []string `json:"open_uncertainties,omitempty"`
	ReviewedAt        string   `json:"reviewed_at"`
}

// Added to IssueOpsRecord:
DomainReview            *IssueOpsDomainReview `json:"domain_review,omitempty"`
AISlopCleanCategories   []string              `json:"ai_slop_clean_categories,omitempty"` // cleanup_evidence
AISlopCleanVerification []string              `json:"ai_slop_clean_verification,omitempty"` // verification_evidence

// Added to IssueOpsFeedbackItem:
Resolution string `json:"resolution,omitempty"` // valid-defect | question-answered | noise-dismissed

// Added to IssueOpsRemoteArtifactVerification (for target_branch_match):
TargetBranch string `json:"target_branch,omitempty"`
```

`split_decision` needs no new field — its completion helper reads existing `record.IssueLinks` (child entries) and `record.Decisions` (kind `scope`).

The ledger is an index over existing fields. For example, `phase_ledger["implement"].artifacts` may contain `worktree_tools`, but the authoritative data remains `record.WorktreeTools`.

## Source-of-Truth Mapping

Every artifact in the matrix below maps to exactly one authoritative field or computed check. This table is the contract the ledger indexes; it was reconciled against the current code in `internal/core/issueops`. Three statuses appear:

- **existing** — a field/check already backs the artifact and is gated at the phase this design assigns it.
- **existing, regated** — a field already exists, but today it is enforced at a *different* phase. The ledger re-indexes its completion at the assigned phase; the original gate must be kept (or intentionally moved) so behavior stays fail-closed.
- **new** — no field or check exists; it must be added before the ledger can require it. These are prerequisites, declared in [State Shape](#state-shape) and recorded by a [CLI/MCP](#cli-and-mcp-surface) command.

| Phase | Artifact | Authoritative source | Today's gate | Status |
| --- | --- | --- | --- | --- |
| problem | `intent_contract` | `record.Intent` (`issueOpsIntentMissing`) | enforced at `plan` (`IssueOpsPlanReadiness`) | existing, regated |
| grill | `issue_url` | `record.IssueURL` | enforced at `plan` | existing, regated |
| grill | `branch` | `record.Branch` | validated at start (`branchprepare.ValidateBranch`), not phase-gated | existing, regated |
| grill | `plan_prep` | `record.PlanPrep` (`planPrepMissing`) | enforced at `plan` | existing, regated |
| grill | `split_decision` | `record.IssueLinks` (child links) or a `record.Decisions` entry with kind `scope` (no-split rationale) | not gated in state today (enforced only as benchmark text scoring in `benchmark/issueops_quality_checks.go`) | existing fields, new gate |
| grill | `domain_review` | none | not persisted (grill produces no record state) | **new** |
| plan | `branch_prepare` | `record.BranchPrepare` | enforced via branch-prepare flow | existing |
| plan | `worktree_path` | `record.WorktreePath` | enforced at `compatibility-review`/`implement` | existing, regated |
| plan | `plan_path` | `record.PlanPath` | enforced at `compatibility-review`/`implement` | existing, regated |
| plan | `design_review` | `record.DesignReview` (`issueOpsDesignReviewMissing`) | enforced at design-review record | existing |
| compatibility-review | `compatibility_review` / `compatibility_approval` / `compatibility_blockers` | `record.CompatibilityReview` (`issueOpsCompatibilityReviewMissing`) | enforced at `compatibility-review` and `implement` | existing |
| implement | `worktree_tools` | `record.WorktreeTools` (`issueOpsWorktreeToolsMissing`) | enforced at `implement` | existing |
| implement | `execution_decision` | `record.ExecutionDecision` | enforced at `implement` | existing |
| implement | `implementation_changes` | `implementation.HasEvidence` / `ChangeFingerprint` (derived, no stored field) | enforced at `ai-slop-clean` (`IssueOpsAISlopCleanReadiness`) | existing (derived), regated |
| ai-slop-clean | `ai_slop_clean_at` / `_head` / `_fingerprint` | `record.AISlopClean*` | set on entering `ai-slop-clean` | existing |
| ai-slop-clean | `cleanup_evidence` | none | not persisted | **new** |
| ai-slop-clean | `verification_evidence` | none | not persisted | **new** |
| feedback | `feedback_classification` | `record.Feedback[].Classification` | validated on feedback add | existing |
| feedback | `contract_feedback_issue_update` | `record.Feedback[].IssueUpdatedAt` (`MarkIssueOpsContractFeedbackIssueUpdated`) | enforced at `pr` (`issueOpsHasUnresolvedContractFeedback` in `issueops_pr_readiness.go`) | existing, regated |
| feedback | `feedback_resolution` | none | not persisted | **new** |
| pr | `strict_pr_readiness` | `IssueOpsStrictPRReadiness` | enforced at `pr` | existing |
| pr | `remote_artifact` | `record.RemoteArtifact` | verified at `done` (`issueOpsRemoteArtifactMissing`) | existing, regated |
| pr | `target_branch_match` | none — needs `RemoteArtifact.TargetBranch` compared to `BranchPrepare.BaseBranch` | not checked (`IssueOpsStrictPRReadiness` only checks `branch_match` = local branch vs `record.Branch`) | **new** |
| done | `prior_phase_pr` | phase rank (`record.Phase == pr` guard) | enforced at `done` | existing |
| done | `verified_remote_artifact` | `issueOpsRemoteArtifactMissing(record)` | enforced at `done` | existing |

Net-new prerequisites (no backing today): `domain_review`, `cleanup_evidence`, `verification_evidence`, `feedback_resolution`, `target_branch_match`. `split_decision` has backing fields but no state-level gate. Everything marked **existing, regated** keeps its original gate; the ledger adds a completion index at the design-assigned phase without removing the stricter downstream check.

## Artifact Matrix

In this matrix, a phase's "Required artifacts" are its COMPLETION set — the prerequisites to ENTER the next phase, not to enter the phase itself. Phases are traversed by rank (`IssueOpsPhases` order); a forward jump of more than one rank requires every intervening phase to be complete. Artifacts a phase PRODUCES (the remote PR/MR in `pr`, the change fingerprint in `implement`, the cleanup evidence in `ai-slop-clean`) are exit/completion artifacts, never entry gates (see rule 10). Example: `implementation_changes` is listed under `implement` but is an exit artifact — today it gates `ai-slop-clean` entry, which is exactly "implement is complete".

### problem

Required artifacts:

- `intent_contract`: raw request, interpreted intent, success criteria.

Purpose: do not allow grilling to proceed without a minimal intent contract. Only `intent_contract` gates `problem` completion, so domain exploration can begin before the remote issue or branch exist — preserving the current free `problem` -> `grill` step. `issue_url` and `branch` are deliberately grill artifacts (below), because the documented workflow creates the issue during grill, before plan.

### grill

Required artifacts:

- `issue_url` (existing, regated): linked remote issue, or an explicit non-remote waiver. Backed by `record.IssueURL`; the issue is created during grill (the documented `problem -> grill -> issue -> plan` workflow), so it gates `plan` entry, not `grill` entry. The non-remote waiver branch is NOT backed today — it requires a `record.Decisions` entry of a defined kind (e.g. `scope`) treated as satisfying `issue_url`, or the waiver clause must be dropped.
- `branch` (existing, regated): issue-number-prefixed IssueOps branch. Backed by `record.Branch`, validated at start by `branchprepare.ValidateBranch`; gates `plan` entry, not `grill` entry, so branchless early exploration is not blocked.
- `plan_prep` (existing, regated): prior decisions, related issues, and web research evidence or per-item waive reasons. Backed by `record.PlanPrep`; today this is gated at `plan`, so the `plan` gate must remain in addition to the grill ledger index. The intent-dependent trivial-class carve-out (`planPrepGateApplies`) moves with it: grill completion waives `plan_prep` for a trivial intent class exactly as plan entry does today.
- `split_decision` (existing fields, new gate): no-split rationale or provider-native child task evidence. Backed by `record.IssueLinks` child entries or a `record.Decisions` entry of kind `scope`; no state-level gate exists today, so a completion helper must read those fields.
- `domain_review` (**new**): terminology, current model fit, risks, and unresolved uncertainties. No backing field exists; requires a new `record.DomainReview` field (see State Shape) recorded by a new `issueops domain-review record` command. Until that field exists this artifact cannot be required without creating duplicate truth.

Purpose: do not allow planning to proceed without investigation and scope pressure testing.

### plan

Required artifacts:

- `branch_prepare`: provider-linked branch, base branch, and link verification.
- `worktree_path`: existing isolated sibling worktree.
- `plan_path`: implementation plan path inside the linked worktree.
- `design_review`: approved design with refactor plan, alternatives, risks, verification, and no open questions.

Purpose: do not allow compatibility review or implementation without a real plan, branch contract, and isolated worktree.

### compatibility-review

Required artifacts:

- `compatibility_review`: backward compatibility, side effects, rollback plan, and verification.
- `compatibility_approval`: approved review.
- `compatibility_blockers`: no blockers.

Purpose: do not allow implementation before public-contract and side-effect judgment is recorded.

### implement

Required artifacts:

- `worktree_tools` (existing): exact linked worktree match, dependency readiness, CodeGraph checked, and CodeGraph ready. Backed by `record.WorktreeTools` (`issueOpsWorktreeToolsMissing`).
- `execution_decision` (existing): auto-proceed boundaries, hook-blocked work, human gates, and sub-agent decision. Backed by `record.ExecutionDecision`.
- `implementation_changes` (existing, derived, regated): change fingerprint from the linked worktree. Derived via `implementation.HasEvidence`/`ChangeFingerprint` (no stored field); today it is gated at `ai-slop-clean`, not `implement`. Keep the downstream gate.

Purpose: prevent "worktree exists" from being confused with "worktree is prepared", and prevent implementation from starting without the pre-work decision record.

### ai-slop-clean

Required artifacts:

- `ai_slop_clean_at` (existing): cleanup timestamp. Backed by `record.AISlopCleanAt`.
- `ai_slop_clean_head` (existing): Git HEAD at cleanup. Backed by `record.AISlopCleanHead`.
- `ai_slop_clean_fingerprint` (existing): implementation fingerprint at cleanup. Backed by `record.AISlopCleanFingerprint`.
- `cleanup_evidence` (**new**): categories checked or cleaned. No backing field; requires a new `record.AISlopCleanCategories []string`.
- `verification_evidence` (**new**): relevant tests or checks rerun after cleanup. No backing field; requires a new `record.AISlopCleanVerification []string`.

Today entering `ai-slop-clean` only stamps the timestamp/head/fingerprint; the two evidence lists are net-new and must be recorded by the cleanup command before this phase can be marked complete.

Purpose: do not let cleanup become a verbal claim.

### feedback

Required artifacts:

- `feedback_classification` (existing): every feedback item has a classification. Backed by `record.Feedback[].Classification`.
- `contract_feedback_issue_update` (existing, regated): contract-changing feedback updated the remote issue body. Backed by `record.Feedback[].IssueUpdatedAt` (`MarkIssueOpsContractFeedbackIssueUpdated`); today this is gated at `pr` (`issueOpsHasUnresolvedContractFeedback`). Keep the `pr` gate; the ledger only indexes it earlier.
- `feedback_resolution` (**new**): valid defect, question, and noise outcomes are recorded. No backing field; requires a new `record.Feedback[].Resolution` (e.g. `valid-defect` / `question-answered` / `noise-dismissed`).

Purpose: do not let review, CI, or user feedback disappear before PR readiness.

### pr

Required artifacts:

- `strict_pr_readiness` (existing): strict readiness passes. Backed by `IssueOpsStrictPRReadiness`.
- `remote_artifact` (existing, regated): provider, kind, URL, labels, assignees. Backed by `record.RemoteArtifact`; today its presence/verification is only required at `done` (`issueOpsRemoteArtifactMissing`), not at `pr`. The ledger may index it at `pr`, but creating the remote artifact legitimately happens during the `pr` phase, so requiring it to *enter* `pr` would be a deadlock — index it on `pr` *completion*, not entry.
- `target_branch_match` (**new**): PR/MR target matches `branch_prepare.base_branch`. No check exists today — `IssueOpsStrictPRReadiness` only checks `branch_match` (local checked-out branch vs `record.Branch`). Requires a new `RemoteArtifact.TargetBranch` field compared to `record.BranchPrepare.BaseBranch`.

Purpose: prove the remote artifact matches the IssueOps contract before completion.

### done

Required artifacts:

- `prior_phase_pr`: the loop entered PR phase first.
- `verified_remote_artifact`: remote artifact verification exists.

Purpose: prevent `done` from being used as an escape hatch before PR/MR verification.

Cleanup remains a separate human gate. Deleting worktrees or branches should not become automatic phase completion.

## Readiness and Transition Rules

1. `AdvanceIssueOpsPhase` must reject entering a phase if the previous phase is not complete.
2. Existing readiness functions remain source-of-truth checks for detailed missing keys.
3. A new helper computes `IssueOpsPhaseCompletion(record, phase)` from existing fields and returns `ready`, `artifacts`, and `missing`.
4. When a phase is entered, the ledger records `entered_at`.
5. When a phase completion helper is ready, the ledger records `completed_at`, `artifacts`, and clears missing keys for that phase.
6. Re-entering the current phase refreshes that phase's ledger entry, preserving existing idempotent behavior such as AI slop cleanup refresh.
7. Moving backward remains rejected, except existing feedback regression behavior remains explicit and tested.
8. `problem` and `grill` have no transition gate today (the first gate is at `plan`). Rule 1 therefore introduces *new* completion checks for these phases. To preserve the current free `problem` -> `grill` step and early exploration before the remote issue/branch exist: `problem` completion = `intent_contract` present only; `grill` completion = `issue_url` + `branch` + `plan_prep` + `split_decision` + `domain_review` present. `issue_url`/`branch` deliberately gate `plan` entry (grill completion), not `grill` entry, matching the create-issue-after-grill workflow and today's plan-entry gate.
9. Artifacts marked **existing, regated** in [Source-of-Truth Mapping](#source-of-truth-mapping) keep their original downstream gate. The ledger adds an earlier completion index; it does not remove `IssueOpsPlanReadiness`, the `pr` contract-feedback check, or the `done` remote-artifact check.
10. Distinguish phase *entry* from phase *completion*. An artifact produced *during* a phase (the remote PR/MR in `pr`, the cleanup/verification evidence in `ai-slop-clean`) gates that phase's `completed_at`, never its `entered_at`. Gating entry on an artifact the phase itself creates would deadlock the loop.
11. Ledger stamping happens at the single state-write chokepoint (`touchAndWriteIssueOps`), not only inside `AdvanceIssueOpsPhase`. At least four paths mutate `record.Phase` WITHOUT the transition gate — `AddIssueOpsFeedback` (sets phase to feedback, and calls `writeIssueOps` directly), `RecordIssueOpsWorktreeTools` (auto-advances to implement), `force_done`, and `force_release` — so they must route through the same stamping helper to keep the ledger consistent with `record.Phase`. Two consequences: (a) `AddIssueOpsFeedback` and force paths must be migrated to the stamping write path (or call the helper); (b) a phase entered outside `AdvanceIssueOpsPhase` has no genuinely-observed `entered_at`, so it is stamped with the derived sentinel and marked derived, never a fabricated wall-clock time. Force-done/force-release record their resulting phase as forced/derived, not as a genuinely-completed phase.
12. Backward regression and out-of-band refresh: define what happens to *downstream* ledger entries. When feedback regression (rule 7) moves the active phase `pr` -> `feedback`, the now-ahead `pr` (and any later) ledger entries are RETAINED as historical audit but marked stale (`completed_at` kept, a `notes` marker added), never silently treated as still-complete; `IssueOpsPhaseCompletion` for the current phase is recomputed. The from-later-phase ai-slop-clean refresh (`shouldRefreshIssueOpsAISlopClean`, which refreshes the earlier `ai-slop-clean` entry without moving `record.Phase`) updates that earlier entry in place; rule 6 is hereby extended to cover refreshing a non-current phase's entry, not only the current phase.

## Backward Compatibility

Existing IssueOps records without `phase_ledger` remain readable. On read or status rendering, the system may derive a virtual ledger from current fields. On the next state-writing IssueOps command, the written record may include backfilled ledger entries for phases whose artifacts are already complete.

This means old records do not fail merely because `phase_ledger` is absent. They fail only when they try to enter a later phase without the required existing artifacts.

Derived-timestamp determinism: old records have no `entered_at`/`completed_at` for past phases. A virtual or backfilled ledger must NOT invent wall-clock times, because that would create false audit precision and make status output and golden fixtures non-deterministic. Use a stable sentinel (e.g. empty string, or the record's `created_at`/`updated_at` where semantically honest) and mark such entries as derived. Only genuinely-observed transitions get a real timestamp. Because `IssueOpsPhaseLedger` is a `map[IssueOpsPhase]Entry`, any status rendering or JSON comparison must iterate phases in `IssueOpsPhases` order, not Go map order, to stay deterministic.

## CLI and MCP Surface

Status output should include `phase_ledger` when present or derived. `issueops status --json` and MCP `issueops_status` should show:

- current phase
- ledger entry for each known phase
- completed artifacts
- missing artifacts
- the owner command for common missing keys

`issueops phase --to X --json` and MCP `issueops_set_phase` should return errors that name the missing previous phase artifacts before naming later readiness failures. This keeps the agent on the right owner command instead of guessing.

### New recording commands (for net-new fields)

Each net-new artifact needs an owner command so the missing-key → owner-command mapping in status stays complete:

- `domain_review` → `issueops domain-review record` + MCP `issueops_record_domain_review`.
- `cleanup_evidence` / `verification_evidence` → extend the existing AI slop cleanup recording path (e.g. `issueops ai-slop-clean record --category ... --verification ...`) rather than adding a separate command, so cleanup evidence and the timestamp/head/fingerprint are written atomically.
- `feedback_resolution` → extend `issueops feedback` (or add `issueops feedback resolve`) and MCP `issueops_add_feedback`/a new resolve tool.
- `target_branch_match` → no recording command; it is a computed check inside `IssueOpsStrictPRReadiness` once `RemoteArtifact.TargetBranch` is captured by `issueops_verify_remote_artifact`.

## Documentation Surface

Update these docs together with the implementation:

- `.issueops/ARCHITECTURE.md`: state model and phase semantics.
- `.issueops/AGENT_WORKFLOW.md`: phase-by-phase resume contract.
- `skills/issueops/SKILL.md`: artifact matrix and owner commands.
- `skills/issueops/references/worktree-context.md`: clarify `worktree prepare` vs `worktree prepare-tools`.
- MCP tool descriptions and golden contract fixtures: `cmd/issueops/testdata/mcp_tools.golden.json`, `cmd/issueops/testdata/response_contracts.golden.json`, and `cmd/issueops/contractgolden` (new tools/fields change these).
- `.issueops/ADR.md`: record the decision to add net-new source-of-truth fields (`domain_review`, cleanup/verification evidence, feedback resolution, remote-artifact target branch) and the entry-vs-completion gating rule, per the repo's ADR convention.

## Testing

Core tests:

- Existing records without `phase_ledger` remain readable.
- Starting a cycle records or derives a `problem` ledger entry.
- Entering `grill` requires only `intent_contract` (problem completion is minimal); a cycle with intent but no issue_url/branch still enters grill, preserving the current free `problem` -> `grill` step.
- Entering `plan` fails when `grill` artifacts are missing, including `issue_url`, `branch`, the net-new `domain_review`, and a `split_decision` (child link or scope decision).
- Entering `implement` fails when `worktree_path` exists but `worktree_tools` is missing.
- Entering `implement` fails when the `compatibility-review` ledger is incomplete (blockers present or not approved).
- A record with no ledger marshals without a `phase_ledger` key; an entry with only `phase`+`entered_at` omits `completed_at`/`artifacts`/`missing`/`notes` (omitempty round-trip).
- Ledger stays consistent with `record.Phase` across direct-Phase-write paths — `AddIssueOpsFeedback` (pr->feedback), `RecordIssueOpsWorktreeTools` (->implement), force-done, force-release — and a non-`AdvanceIssueOpsPhase` entry gets the derived sentinel, not a wall-clock `entered_at`.
- Feedback regression (pr->feedback) retains the downstream `pr` entry marked stale per rule 12, and re-entering `pr` still works (no deadlock from `feedback_resolution`).
- Deriving a ledger for an old record with several phases of fields (no `phase_ledger`) marks the right phases complete with correct artifacts/missing (multi-phase virtual derivation).
- Running `prepare-tools` records `worktree_tools` and updates the implement ledger artifacts.
- Re-entering AI slop cleanup refreshes cleanup ledger evidence without moving backward.
- `ai-slop-clean` completion requires `cleanup_evidence` and `verification_evidence`, but entering `ai-slop-clean` does not (entry-vs-completion: no deadlock).
- `pr` completion requires `remote_artifact` and `target_branch_match`, but entering `pr` does not require an already-created remote artifact (no deadlock).
- Regated artifacts keep their downstream gate: a record that satisfies the grill ledger for `plan_prep` still fails `IssueOpsPlanReadiness` if `plan_prep` is later cleared; `contract_feedback_issue_update` still blocks `pr`.
- `target_branch_match` fails when `RemoteArtifact.TargetBranch` != `BranchPrepare.BaseBranch`.
- Backfill determinism: deriving a ledger for an old record uses a fixed sentinel (not wall-clock) for `entered_at`/`completed_at`, and re-deriving the same record yields byte-identical ledger output.
- Done remains terminal and still requires verified remote artifact state.

CLI tests:

- `issueops status --json` includes the ledger and missing artifact names.
- `issueops phase --to implement --json` reports missing previous phase artifacts before implementation artifacts.
- `issueops worktree prepare --json` does not mark `worktree_tools` complete.
- `issueops worktree prepare-tools --json` marks `worktree_tools` complete when successful.

MCP tests:

- `issueops_status` exposes the same ledger shape as CLI JSON.
- `issueops_set_phase` preserves the same fail-closed missing artifact behavior.
- Tool schema descriptions mention phase ledger and owner commands.
- Status JSON maps representative missing keys to their owner command (e.g. `domain_review` -> `issueops domain-review record`).
- Skill-contract tests (`internal/core/issueops/issueops_skill_contract_test.go`) are updated for the new readiness keys (`split_decision`, `domain_review`, `target_branch_match`, etc.) and for `plan_prep` being indexed at grill; they must stay green under `go test ./...`.

Verification commands:

```bash
go test ./internal/core/issueops -count=1
go test ./cmd/issueops/issueopscli -count=1
go test ./cmd/issueops/mcpcli -count=1
go test ./... -count=1
go test ./cmd/issueops/... -run Golden -count=1
```

(`go test ./cmd/issueops -run Golden` runs ZERO golden tests — the golden tests live in subpackages `cmd/issueops/contractgolden` and `cmd/issueops/issueopsapp`, so the recursive `./cmd/issueops/...` selector is required.)

## Rollout

Implement the ledger as additive state first. Do not run a destructive migration. After tests pass, run `issueops update --path-mode=skip --json`, then verify the installed surfaces with `issueops daemon status --json`, `codex mcp get issueops`, and `claude mcp list`.
