# Aside Browser Contract for Visual QA

This file separates the public CLI contract from version-sensitive behavior
observed on the installed executable.

## Authority

- Official developer guide:
  <https://docs.aside.com/help/developers>
- Verified locally on 2026-08-20 with Aside CLI `1.26.717.1619`.
- The CLI reported `1.26.810.1915` as available. Do not update it during QA.

The official guide guarantees the CLI, account commands, `mcp`, and `repl`.
Detailed page and locator methods below are an observed runtime contract and
must be reprobed after a version change.

## Preflight

```bash
command -v aside
aside --version
aside account status
aside repl --help
```

Do not include raw `aside account list` output in a report because it may expose
account identities. Record only `signed in`, `signed out`, or `unavailable`.

## Observed top-level helpers

| Helper | Visual QA use |
|---|---|
| `listBrowserTabs()` | Discover tabs; output may contain private URLs |
| `attachBrowserTab(targetId)` | Attach required authenticated tab |
| `openTab(url)` | Create a test-owned isolated tab |

## Observed page operations

Navigation and readiness:

`goto`, `goBack`, `goForward`, `reload`, `waitForLoadState`, `waitForURL`,
`waitForSelector`, `bringToFront`, `close`.

Inspection and evidence:

`title()`, `url()`, `content()`, `snapshot()`, `screenshot()`, `pdf()`,
`evaluate()`, `frames()`, `frameLocator()`, `refreshViewportSize()`.

Selection and interaction:

`getByRole`, `getByLabel`, `getByText`, `locator`, `$`, `$$`, `$$eval`,
`click`, `fill`.

Observed locator operations include `click`, `dblclick`, `hover`, `focus`,
`blur`, `fill`, `press`, `selectOption`, `check`, `uncheck`, `setChecked`,
`setInputFiles`, `waitFor`, visibility/enabled/editable/checked checks, text and
attribute reads, screenshots, and bounding boxes.

## Structured evidence recipe

Use one JSON object with a stable sentinel. Avoid printing a raw snapshot until
private text has been reviewed.

Verified on 1.26.717.1619: `snapshot()` resolves to `{ tree: string, refs: object }`.
`tree` is a human-readable accessibility-tree dump; emit or slice `snapshot.tree`,
never the object itself (`snapshot.slice(...)` throws `TypeError`).

### Batch probes into one invocation

Each `aside repl` call pays ~0.5s of process overhead and a fresh JS context —
tabs persist in the browser, variables do not. Batch a whole state's probes
(title, URL, viewport, computed styles, snapshot.tree head, screenshot bytes)
into one invocation, and guard cleanup with `finally` so a failed probe cannot
leak a test-owned tab:

```bash
aside repl "const p = await openTab('https://example.com'); try { await p.waitForLoadState('domcontentloaded'); const viewport = await p.evaluate(() => ({width: innerWidth, height: innerHeight, dpr: devicePixelRatio})); const tree = (await p.snapshot()).tree; const shot = await p.screenshot(); console.log('ASIDE_QA_RESULT ' + JSON.stringify({title: await p.title(), url: await p.url(), viewport, treeHead: tree.slice(0, 500), screenshotBytes: shot.length})); } finally { await p.close(); }"
```

A full engagement-scale batch (contrast + focus + snapshot + three functional
probes + reload) verified at ~1.2s total versus ~0.5s per probe unbatched.

The CLI may print update notices and status lines around the sentinel. It has no
verified `--json` mode.

## Existing authenticated tab recipe

First list and privately select one exact target. Do not paste the complete tab
list into a report.

```bash
aside repl "const tabs = await listBrowserTabs(); console.log('ASIDE_QA_RESULT ' + JSON.stringify(tabs.map(({targetId,title,url,active}) => ({targetId,title,url,active}))));"
```

Then attach by the selected `targetId`. Do not close an attached user tab.

## Viewport limitation

Verified:

```javascript
await p.refreshViewportSize()
await p.evaluate(() => ({
  width: innerWidth,
  height: innerHeight,
  dpr: devicePixelRatio
}))
```

Do not use `setCachedViewportSize()` as resize evidence. On the verified
version it changed neither `innerWidth` nor `innerHeight`.

