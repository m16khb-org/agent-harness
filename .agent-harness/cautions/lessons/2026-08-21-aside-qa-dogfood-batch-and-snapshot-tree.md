---
name: cautions/lessons/2026-08-21-aside-qa-dogfood-batch-and-snapshot-tree.md
description: Dated lesson — aside repl snapshot shape, per-invocation overhead, and localhost server liveness from aside-qa dogfooding on 2026-08-21.
---

# 2026-08-21 — aside-qa dogfood: snapshot shape, invocation economics, localhost liveness

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: aside-qa skill dogfooding against `skills/aside-functional-qa/testdata/client-qa-fixture.html` with Aside CLI 1.26.717.1619 (2026-08-21)
- Summary: Three operational facts the aside-qa skills initially got wrong or omitted; now encoded in the skills' browser contracts.

## 1. `snapshot()` resolves to `{tree, refs}`, not a string

`const s = await p.snapshot(); s.slice(0, 400)` throws `TypeError: snapshot.slice
is not a function`. The accessibility tree is `s.tree` (string); `s.refs` is an
object keyed by snapshot `ref=` ids. The visual-qa evidence recipe shipped this
exact bug; verified live before fixing.

## 2. Each `aside repl` invocation is a fresh JS context with ~0.5s overhead

Tabs persist in the desktop browser across invocations; variables and `const p`
handles do not. A workflow written as one probe per invocation (open → inspect →
assert → cleanup as separate calls) pays process overhead per call, re-opens the
page per call, and cannot carry state. Verified: 6-probe engagement batched into
one invocation ran ~1.2s total vs ~0.5s per single probe unbatched. Batch one
state's full lifecycle per invocation and guard `p.close()` with `finally`.

## 3. A localhost target server can die between dogfood sessions

An `http.server` started in a foreground shell dies with that shell; the QA run
then sees `chrome-error://chromewebdata/` with a generic `localhost` title and
can misread it as a product defect. Preflight must curl the target (or record
HTTP status) before blaming the page; serve long-lived targets from a
background session.

- Resolution: recipes and invocation-economics sections in both browser
  contracts (`skills/aside-*/references/aside-browser-contract.md`) now carry
  the verified shapes, batching rule, and `finally` cleanup; rubric probes
  (contrast, focus, target size, stale status, timing) are recorded as
  verified operational probes.

> Incident-time version references (1.26.717.1619) are dated evidence; reprobe
> after an Aside version change before trusting method shapes.
