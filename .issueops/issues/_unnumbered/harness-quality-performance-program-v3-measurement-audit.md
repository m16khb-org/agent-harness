# Agent-Harness Quality/Performance v3 Measurement Audit

Date: 2026-07-03
Worktree: `/Users/sample/workspace/issueops.worktrees/1-quality-performance-program-v3`
Binary: `./bin/issueops`

This note records the live measurement evidence gathered after the v3 changes.
It does not change the source plan's scope or mark user-decision items as
complete.

Source plan note: this worktree now includes
`.issueops/issues/_unnumbered/harness-quality-performance-program-v3.md`, copied
byte-for-byte from the original untracked plan in the main checkout. This file
records the execution audit for the implementation branch.

## P-1/P-2 Hook Metrics

Command:

```bash
zsh -lc 'measure session-start ./bin/issueops hook session-start --repo "$PWD" --host codex --json'
```

Result:

```text
session-start count=10 min=75.2 p50=94.5 max=202.9
```

State evidence:

```text
hook metric temp files: 0
hook-metrics.jsonl: 10016 lines, 853510 bytes
```

Decision: P-1/P-2 pass the v3 success check for session-start p50 under 100ms
and stale temp cleanup.

## P-3 Doc-Upkeep Queue

Current worktree lifecycle state:

```text
repo_id: 2796cc7fbf615b7d1165c183
queue: /Users/sample/.local/state/issueops/projects/2796cc7fbf615b7d1165c183/doc-upkeep-queue.jsonl
queue status: missing
```

Main checkout lifecycle state, where the original large queue still exists:

```text
repo: /Users/sample/workspace/issueops
repo_id: feee6730cfd6453a2cb60ac7
queue: /Users/sample/.local/state/issueops/projects/feee6730cfd6453a2cb60ac7/doc-upkeep-queue.jsonl
queue size: 3618 lines, 6196039 bytes
status counts: 3618 pending
```

Hook read latency on the main checkout with the new binary:

```text
main-user-prompt count=10 min=19.0 p50=20.6 max=53.7
main-post-tool-use count=10 min=9.8 p50=10.3 max=11.0
main-stop count=10 min=19.1 p50=19.3 max=20.8
```

Decision: P-3 latency is no longer a visible hook-path bottleneck, but the v3
"queue file size bound" success check is not proven. The current compaction
handles resolved events; the main checkout queue is all pending, so a separate
retention policy is required before shrinking it.

## Remaining Policy Boundary

The unresolved P-3 decision is how to bound a pending-only doc-upkeep queue
without losing project-document upkeep intent. That is a user-visible retention
policy, not a mechanical cleanup.

Judgment: split pending-only queue retention into a follow-up decision instead
of implementing it in this branch. The current code safely removes resolved and
malformed queue entries, but pending entries are the durable "docs still need
attention" signal. A mechanical age/count cap would silently discard that
intent.

Follow-up policy choices to decide before implementation:

- convert stale pending events into an explicit archived/superseded state after
  a review command confirms they are no longer actionable;
- collapse pending events by target doc and normalized summary hash while
  preserving the newest evidence;
- require an explicit operator command for destructive pending retention rather
  than pruning from hook read paths.

This keeps P-3 partially complete: hook-path latency is bounded by the resolved
compaction and relay optimizations, but the source plan's "queue file size
bound" criterion remains open for pending-only queues. The follow-up
implementation plan is
`.issueops/issues/_unnumbered/p3-doc-upkeep-pending-retention-policy.md`.

## P-6 Cold Start

Current `regexp.MustCompile` count:

```text
cmd/internal Go files: 56
```

Current binary timing, 30 runs each:

```text
help avg=17.7ms min=10.5ms max=69.5ms
hook-session-start avg=70.8ms min=55.0ms max=109.1ms
hook-pre-tool-use avg=14.3ms min=11.6ms max=23.5ms
hook-stop avg=14.7ms min=10.6ms max=24.4ms
```

Temporary stripped binary check:

```text
./bin/issueops: 17364354 bytes
-ldflags="-s -w": 12149698 bytes
stripped-help avg=73.3ms min=10.8ms max=502.3ms
stripped-hook-session-start avg=92.5ms min=64.5ms max=223.9ms
stripped-hook-pre-tool-use avg=15.2ms min=11.8ms max=30.1ms
stripped-hook-stop avg=32.0ms min=11.2ms max=236.6ms
```

Decision: defer implementation. The stripped binary reduces artifact size but
does not show a hook-latency win, and `pre-tool-use`/`stop` are already near
15ms average. P-6 should not be implemented without a narrower proof that lazy
initialization removes measurable hot-path startup cost.

## P-5 Relay Stat Skip

Change: `post-tool-use` now checks for
`stop-next-action-relay.json` before entering the lifecycle profile validation
and clear path. Missing relay files return `no_next_action_relay` without
reading `project.json`; existing or uncertain relay paths still use the existing
validated clear path.

TDD evidence:

