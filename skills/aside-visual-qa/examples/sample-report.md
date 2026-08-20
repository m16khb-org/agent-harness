# UI/UX QA Report: Release checklist fixture

## Executive summary

- **Verdict**: Not ready
- **Scope**: Initial checklist form at the actual 1440x900 browser viewport,
  focused text field, accessibility snapshot, and static visual state.
- **Summary**: The page has clear hierarchy and correctly named form controls,
  but keyboard focus is invisible and supporting text contrast is materially
  below the WCAG 2.2 minimum for normal text.
- **Top risks**:
  1. Keyboard users cannot see which control has focus.
  2. The introductory text can be difficult to read for low-vision users.

This is a demonstration report for the repository fixture, not a production
release assessment.

## Environment

| Field | Value |
|---|---|
| Target/build | Local `client-qa-fixture.html` |
| Reviewed at | 2026-08-20T04:03:43Z |
| Aside version | 1.26.717.1619 |
| Account class | Signed in; identity omitted |
| Actual viewport | 1440x900, DPR 2 |
| Intent source | Requirements rendered in the fixture |
| Exclusions | Narrow viewports, network, console history |

## Coverage

| Screen/state/viewport | Status | Evidence | Note |
|---|---|---|---|
| Initial form, 1440x900 | Reviewed | EV-V-001, EV-V-002 | Current fixture |
| Text field focused | Reviewed | EV-V-001 | Focused through Aside |
| Accessibility structure | Reviewed | EV-V-001 | Snapshot exposed names and roles |
| Narrow viewport reflow | Not Run | EV-V-003 | Installed REPL did not prove physical resize |

## Findings

### VQA-001 — Focused input has no visible indicator

- **Severity**: High
- **Disposition**: Confirmed
- **Affected task/user**: Keyboard users adding a checklist item
- **Location/state/viewport**: Checklist item field, focused, 1440x900
- **Expected / authority**: Keyboard focus must be visible; WCAG 2.2
  [2.4.7 Focus Visible](https://www.w3.org/WAI/WCAG22/Understanding/focus-visible.html).
- **Observed**: Aside focused `#item`, which became the active element, while
  computed `outline-style` remained `none`. The inspected screenshot showed no
  replacement focus treatment.
- **Reproduction**:
  1. Open the fixture through Aside.
  2. Focus the control named `Checklist item`.
  3. Inspect the active element and computed outline.
- **Evidence**: EV-V-001, EV-V-002
- **Impact**: Keyboard users lose their location and may submit or edit the
  wrong control.
- **Recommendation**: Restore a high-contrast `:focus-visible` treatment for
  the input and button.
- **Confidence**: High
- **Retest**: Pending

### VQA-002 — Introductory text contrast is 2.10:1

- **Severity**: Medium
- **Disposition**: Confirmed
- **Affected task/user**: Users reading the page purpose, especially low-vision
  users
- **Location/state/viewport**: Supporting text below the H1
- **Expected / authority**: Normal text requires at least 4.5:1 contrast under
  WCAG 2.2
  [1.4.3 Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html).
- **Observed**: Computed `rgb(179, 179, 179)` on white produced a measured
  contrast ratio of 2.0967:1.
- **Reproduction**:
  1. Open the fixture through Aside.
  2. Read computed foreground and background colors for `.lede`.
  3. Apply the WCAG relative-luminance formula.
- **Evidence**: EV-V-001, EV-V-002
- **Impact**: Important context is needlessly hard to read.
- **Recommendation**: Use a darker semantic secondary-text token that reaches
  at least 4.5:1 on white.
- **Confidence**: High
- **Retest**: Pending

## Positive observations

- The H1, form label, input, button, and requirements region form a clear
  content hierarchy.
- Aside's accessibility snapshot exposed `Checklist item` and `Add item` with
  understandable names.
- Form grouping and the item summary are visually coherent.

## Release recommendation

**Not ready.** Restore visible keyboard focus before release. Correct the
supporting-text contrast in the same visual pass, then rerun keyboard and
contrast checks.

## Evidence index

| Evidence ID | Type | Scope | Freshness | Sensitive-data check |
|---|---|---|---|---|
| EV-V-001 | Aside JSON | viewport, computed styles, accessibility summary | Current fixture | Clear |
| EV-V-002 | Aside in-memory JPEG | rendered card and focused state | Current fixture | Clear |
| EV-V-003 | Capability note | viewport resize limitation | Current installed CLI | Clear |

Evidence record:
[`evidence.json`](evidence.json).

## Limitations and cleanup

- **Not Run**: Narrow viewport reflow, browser console history, and network
  interception.
- **Unsupported Aside capability**: No verified physical viewport resize on
  this installed version.
- **Test-owned tabs closed**: Yes.
- **Attached user tabs left open**: Yes; none were used.