## Operational probes (verified)

Rubric categories need measurable probes, not eyeballing. All verified on
1.26.717.1619 against a local fixture; adapt selectors and keep results in the
sentinel JSON. If a probe errors, record the check `Not Run` — do not silently
substitute visual impression.

Text contrast (WCAG 1.4.3) — compute, never guess:

```javascript
await p.evaluate(() => {
  const L = (c) => { const m = c.match(/\d+/g).map(Number).map(v => { v/=255; return v<=0.03928 ? v/12.92 : Math.pow((v+0.055)/1.055, 2.4); }); return 0.2126*m[0]+0.7152*m[1]+0.0722*m[2]; };
  const ratio = (fg, bg) => (Math.max(L(fg),L(bg))+0.05)/(Math.min(L(fg),L(bg))+0.05);
  const s = getComputedStyle(document.querySelector('SELECTOR'));
  return { color: s.color, bg: s.backgroundColor, ratio: +ratio(s.color, 'rgb(255 255 255)').toFixed(2) };
})
```

When the element's real background is not white, resolve the effective
background (nearest ancestor with non-transparent `backgroundColor`) first.
Threshold: 4.5:1 body text, 3:1 large text (≥24px or ≥18.66px bold) and UI
components/graphical objects (1.4.11).

Focus indicator presence (2.4.7) — static CSS check flags the common defect:

```javascript
await p.evaluate(() => ({ outline: getComputedStyle(document.querySelector('SELECTOR')).outlineStyle }))
// 'none' on interactive elements → focus-visible finding unless another
// indicator (box-shadow, border change) is verified on actual :focus.
```

Target size (2.5.8) — via locator bounding box:

```javascript
const box = await p.getByRole('button', {name: 'LABEL'}).boundingBox()
// report {width, height}; flag < 24×24 unless spacing/exceptions apply
```

Fixed-width reflow risk (1.4.10) — static evidence when physical resize is
unsupported:

```javascript
await p.evaluate(() => ({ minWidth: getComputedStyle(document.querySelector('SELECTOR')).minWidth }))
// a px min-width wider than the smallest required viewport is a reflow
// finding; the physical reflow check itself stays `Not Run` per the viewport
// limitation above.
```

Stale status after a failed action — run after any negative-path submit:

```javascript
await p.evaluate(() => ({ status: document.querySelector('STATUS_SELECTOR').textContent, live: !!document.querySelector('[aria-live], [role=alert]') }))
// a prior success message still showing after a failed submit is both a
// system-state finding and a status-messages (4.1.3) finding.
```

