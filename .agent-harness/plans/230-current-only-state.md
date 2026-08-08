# #230 Current-Only State Execution Plan

## Execution identity

- Remote issue: https://github.com/m16khb/agent-harness/issues/230
- Parent cycle: `io-c26802f00c2b`
- Child cycle: `io-8dcba592790b`
- Branch: `230-current-only-state`
- Base branch: `228-clean-break-hexagonal-architecture`
- Base SHA: `c6d82ea884664ae4858c91250c3935ab60c477e8`
- Canonical worktree: `/Users/m16khb/Workspace/agent-harness.worktrees/230-current-only-state`
- Parent plan: `.agent-harness/plans/228-clean-break-hexagonal-architecture.md`, Task 3 and the Normative Child Delivery Epilogue

## Bounded scope

Remove the `reset-legacy` and `state migrate` production surfaces, pre-v1 schema promotion, and version-specific unsupported-state errors. Preserve exact schema-v1 read/write, absent-record not-found behavior, and the current native activation service delivered by Task 1. Do not widen into unrelated provider behavior or another architecture child.

## Acceptance criteria

1. Production `reset-legacy`, `reset_legacy*.go`, `state migrate`, and `StateMigrate` surfaces are absent.
2. Existing records with missing/zero/future schema, malformed JSON, legacy authority fields, key mismatch, or byte mismatch return `state.ErrInvalidState` with public text `invalid state`.
3. Absent records retain their existing not-found identity; exact schema-v1 read/write and current native activation remain green.
4. Focused, race, golden, full-suite, architecture, and exact-head Codex/Claude child-smoke gates pass.

## TDD sequence

1. Characterize the preserved exact-v1 and absent-record behavior.
2. Add the invalid-record matrix and observe RED for schema-zero promotion and version/parser detail leakage.
3. Make decoders exact-v1 and project all existing-invalid cases to the single sentinel.
4. Remove reset/migrate handlers, router entries, usage, validation, rerun suggestions, tests, and obsolete error types.
5. Run the exact absence scan and focused gates, then the ordered full verification gate.

## Verification and evidence

- `scripts/verify-go-test-match.sh --run 'InvalidMatrix|Absent' --expect 'InvalidMatrix|Absent' -- ./internal/contract/state ./internal/contract/issueopslease`
- `! rg -n 'reset-legacy|PreviewLegacyReset|ConfirmLegacyReset|StateMigrate|UnsupportedSchemaError|unsupported_schema|unsupported (state|issueops) schema' cmd internal --glob '*.go'`
- `go test ./internal/contract/state ./internal/contract/issueopslease ./cmd/harness/statecli ./cmd/harness/issueopscli ./cmd/harness/mcpcli -count=1`
- `go test -race ./internal/application/nativeactivation ./internal/contract/issueopslease -count=1`
- `scripts/verify-go-test-match.sh --run 'Golden' --expect 'Golden' -- ./cmd/harness/contractgolden ./cmd/harness/harnessapp`
- Full repository gates and the private exact-head dual-host receipt required by the parent plan.

Turing evidence is written under `.agent-harness/turing/evidence/230-current-only-state/`; no runtime process is spawned by the unit-test matrix, so its cleanup receipt is `none spawned`.

## Completion and cleanup boundary

Commit and push only the verified child diff, create a Korean draft PR from `230-current-only-state` to `228-clean-break-hexagonal-architecture`, and record the exact final HEAD and evidence in child IssueOps state. Merge only after green provider checks, unconditional implementation review, and the exact-head dual-host smoke receipt. After verified merge, close and clean only child #230; do not close parent #228 or delete the parent worktree/branch.
