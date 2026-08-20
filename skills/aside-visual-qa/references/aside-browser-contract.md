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

```bash
aside repl "const p = await openTab('https://example.com'); await p.waitForLoadState('domcontentloaded'); const viewport = await p.evaluate(() => ({width: innerWidth, height: innerHeight, dpr: devicePixelRatio})); const snapshot = await p.snapshot(); const shot = await p.screenshot(); console.log('ASIDE_QA_RESULT ' + JSON.stringify({title: await p.title(), url: await p.url(), viewport, snapshot, screenshotBytes: shot.length})); await p.close();"
```

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
