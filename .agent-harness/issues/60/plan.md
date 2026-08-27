# #60 Engelbart Canvas Evidence Stop-Hook Plan

**Issue:** https://github.com/m16khb/agent-harness/issues/60
**Cycle:** `io-616e65ef1e6f`
**Base:** `main` at `8acdf260c906d7cf0b2bd93cfd50590904b9d920`
**Scope:** Remove only the assistant-prose fallback from the Engelbart template gate. Keep the current Slack Canvas write parser and create/update semantics.

## Success contract

- No transcript, unreadable transcript, and discussion-only assistant prose produce allow/no-op.
- An incomplete real `slack_create_canvas` remains blocked.
- A complete create passes.
- The latest `slack_update_canvas` can clear an earlier incomplete create and never has to repeat the complete template by itself.
- No connector, evidence schema/scorer, or new prose heuristic is introduced.

## Implementation

1. Add named acceptance tests in `cmd/harness/hookcli/hook_stop_contract_test.go` (reusing helpers from `hook_stop_test.go`) for no transcript, unreadable transcript, discussion-only prose, incomplete create, complete create, latest update, and the decisive mixed-evidence case where an incomplete create plus complete assistant prose must still block.
   - Verify RED: `go test ./cmd/harness/hookcli -run 'TestRunHookStopAllowsEngelbartDiscussionWithoutCanvasWrite|TestRunHookStopAllowsEngelbartDiscussionWithUnreadableTranscript' -count=1` must fail against the current prose fallback.
2. In `cmd/harness/hookcli/hook_stop.go`, make `buildEngelbartCanvasSectionsBlock` return allow when `latestSlackCanvasWrite` has no write evidence. Keep assistant text only for Engelbart context classification; for a create, compute completeness solely from `write.Content`. Preserve parser and latest-update semantics.
3. Run the named GREEN tests and focused package test:
   - `go test ./cmd/harness/hookcli -run 'TestRunHookStop.*Engelbart' -count=1`
   - `go test ./cmd/harness/hookcli -count=1`
4. Build `./bin/agent-harness` in the worker and run both a discussion-only allow probe and an incomplete-create/complete-prose block probe with `hook stop --enforce-engelbart-canvas-sections --json`.
5. Run repository verification:
   - `go test ./... -count=1`
   - `go test -race ./... -count=1`

## Review and publication

- Measure Shannon signal before and after the cleanup pass; keep the production diff to the evidence-boundary branch only.
- Commit with Conventional Commit subject plus Lore body, push the exact issue branch, and open a draft PR to `main` with #60 labels and assignee.
- Do not merge, force-push, delete the branch/worktree, or close the issue.
