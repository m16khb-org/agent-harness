---
name: cautions/lessons/2026-08-21-aside-qa-ux-probe-round.md
description: Dated lesson — aside-qa round-2 dogfood: full UI/UX element probes (typography wrapping, dialogs, sticky obscuring, double-submit), scroll-stabilization waits, and fixture geometry.
---

# 2026-08-21 — aside-qa round 2: UI/UX element probes and measurement pitfalls

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: continued aside-qa dogfooding (Aside CLI 1.26.717.1619) after the
  user required all UI/UX elements — not just typography — to be verifiable
  through the integrated QA.
- Summary: round 1 probes covered contrast/focus/target-size/reflow/stale
  status/timing only. Round 2 added measured probes for text wrapping
  (orphan/clipping/ellipsis/leading/measure/break-all), dialog behavior
  (Escape/trap/focus return/aria-modal), tab-vs-visual order, sticky
  obscuring after anchor jumps, aria-live status announcements,
  double-submit protection, destructive-action confirmation, and hash-nav
  title updates — each verified live against planted-defect fixtures
  (`skills/aside-visual-qa/testdata/typography-fixture.html`,
  `ux-interactions-fixture.html`) before encoding into the skill contracts.

## Pitfalls verified

1. **Anchor-obscuring probes must await scroll stabilization, not measure
   immediately.** Measuring `getBoundingClientRect().top` right after a link
   click read mid-scroll positions (780px instead of ~10px) and masked the
   defect. Poll until the rect stops changing; fixed sleeps are still
   forbidden.
2. **Fixture geometry can make a defect unprovable.** The sticky-header
   obscuring case could not manifest until enough trailing content existed
   for the browser to scroll the anchor fully to the top. A `Not Run` on a
   planted defect means the fixture is wrong before assuming the probe is.
3. **CJK glyph width ≈ font-size, latin ≈ 0.5em is a usable estimator, not a
   model.** Orphan thresholds computed from character arithmetic were wrong
   twice (proportional Hangul); the verified rule is measure-then-derive
   (`lastWidth / fontSize ≤ ~2` chars), never syllable counting.
4. **`confirm` instrumentation needs restore.** Overriding `window.confirm`\n   to detect a missing confirmation layer must restore the original or
   subsequent in-page dialogs break; and absence of `confirm` is not itself
   the defect — custom modals/undo also pass.

- Resolution: probes now live in the browser contracts
  (`skills/aside-visual-qa/references/aside-browser-contract.md` — wrapping
  and interaction sections; `skills/aside-functional-qa/references/aside-browser-contract.md`
  — double-submit and destructive-action sections) and the visual rubric has
  `Dialogs` and `Submit protection` rows.

> Incident-time version references (1.26.717.1619) are dated evidence;
> reprobe after an Aside version change before trusting method shapes.
