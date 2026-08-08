# Issue #333 Turing report

## Outcome

The linked-worktree install contract fixture now models the activation protocol
used by `install-native.sh`. Its fake runtime emits a valid begin receipt bound
to the candidate binary digest, seals that same transition after installation,
and returns an explicit abort receipt on rollback.

This repairs an integration-only failure exposed while rebasing #248 onto the
latest #228 parent. The production activation path remains fail-closed; only the
test double was brought up to the current receipt contract.

## TDD evidence

- RED: `TestInstallNativeScriptActivatesLinkedWorktreeBuildAtStableSource`
  failed while decoding the empty begin-activation output from the fake runtime.
- GREEN: the focused linked-worktree activation test passes with the receipt-aware
  fixture.
- GREEN: the full `internal/adapter` package and repository test suites pass.
- GREEN: the focused race test passes.

## Verification

- `go test ./internal/adapter -run TestInstallNativeScriptActivatesLinkedWorktreeBuildAtStableSource -count=1`
- `go test ./internal/adapter -count=1`
- `go test -race ./internal/adapter -run TestInstallNativeScriptActivatesLinkedWorktreeBuildAtStableSource -count=1`
- `go test ./... -count=1`
- `git diff --check`

## Live dogfood

IssueOps created and verified GitHub issue #333, a linked branch pinned to the
sealed #228 base, the canonical child worktree, and draft PR #335. The child
bootstrap was performed through the provenance-bound parent handoff introduced
by #334, then ownership transferred to the child as the sole active writer.

## Safety boundary

The fixture uses a deterministic transition identifier only inside its isolated
temporary runtime. It still computes the real candidate binary SHA-256, and the
seal receipt must match both that digest and the begin transition. No production
receipt validation, activation, rollback, or native installation code changed.
