# Issues #329 and #330 Turing report

## Outcome

Codex 0.146 does not serialize `exec_command.workdir` into its stable
PreToolUse payload. `ExecCommandHandler::pre_tool_use_payload` sends only the
command in `tool_input`, while the hook runtime's top-level `cwd` is the turn
cwd. A generated IssueOps mutation requested for a sibling worktree therefore
looked as though it ran from the source checkout and was blocked permanently.

The hook now admits the transport-blind form only when all existing authority
checks pass and the command uses a canonical absolute agent-harness executable,
current generation provenance, current native holder identity, and an exact
`--cwd` equal to the durable worktree. The CLI independently compares the real
process cwd from `os.Getwd()` with that `--cwd` before an owner mutation. Child
commands select their durable lifecycle through `--parent`, not `--id`.

Released-completion `execution sync-base` applies the same two-boundary rule:
the hook validates the generated command and the CLI verifies the process cwd
before apply, finalize, or abort.

## TDD evidence

- RED: a current generated sync-base apply from the source turn cwd was blocked,
  and sync-base mutation accepted a process cwd different from `--cwd`.
- RED: a generated `child start` was rejected because provenance lookup required
  `--id`; after identity lookup, command-only hook authorization still treated
  the generated executable as an external mutation target.
- GREEN: current generated sync-base and owner mutations survive the exact Codex
  command-only payload; bare commands remain blocked from the source turn cwd.
- GREEN: generated owner mutations with mismatched actual process cwd fail before
  the child index changes, while the matching canonical cwd succeeds.

## Verification

- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- focused commandparse, lifecycle, IssueOps CLI, and execution CLI tests
- `go build -o bin/agent-harness ./cmd/harness`
- `./scripts/install-native.sh --json`
- `git diff --check`

The installed source binary and the tested parent-worktree binary both had
SHA-256 `0d4619306b26eab259db4037dfeaacf0e6aafa540c1e04ff1bd4f6d23297c602`.

## Live dogfood

The installed binary executed a provenance-bound `issueops child start` through
the live Codex hook while the turn cwd remained the source checkout and the
actual process cwd was the #228 canonical parent worktree. Generation 28 and
session `019fc065-25e9-7613-9de3-86c8b61b502c` were verified, and the mutation
succeeded as child lifecycle `io-b8b4b1cba1ec` on branch
`330-codex-command-only-hook-workdir`.

Before this change the same owner-control path returned `write_lease_required`.
The #248 released-completion path also reached an actual sync-base apply, entered
the expected merge-conflict state, and then completed an exact governed abort.
Those live results prove both the hook boundary and the CLI process-cwd boundary,
not only the pure parser behavior.

The same Codex transport omission also blocks the separately classified
`atomic-commit-push` Python gates even when their script and repository paths
are absolute. That surface is intentionally not widened by this IssueOps owner
mutation fix and is tracked independently as GitHub issue #331.

## Safety boundary

No generic workdir inference or hook fail-open was added. A bare executable,
non-canonical absolute executable, stale hash or generation, wrong holder,
wrong lifecycle, wrong command `--cwd`, or wrong actual process cwd remains
fail-closed before state or repository mutation.
