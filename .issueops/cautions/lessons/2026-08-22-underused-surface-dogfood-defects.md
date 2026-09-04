---
name: cautions/lessons/2026-08-22-underused-surface-dogfood-defects.md
description: Dated lesson — dogfooding underused surfaces (web-fetch benchmark, hook help, mcpsmoke) found one shipped panic, one shipped data race, and two telemetry-noise bugs; contracts recorded.
---

# 2026-08-22 — underused-surface dogfooding: shipped panic, data race, and telemetry noise

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: dogfooding every remaining CLI surface end to end (web-fetch fetch/benchmark, hook user-prompt/pre-tool-use/post-tool-use/pre-compact/post-compact/session-start/stop, install --project-local, state maintain/doctor, loop flows) during the 2026-08-22 self-augmentation session.
- Summary: four real defects shipped in rarely-exercised paths; all fixed same day. The recurring pattern: underused surfaces keep hiding user-visible defects, so dogfooding them is the highest-yield defect discovery vector.

## 1. web-fetch offline benchmark panicked on live-only fixtures

`web-fetch benchmark --fixtures testdata/webfetch/live/public-fixtures.json` (no `--live`)
crashed immediately: `http: panic serving: invalid WriteHeader code 0`. Live
fixtures carry only URLs (StatusCode zero) and the offline replay server wrote
that zero. Fix: clamp replay status to 200 when outside 100-599
(`internal/adapter/outbound/webfetch/benchmark.go`). Live runs need
`ISSUEOPS_WEBFETCH_LIVE=1` and score 100 on the public fixtures.

## 2. mcpsmoke had a shipped data race between stderr writer and reader

`runSDKSmoke` evaluated `stderr.String()` while the subprocess still wrote
into the `cmd.Stderr` buffer; the deferred `session.Close()` ran after result
assembly. A new live `ValidateMCP` test under `-race` caught the actual
`WARNING: DATA RACE`. Fix: close the session explicitly, snapshot stderr
once. **General rule: `session.Close()` (or any subprocess teardown) must
happen before reading any buffer wired to that subprocess.** Live tests catch
what seam-mocked tests cannot — the WithDeps seams were fully covered, the
real composition raced.

## 3. Help requests polluted hook metrics and failures

`hook --help` was recorded as a hook failure with hook name `"--help"`, and
`hook <sub> --help` polluted metrics (`--help` appeared in `by_hook`). Fixes:
top-level help returns `flag.ErrHelp` from the dispatcher; the metrics path
skips `errors.Is(err, flag.ErrHelp)` exactly like the failures path already
did. Never treat help output as a hook event.

## 4. stop --enforce-numbered-next-actions requires the FULL choice format

Partial formats block correctly. `allow` requires all of: a `선택지:` header,
exactly 3 items, exactly 1 marked `(추천)`, and a `선택지 품질 증거` section
(context 확인 / 추천 근거 / 사용자 승인 경계). The input key is
`last_assistant_message` (not `last_message`). `post-compact` emits a
context-injection schema without an `ok` field by design.

## 5. install --project-local refusal from non-canonical paths is intended

Direct `install --project-local` from a binary outside the canonical target
directory is refused ("native install candidate must be the canonical target
or a same-directory staged binary"). That is the atomic activation contract;
`scripts/install-native.sh` stages `.issueops.activate-*` binaries in the
target directory to satisfy it. Dry-run works from anywhere.