Interaction-to-feedback timing — wrap the action in two `Date.now()` reads and
report the delta; flag > 400ms without progress feedback (Doherty threshold,
<https://lawsofux.com/doherty-threshold/>). Timing is one signal among
several, not a pass/fail gate on its own.

## Text wrapping and orphan probes (verified)

Rubric `Typography` row made measurable. Verified on 1.26.717.1619 against
`testdata/typography-fixture.html` (reproducible: `python3 -m http.server` in
that directory). Measure per-line widths by grouping `Range.getClientRects()`
rows — do not eyeball wrapping:

```javascript
await p.evaluate(() => {
  const el = document.querySelector('SELECTOR');
  const cs = getComputedStyle(el);
  const range = document.createRange(); range.selectNodeContents(el);
  const rects = Array.from(range.getClientRects()).filter(r => r.width > 0 && r.height > 0);
  const lines = [];
  for (const r of rects) {
    const line = lines.find(L => Math.abs(L.top - r.top) < Math.max(2, r.height * 0.6));
    if (line) { line.right = Math.max(line.right, r.right); line.left = Math.min(line.left, r.left); }
    else lines.push({ top: r.top, left: r.left, right: r.right });
  }
  const widths = lines.map(L => +(L.right - L.left).toFixed(1));
  const fontSize = parseFloat(cs.fontSize);
  return {
    lines: lines.length,
    lastRatio: widths[widths.length - 1] / Math.max(...widths),
    estLastCJKChars: (widths[widths.length - 1] / fontSize).toFixed(1),   // CJK glyph ≈ fontSize wide
    estMaxLatinChars: Math.round(Math.max(...widths) / (0.5 * fontSize)), // latin avg ≈ 0.5em
    lineHeightRatio: cs.lineHeight === 'normal' ? 'normal' : parseFloat(cs.lineHeight) / fontSize,
    wordBreak: cs.wordBreak,
    textOverflow: cs.textOverflow,
    clippedX: cs.overflowX !== 'visible' && el.scrollWidth > el.clientWidth + 1
  };
})
```

Flags (verified live): last line ≤ ~2 CJK chars with ≥2 lines → orphaned
fragment; `clippedX` with `textOverflow: 'clip'`-equivalent (no ellipsis) →
silent content loss; ellipsis present → verify the full value is still
exposed (`title`, `aria-label`, or an expandable control) — otherwise
information loss; multi-line body `lineHeightRatio < 1.4` → cramped leading
(CJK guidance ≈ 1.5, a convention not a WCAG criterion); latin measure far
beyond ~85 chars → readability finding; `word-break: break-all` on latin
text → mid-word breaks. All thresholds are heuristics to justify a finding,
never verdicts without the observed context.

## Interaction probes (verified)

UX elements beyond visuals, verified on 1.26.717.1619 against
`testdata/ux-interactions-fixture.html`. Batch per state in one invocation;
if a probe errors, record `Not Run`.

Dialog behavior (Escape / focus trap / focus return / semantics):

```javascript
await p.getByRole('button', {name: OPEN_LABEL}).click();
await p.waitForSelector('.dialog-open-selector');
await p.locator(CLOSE_SELECTOR).press('Escape');          // open state after → defect
await p.locator(CLOSE_SELECTOR).press('Tab');             // activeElement outside dialog → no trap
await p.locator(CLOSE_SELECTOR).click();                  // activeElement !== opener → no return
await p.evaluate(() => ({ role: el.getAttribute('role'), ariaModal: el.getAttribute('aria-modal') }))
```

Tab order vs visual order (2.4.3): sort focusable elements by `getBoundingClientRect().x`
and compare with DOM order; a CSS-reordered toolbar that tabs against the
visual order is a finding.

Sticky-header obscuring after anchor jump (2.4.11/2.4.7): await scroll
**stabilization** (poll `getBoundingClientRect().top` until unchanged) —
fixed sleeps are not readiness — then `document.elementFromPoint()` at the
target's center; a hit inside the sticky layer means the focused/anchor
target is obscured. Verified probe shape:

```javascript
await p.evaluate(() => new Promise((resolve) => {
  let last = -1, ticks = 0;
  const iv = setInterval(() => {
    const top = document.querySelector('TARGET').getBoundingClientRect().top;
    if (Math.abs(top - last) < 0.5 || ++ticks > 100) { clearInterval(iv); resolve(top); }
    last = top;
  }, 30);
}))
```

Status announcements (4.1.3): after a dynamic status update, the status
element must expose `aria-live` (or `role=status|alert`); absence is a
finding when the update conveys workflow outcomes.

Hover affordance: `hover()` then compare the computed state (background,
`aria-expanded`, cursor); an interactive control with no state change on
hover is a consistency observation, not automatically a defect.

Reduced-motion preference is readable via `matchMedia('(prefers-reduced-motion: reduce)')`;
behavior emulation is not a verified CLI capability — actual reduced-motion
behavior stays `Not Run` per the viewport-limitation rule.

## Unsupported or unverified channels

Do not claim these checks without a fresh successful probe:

- browser-console history or reliable console event collection;
- request/response interception, HAR, or network failure logs;
- physical viewport resizing through REPL;
- popup lifecycle events;
- formal download lifecycle;
- CLI timeout flags or a stable error taxonomy.

Mark the affected check `Not Run` and state the capability gap. Do not silently
substitute JavaScript state for browser-level evidence.

## Cleanup

- Close only tabs created with `openTab()` during the run.
- Leave attached existing tabs open.
- Record cleanup in the evidence index.
- If a command fails before cleanup, run a separate tab-list probe and close
  only a positively identified test-owned tab.