```text
go test ./internal/core/lifecycle -run 'TestClearStopNextActionRelayIfPresentSkipsProfileReadWhenRelayMissing|TestRecordStopNextActionRelaySuppressesWhenStateCannotPersist' -count=1
go test ./cmd/issueops/hookcli -run 'TestRunHookPostToolUseClearsSuppressedNextActionRelay|TestRunHookStopRelaysSameNextActionChoicesOnlyOnce|TestRunHookPostToolUseEmitsCodexCompatibleNoopJSON' -count=1
```

## Q-3 State Retention Policy

Change: observation-style state records now have same-prefix retention at the
write sites that create unbounded history:

- `external-llm-usage-*`: after a successful usage observation write, prune
  same-prefix records older than 30 days and cap retained same-prefix records at
  10,000.
- `self-augment-lesson-*`: after a successful lesson write, prune same-prefix
  records older than 30 days and cap retained same-prefix records at 10,000.

The prune is best-effort after a successful write, so recording and lesson save
semantics remain unchanged when state cleanup fails. It also filters by key
prefix, so unrelated state and the still-unresolved P-3 doc-upkeep pending queue
are not affected.

TDD evidence:

```text
go test ./internal/core/state -run TestStatePrunePrefixAppliesAgeAndCountOnlyToMatchingKeys -count=1
go test ./internal/core -run TestExternalLLMUsagePrunesOldUsageRecords -count=1
go test ./cmd/issueops/selfworkflow/augmentlesson -run TestSaveSelfAugmentLessonPrunesOldLessonRecords -count=1
go test ./internal/core ./internal/core/state ./cmd/issueops/selfworkflow/augmentlesson ./cmd/issueops/selfworkflow/augmentplan -count=1
```

## T-6 ID Helper Design

Decision: do not migrate persisted key formats in this branch. Existing
observable prefixes (`external-llm-usage-*`, `self-augment-lesson-*`,
`job-*`, `audit-*`) remain backwards-compatible.

Future ID/key work should introduce one small core helper package, tentatively
`internal/core/idkey`, with these constraints:

- accept `time.Time` from the caller instead of calling `time.Now()` internally,
  so tests can prove collision and ordering behavior deterministically;
- preserve caller-owned visible prefixes and existing persisted key shape when
  retrofitting old surfaces;
- add a deterministic hash or counter suffix for uniqueness instead of relying
  on timestamp-only keys;
- migrate one call site at a time with behavior tests, not a broad state-key
  rewrite.

Q-3 only centralizes local prefixes and retention constants. It intentionally
does not introduce the helper yet because that would broaden the persisted-state
surface beyond the confirmed audit/design step.

## Execution Status Checkpoint

Fresh verification on this worktree:

```text
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go run ./cmd/issueops quality inspect --json
adapter/core/port cmd imports: 0
old remoteparse import: 0
go test ./internal/core/externalllm ./internal/core/issueops ./cmd/issueops/selfworkflow/augmentplan ./internal/core/qualitycatalog ./cmd/issueops/qualitycli ./internal/adapter/gjc ./cmd/issueops/webfetchcli ./cmd/issueops/issueopscli/feedbackcleanup ./cmd/issueops/mcpcli ./internal/adapter/provider/... ./cmd/issueops/issueopscli/remoteverify ./cmd/issueops/issueopscli/remotecmd ./internal/core/lifecycle ./cmd/issueops/hookcli -count=1
go test -race ./internal/core/... -count=1
go build -o bin/issueops ./cmd/issueops
git diff --check
```

Current status against the source plan:

| Item | Status | Evidence / remaining boundary |
|------|--------|--------------------------------|
| T-1 | Done | `TestResponseContractsGolden` passes on current HEAD. |
| P-1/P-2/P-4 | Done | Hook metrics bounded prune, stale tmp sweep, and locking tests pass; session-start p50 recorded under 100ms. |
| P-3 | Partial / follow-up split | Resolved-event compaction and locking are implemented; pending-only doc-upkeep queue retention is intentionally split out because pending entries preserve unresolved documentation intent. |
| A-1/A-2/A-3 | Done | old remoteparse import 0; adapter/core/port cmd imports 0; provider helper tests pass. |
| T-2 | Done | CI adds `go test -race ./internal/core/... -count=1`; fresh race run passes. |
| Q-1/Q-2/Q-4 | Done | Focused externalllm, augmentplan, and issueops tests pass. |
| QC-SYNC/T-3/T-4/T-5 | Done | `quality inspect` reports six candidates and no facade low-coverage package; focused tests pass. |
| Q-3 | Done | Usage and lesson state records prune same-prefix records older than 30 days and cap same-prefix retention at 10,000; focused TDD and related package tests pass. |
| T-6 | Design fixed | No persisted key migration in this branch; future helper should accept injected time, preserve visible prefixes, and migrate one call site at a time. |
| P-6/P-7/P-8 | Deferred | P-6 measured as not worth implementing now; P-7/P-8 remain optional P3 follow-ups. |
| P-5 | Done | Relay stat skip TDD and hook relay regression tests pass. |
