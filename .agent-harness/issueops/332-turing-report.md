# Issue #332 Turing report

## Outcome

Released-completion sync-base conflicts now receive a durable, bounded
resolution authority instead of reopening a general write lease. The receipt
binds the current lease and completion generations, base OID, PID-safe native
actor, exact conflict paths, and start time.

The lifecycle guard admits mutations only when every resolved target belongs to
that sealed conflict set and the current process ancestry matches the receipt.
Finalize appends its sync-base event and removes the receipt in the same record
write. Abort withdraws the Git merge and removes the receipt.

## TDD evidence

- RED: a live governed apply reached 12 conflicts, but the first conflict
  `apply_patch` was blocked with `lease_released`.
- GREEN: a released conflict apply persists one resolution receipt; the sealed
  actor can edit the named conflict while an unrelated path and foreign session
  remain blocked.
- GREEN: finalize and abort remove the receipt; a foreign finalize actor is
  rejected before Git commit or push.
- GREEN: stable-v1 strict decoding and preparation snapshots retain the new
  receipt without aliasing its actor receipt or conflict slice.

## Verification

- focused contract, IssueOps core, and lifecycle tests
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/agent-harness ./cmd/harness`
- `./scripts/install-native.sh --json`

## Live dogfood

For `io-268bd6ac6e7a` generation 5, the installed binary applied parent
`eb94a5ebc2dd79a14d2b072f2b8ebef857dc6102`, sealed the predicted 12-file
conflict set, and allowed the exact current Codex session to patch
`internal/adapter/mcp/issueops_catalog.go`. The same patch had previously been
blocked. An exact governed abort then restored the clean child branch and
removed the temporary authority before the #332 parent commit.

## Safety boundary

No generic released writer, directory-wide exception, or source-cwd inference
was added. The receipt is valid only for one current completion and one native
process identity, and only exact conflict targets inside the canonical worktree
are mutable.
