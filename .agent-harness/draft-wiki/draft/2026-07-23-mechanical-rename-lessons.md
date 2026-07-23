---
title: "Mechanical bulk-rename: regex blind spots and on-disk contract invariants"
source: "session 2026-07-23 V1-suffix removal (commit e8895882)"
target_wiki: "dev-fundamentals"
target_type: "notes"
summary: "A 552-identifier V1-suffix sweep missed identifiers that START with the token (V1Root) because the collection regex required a preceding character; on-disk namespace strings must keep byte-identical values and are now derived from model.IssueOpsSchemaVersion; untracked parallel work (skills/boehm) can fail count-based golden tests, so prove failures against your own diff by parking foreign files."
suggester: "main-agent"
---

# Mechanical bulk-rename: regex blind spots and on-disk contract invariants

Evidence base: commit `e8895882` (refactor(harness): remove V1 suffix naming),
2026-07-23 session. 552 identifiers, 102 files, zero behavior change verified by
`go build`, `go vet`, full `go test ./...`, and a 5-agent adversarial review.

## Lesson 1 — token-collection regex misses token-initial identifiers

`\b[A-Za-z_][A-Za-z0-9_]*V1[A-Za-z0-9_]*\b` requires at least one character
before `V1`, so `V1Root` never entered the rename map even though a special-case
mapping for it existed. The special map is only consulted for identifiers the
collector found. Countermeasure that caught it: an independent post-pass
residual scan (`rg '\bV1[A-Za-z0-9_]*'`) plus adversarial reviewers comparing
the stated rename list against the actual diff. Always run a residual scan with
a *different, broader* pattern than the collector used.

## Lesson 2 — split identifier renames from persisted-string values

On-disk names (`issueops_v1`, `issueops_reset_v1`, `issueops_meta_v1`,
`lease_holder_v1`, `external_intent_v1`) are contracts: renaming the value
orphans existing installs' state. The rename kept every value byte-identical
and instead derived the names from the single version source:
`fmt.Sprintf("issueops_v%d", model.IssueOpsSchemaVersion)` (const → var).
Invariant to preserve: bumping `IssueOpsSchemaVersion` now renames every
namespace at once; literal assertions in `execution_namespace_test.go`,
`reset_legacy_test.go`, and `state_doctor_test.go` pin the current values and
will catch accidental drift. `internal/core/state` keeps hardcoded literals on
purpose — importing the issueops model there would break the layering boundary.

## Lesson 3 — prove test failures against your own diff, not the worktree

`TestResponseContractsGolden` failed with `skills length: got 20, want 19`
because an untracked parallel work product (`skills/boehm/`) sat in the
worktree. The proof pattern: park the foreign untracked path outside the repo,
re-run the failing test (it passed), restore the path, and exclude the foreign
files at staging (hunk-level `git apply --cached` for mixed files like
`ARCHITECTURE.md`). Never absorb or "fix" a parallel change set to make your
own branch green.
