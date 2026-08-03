# Issue #288 Turing report

## Outcome

The installed Orca CLI was present and usable. A long-lived host process had
inherited an obsolete `ORCA_RELAY_DIR`, `ORCA_RELAY_SOCKET_PATH`, and
`ORCA_RELAY_CREDENTIAL_FILE`, so agent-harness kept contacting an ownerless
relay instead of the relay selected by the current Orca wrapper.

`ExecRunner` now preserves a responsive inherited relay. If and only if the
first command fails with Orca's ownerless-relay diagnostic and all three local
relay selectors were inherited, it retries once with those selectors removed.
The retry preserves the node selector, remote environment, pairing code, and
all unrelated environment entries.

## TDD evidence

- RED: the unconditional-filter implementation produced only `current` for the
  retry case and removed the responsive inherited relay.
- GREEN: the ownerless case records `inherited` then `current`; the responsive
  case records only `inherited`.
- No credential contents are parsed, copied, logged, or included in errors.

## Verification

- `go test ./internal/adapter/orca -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- CLI and response-contract golden tests
- `go build -o bin/agent-harness ./cmd/harness`
- `git diff --check`

The first regular full-suite run overlapped the race suite and one unrelated
five-second webfetch comparator subprocess was killed. Its isolated test passed
in 0.14 seconds, and the sequential regular full suite then passed.

## Dogfood status

The current wrapper relay is reachable and owns the #237 Orca terminal. The
first #237 claim reached that owner and exposed independent issue #287: its
98,163-byte sealed plan exceeded a claim-only 64 KiB reader even though the
producer and other readers allow 1 MiB. Issue #287 was fixed and merged in PR
#289 by unifying the owner-artifact reader limit at the shared 1 MiB contract.

The live acceptance check then succeeded without restarting this coordinator:
#237 claimed generation 1 as run `run_8401e427602d`, task
`task_72288405815a`, and dispatch `ctx_a4ec921cd2ac` on the current relay. Its
owner passed planning review, entered implementation, produced the expected
architecture RED test, and began the source move. This demonstrates that an
installed Orca CLI is used after the bounded stale-relay recovery and that the
original long-lived host environment no longer prevents IssueOps dispatch.
