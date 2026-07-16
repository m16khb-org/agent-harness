# Issue #22 coordinator bootstrap recovery plan

## Scope

Fix the initial coordinator seal path when multiple `coordinator_preparing`
cycles share one source checkout. The fix is limited to lifecycle hook guidance
and its focused regression tests.

## Invariant

Only an exact cycle ID, the exact source checkout, and a concrete coordinator
terminal handle may request bootstrap guidance. The hook supplies the native
host/session/agent tuple from its authenticated event; no agent guesses it.
All malformed, cross-cycle, cross-root, or post-seal requests remain denied.

## Verification

Run the focused lifecycle authority/guard tests and the affected IssueOps
handoff-start regression. Re-run an Orca coordinator preview using the exact
cycle ID and current coordinator terminal handle before dispatching a worker.
