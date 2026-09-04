# IssueOps Phase Ledger — Implementation Impact

Companion to `2026-06-29-issueops-phase-ledger-design.md`. Lists the concrete Go
files, tests, and golden fixtures that change, grounded in the current tree
(`internal/core/issueops`, `cmd/issueops`). Ordered by dependency so each phase
compiles and tests green before the next.

## Layer 1 — Model (`internal/core/issueops/model/`)

| File | Change |
| --- | --- |
| `types.go` | Add `IssueOpsPhaseLedgerEntry`, `IssueOpsPhaseLedger`. Add to `IssueOpsRecord`: `PhaseLedger`, `DomainReview *IssueOpsDomainReview`, `AISlopCleanCategories []string`, `AISlopCleanVerification []string`. Add `IssueOpsDomainReview` type. Add `Resolution string` to `IssueOpsFeedbackItem`. Add `TargetBranch string` to `IssueOpsRemoteArtifactVerification`. |
| `phase.go` | No struct change. `IssueOpsPhases` is the canonical order to iterate the ledger map deterministically. |

All additive + `omitempty` → old JSON round-trips unchanged. No `phase.go` rank changes (phase list is unchanged per Non-Goals).

## Layer 2 — Core logic (`internal/core/issueops/`)

| File | Change |
| --- | --- |
| **new** `issueops_phase_ledger.go` | `IssueOpsPhaseCompletion(record, phase) (ready bool, artifacts, missing []string)` reading existing source-of-truth per the mapping table. Ledger derivation/backfill (`deriveIssueOpsPhaseLedger`) using a stable sentinel, never wall-clock. Deterministic iteration via `model.IssueOpsPhases`. |
| `issueops_readiness.go` | Add `IssueOpsProblemReadiness` (`intent_contract` only — keeps problem->grill free) and `IssueOpsGrillReadiness` (`issue_url` + `branch` + `plan_prep` + `split_decision` + `domain_review`). Add `split_decision` helper reading `IssueLinks`/`Decisions(kind=scope)`. Keep the trivial-class `plan_prep` carve-out (`planPrepGateApplies`) in the grill check. |
| `issueops_phase.go` | In `advanceIssueOpsPhaseLocked`: add `grill`-entry gate (problem complete) and `plan`-entry now also asserts grill complete. Record `entered_at` on transition and `completed_at` via `IssueOpsPhaseCompletion`. Preserve idempotent ai-slop refresh, backward-rejection, feedback regression. Apply entry-vs-completion rule (rule 10) for `pr`/`ai-slop-clean`; apply rule 12 (stale downstream entries) on the feedback regression. |
| `issueops_state.go` | Add the single ledger-stamping helper invoked by `touchAndWriteIssueOps` (rule 11), so direct-Phase-write paths cannot desync the ledger. |
| `issueops_feedback.go` / `issueops_force_done.go` / `issueops_force_release.go` / `RecordIssueOpsWorktreeTools` (`package.go`) | Route their direct `record.Phase` mutations through the stamping write path; force paths mark the entry forced/derived (rule 11). |
| `issueops_pr_readiness_strict.go` | Add `target_branch_match`: compare `record.RemoteArtifact.TargetBranch` to `record.BranchPrepare.BaseBranch` (only when a remote artifact exists, so `pr` entry doesn't deadlock). |
| `issueops_feedback.go` | Add resolution recording (`Feedback[].Resolution`); keep `MarkIssueOpsContractFeedbackIssueUpdated`. |
| **new** `issueops_domain_review.go` | `RecordIssueOpsDomainReview(stateRoot, id, req)` writing `record.DomainReview` (mirrors `intentdesign`/compatibility recorders). |
| AI slop clean recording path | Extend whatever writes `AISlopClean*` (entry in `issueops_phase.go` + any cleanup command core) to also persist `AISlopCleanCategories`/`AISlopCleanVerification` atomically. |
| `package.go` | Re-export new types/functions if they cross the package boundary like existing ones. |

## Layer 3 — Facade (`internal/core/issueops_facade.go`)

Mirror the existing `RecordIssueOps*` wrappers (see `RecordIssueOpsCompatibilityReview` at :141, `AddIssueOpsFeedback` at :197):

- `RecordIssueOpsDomainReview`
- `RecordIssueOpsAISlopCleanEvidence` (or extend the existing cleanup entry point)
- `ResolveIssueOpsFeedback`
- `IssueOpsPhaseCompletion` exposure for CLI/MCP status

## Layer 4 — CLI (`cmd/issueops/issueopscli/`)

| File | Change |
| --- | --- |
| `issueops.go` | Register new subcommands in the dispatch map (alongside `"plan-prep": runIssueOpsPlanPrep` at :24): `domain-review`, `feedback resolve`, `ai-slop-clean record`. |
| **new** `issueops_domain_review.go` (cli) | `runIssueOpsDomainReview` flagset (mirror `issueops_plan_prep.go`). |
| `issueops_intent_design.go` / cleanup CLI | Add the cleanup-evidence flags to the ai-slop-clean path. |
| `feedbackcleanup/` | Add `--resolution` recording. |
| `issueops_cli_support.go` | Update usage text. |
| status command | Render `phase_ledger` (per-phase entry, completed/missing artifacts, owner command). |

## Layer 5 — MCP (`cmd/issueops/mcpcli/`)

| File | Change |
| --- | --- |
| `mcp_tool_issueops.go` | Add to the handler registry map (alongside `"issueops_record_compatibility_review"` at :27): `issueops_record_domain_review`, a feedback-resolve tool, and ai-slop-clean evidence args. |
| `mcp_tool_issueops_handlers.go` | New handlers; extend `handleMCPIssueOpsStatus` to surface the ledger and `handleMCPIssueOpsVerifyRemoteArtifact` to capture `target_branch`. |
| tool schema/descriptions | Mention phase ledger + owner commands (drives golden fixtures). |

## Layer 6 — Golden fixtures & tests

| Target | Change |
| --- | --- |
| `cmd/issueops/testdata/mcp_tools.golden.json` | New tools + descriptions. |
| `cmd/issueops/testdata/response_contracts.golden.json` | New record fields in contracts. |
| `cmd/issueops/contractgolden/` | Contract shape for new fields/tools. |
| `internal/core/issueops/*_test.go` | New core tests (see spec Testing): problem/grill gates, entry-vs-completion no-deadlock, regated downstream gates, target_branch_match, backfill determinism. |
| `cmd/issueops/issueopscli/*_test.go` | CLI ledger/status + new subcommands. |
| `cmd/issueops/mcpcli/issueops/*_test.go` | MCP ledger parity + new tools. |

Regenerate golden: `go test ./cmd/issueops -run Golden -count=1` (with the repo's update flag if present).

## Layer 7 — Docs

`.issueops/ARCHITECTURE.md`, `AGENT_WORKFLOW.md`, `ADR.md`; `skills/issueops/SKILL.md`, `skills/issueops/references/worktree-context.md`.

## Recommended order

1. **Model** (Layer 1) — fields only; everything compiles.
2. **Completion helper + derivation** (`issueops_phase_ledger.go`) with unit tests, no transition wiring yet.
3. **New recorders** (domain-review, cleanup evidence, feedback resolution, remote target branch) + facade + CLI/MCP, each with tests. These create the source-of-truth the gates will require.
4. **Transition wiring** in `issueops_phase.go` (problem/grill gates, entered_at/completed_at, entry-vs-completion). This is the fail-closed step — land it only after step 3 so cycles can satisfy the new gates.
5. **Strict PR** `target_branch_match`.
6. **Status rendering** + golden regeneration.
7. **Docs + ADR**, then `issueops update --path-mode=skip --json` and surface verification per the spec Rollout.

Steps 3→4 ordering matters: wiring the grill/problem gates before the recorders exist would block every new cycle. Land recorders first, gates last.

## Risk notes

- **Deadlock**: gating `pr`/`ai-slop-clean` *entry* on artifacts they produce. Mitigation: rule 10 (entry vs completion).
- **Determinism**: map iteration + derived timestamps. Mitigation: iterate `IssueOpsPhases`, sentinel timestamps.
- **Double gate drift**: regated artifacts must keep both the ledger index and the original downstream check, or strictness silently drops.
