# Issue #288 — stale Orca relay environment recovery

## Objective

Prevent a long-lived Codex or Claude host session from pinning agent-harness
Orca subprocesses to an obsolete local relay after the installed Orca wrapper
has moved to a newer owning relay.

## Verified failure

- The host inherited `ORCA_RELAY_DIR`, `ORCA_RELAY_SOCKET_PATH`, and
  `ORCA_RELAY_CREDENTIAL_FILE` for relay `e424c834f143`.
- The installed `~/.orca-relay/bin/orca` wrapper now defaults to relay
  `576ecd9bd42d`.
- The inherited relay handshakes but has no owning client.
- Removing only the three inherited local relay selectors reaches the current
  wrapper relay, where the runtime, graph, and #237 owner terminal are ready.

## Design

`internal/adapter/orca.ExecRunner` is the process boundary for every Orca CLI
call. It first preserves the inherited environment so a responsive pinned relay
continues to work. Only when Orca returns the exact ownerless-relay diagnostic
will it retry once with the three local relay selector variables removed. The
installed wrapper therefore owns recovery relay selection without changing the
successful path.

The recovery retry continues to preserve:

- `ORCA_RELAY_NODE_PATH`, which selects the node executable;
- `ORCA_ENVIRONMENT` and `ORCA_PAIRING_CODE`, which select an explicit remote
  runtime through Orca's supported public surface;
- all unrelated host environment entries.

The adapter will not parse the wrapper, inspect credentials, copy Orca state,
or reproduce relay behavior.

## TDD tasks

1. Run real child processes proving an ownerless inherited relay retries through
   the current wrapper while a responsive inherited relay remains selected
   (RED).
2. Retry exactly once on the ownerless-relay diagnostic, filtering only the
   three local relay selector names on the retry (GREEN).
3. Run the Orca adapter suite, architecture tests, full tests, race tests, vet,
   build, and contract goldens.
4. Build the exact child head without user-scope installation and prove #237
   resume reaches the owning relay and creates a new dispatch.
5. Review, publish a PR to the #228 branch, merge, reflect completion, and
   clean the child lifecycle/worktree/branches.

## Rollback

Revert the single runner environment-boundary commit. No Orca credential,
runtime state, user hook configuration, or record schema is migrated.
