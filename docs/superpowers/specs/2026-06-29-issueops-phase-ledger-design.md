# IssueOps Phase Ledger Design

## Problem

IssueOps already has durable fields for many gates, including intent, plan prep, branch preparation, linked worktree, design review, worktree tool preparation, compatibility review, execution decision, AI slop cleanup, feedback, PR readiness, and remote artifact verification. The missing piece is a phase-level ledger that says which phase has entered, which phase has completed, and which concrete artifacts satisfied that phase.

Without that ledger, adjacent steps can be confused. For example, `issueops worktree prepare` only reports the worktree path and next command, while `issueops worktree prepare-tools` records dependency and CodeGraph readiness. Both are legitimate commands, but the current state shape makes it too easy for an agent to treat "worktree exists" as "worktree tools are prepared".

## Goal

Add a backward-compatible `phase_ledger` to IssueOps records and use it as a phase completion index. The ledger must make phase progress resumable, auditable, and fail-closed: a phase cannot be entered until the previous phase's required artifacts are complete, and completion evidence must point back to existing source-of-truth fields rather than creating duplicate truth.

## Non-Goals

- Do not replace the current IssueOps phase list.
- Do not remove or rename existing JSON fields.
- Do not make hooks perform workflow work. Hooks may observe and block deterministic violations only.
- Do not store issue bodies, secrets, raw command output, or provider tokens in the ledger.
- Do not force migration of old state files before they can be read.

## Current Evidence

- `internal/core/issueops/issueops_phase.go` already blocks several phase transitions through readiness checks.
- `internal/core/issueops/issueops_readiness.go` already computes missing keys for plan, compatibility review, implementation, AI slop cleanup, and PR readiness.
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

Add this field to `IssueOpsRecord`:

```go
PhaseLedger IssueOpsPhaseLedger `json:"phase_ledger,omitempty"`
```

The ledger is an index over existing fields. For example, `phase_ledger["implement"].artifacts` may contain `worktree_tools`, but the authoritative data remains `record.WorktreeTools`.

## Artifact Matrix

### problem

Required artifacts:

- `intent_contract`: raw request, interpreted intent, success criteria.
- `issue_url`: linked remote issue or an explicit non-remote waiver recorded as a decision.
- `branch`: issue-number-prefixed IssueOps branch.

Purpose: do not allow investigation and planning to proceed without a concrete problem contract.

### grill

Required artifacts:

- `plan_prep`: prior decisions, related issues, and web research evidence or per-item waive reasons.
- `split_decision`: no-split rationale or provider-native child task evidence.
- `domain_review`: terminology, current model fit, risks, and unresolved uncertainties.

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

- `worktree_tools`: exact linked worktree match, dependency readiness, CodeGraph checked, and CodeGraph ready.
- `execution_decision`: auto-proceed boundaries, hook-blocked work, human gates, and sub-agent decision.
- `implementation_changes`: change fingerprint from the linked worktree.

Purpose: prevent "worktree exists" from being confused with "worktree is prepared", and prevent implementation from starting without the pre-work decision record.

### ai-slop-clean

Required artifacts:

- `ai_slop_clean_at`: cleanup timestamp.
- `ai_slop_clean_head`: Git HEAD at cleanup.
- `ai_slop_clean_fingerprint`: implementation fingerprint at cleanup.
- `cleanup_evidence`: categories checked or cleaned.
- `verification_evidence`: relevant tests or checks rerun after cleanup.

Purpose: do not let cleanup become a verbal claim.

### feedback

Required artifacts:

- `feedback_classification`: every feedback item has a classification.
- `contract_feedback_issue_update`: contract-changing feedback updated the remote issue body.
- `feedback_resolution`: valid defect, question, and noise outcomes are recorded.

Purpose: do not let review, CI, or user feedback disappear before PR readiness.

### pr

Required artifacts:

- `strict_pr_readiness`: strict readiness passes.
- `remote_artifact`: provider, kind, URL, labels, assignees.
- `target_branch_match`: PR/MR target matches `branch_prepare.base_branch`.

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

## Backward Compatibility

Existing IssueOps records without `phase_ledger` remain readable. On read or status rendering, the system may derive a virtual ledger from current fields. On the next state-writing IssueOps command, the written record may include backfilled ledger entries for phases whose artifacts are already complete.

This means old records do not fail merely because `phase_ledger` is absent. They fail only when they try to enter a later phase without the required existing artifacts.

## CLI and MCP Surface

Status output should include `phase_ledger` when present or derived. `issueops status --json` and MCP `issueops_status` should show:

- current phase
- ledger entry for each known phase
- completed artifacts
- missing artifacts
- the owner command for common missing keys

`issueops phase --to X --json` and MCP `issueops_set_phase` should return errors that name the missing previous phase artifacts before naming later readiness failures. This keeps the agent on the right owner command instead of guessing.

## Documentation Surface

Update these docs together with the implementation:

- `.agent-harness/ARCHITECTURE.md`: state model and phase semantics.
- `.agent-harness/AGENT_WORKFLOW.md`: phase-by-phase resume contract.
- `skills/issueops/SKILL.md`: artifact matrix and owner commands.
- `skills/issueops/references/worktree-context.md`: clarify `worktree prepare` vs `worktree prepare-tools`.
- MCP tool descriptions and golden contract fixtures.

## Testing

Core tests:

- Existing records without `phase_ledger` remain readable.
- Starting a cycle records or derives a `problem` ledger entry.
- Entering `plan` fails when `grill` artifacts are missing.
- Entering `implement` fails when `worktree_path` exists but `worktree_tools` is missing.
- Running `prepare-tools` records `worktree_tools` and updates the implement ledger artifacts.
- Re-entering AI slop cleanup refreshes cleanup ledger evidence without moving backward.
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

Verification commands:

```bash
go test ./internal/core/issueops -count=1
go test ./cmd/harness/issueopscli -count=1
go test ./cmd/harness/mcpcli -count=1
go test ./... -count=1
go test ./cmd/harness -run Golden -count=1
```

## Rollout

Implement the ledger as additive state first. Do not run a destructive migration. After tests pass, run `agent-harness update --path-mode=skip --json`, then verify the installed surfaces with `agent-harness daemon status --json`, `codex mcp get agent_harness`, and `claude mcp list`.
