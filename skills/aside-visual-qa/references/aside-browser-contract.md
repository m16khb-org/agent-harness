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
