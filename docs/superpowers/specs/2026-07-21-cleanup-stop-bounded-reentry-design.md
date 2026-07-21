# Cleanup Stop Bounded Re-entry Design

## Problem

`runHookStop` evaluates `cleanup_pending_human_decision` before the existing
`stop_hook_active` and `자동진행하지 않음` loop guards. The first cleanup Stop
correctly asks the source session to present the three human cleanup choices,
but every continuation Stop sees the unchanged durable cleanup state and blocks
again. The host therefore re-enters the agent without a terminating branch.

The cleanup state must remain pending until a human authorizes a disposition.
The defect is treating that unchanged state as authority to relay the same Stop
decision indefinitely.

## Required behavior

- A fresh source-session Stop with pending ownership cleanup blocks once with
  the existing three-choice instruction.
- A continuation carrying `stop_hook_active=true` returns the host no-op payload.
- A response beginning with the existing no-auto-proceed judgement returns the
  host no-op payload even on the first observed cleanup Stop. Explicit user
  termination outranks unchanged durable cleanup state; the next ordinary user
  turn may surface cleanup again.
- A later independent user turn may surface the pending cleanup once again.
- Stop remains read-only and does not mutate IssueOps, Git, Orca, or provider state.
- When only human input remains, the agent ends its turn instead of invoking an
  agent/background wait primitive.

## Design

Keep `OwnershipCleanupHumanGate` unchanged and read-only. Treat pending cleanup
as one explicit branch so a suppressed cleanup relay cannot fall through into
the independent next-action relay below it:

```go
if cleanupPending {
    if stopHookActive || noAutoProceedJudgement {
        return printJSON(ho.FormatNoop())
    }
    // existing cleanup block response
}
```

The immediate no-op is required because a continuation may contain the three
cleanup choices and enable `--relay-next-action-judgement`; ordinary fallthrough
would let that separate branch re-block. This adds no persistent acknowledgement
or schema migration.

## Rejected alternatives

1. Refactor every Stop gate behind a new bounded-relay helper. This widens the
   regression surface beyond the ordering defect.
2. Persist a cleanup relay acknowledgement per cycle. This needs migration and
   consumption semantics and could hide cleanup on later real user turns.

## Constitutional invariant

Every agent, hook, relay, retry, monitor, or orchestration loop must have a
finite bound and an explicit success, failure, cancellation, timeout, or no-op
exit. A user's explicit stop/no-auto-proceed judgement takes precedence over an
unchanged durable state. Human-input waits end the current turn; they do not
justify background-agent waiting.

## Verification

- Initial cleanup Stop still blocks with the exact human-choice reason.
- `stop_hook_active` continuation with the relay flag and three choices no-ops.
- A first-contact no-auto-proceed response no-ops, followed by an ordinary fresh
  Stop that still blocks.
- A later fresh Stop can block once again.
- The persisted IssueOps record is unchanged by all Stop calls.
- Focused hook tests, `go test ./... -count=1`, and
  `go test -race ./... -count=1` pass.
