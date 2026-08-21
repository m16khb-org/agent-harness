# Aside Browser Contract for Functional QA

This file defines the deterministic operations verified for functional testing.
Probe the installed version before each run because the detailed REPL schema is
not fully documented by Aside.

## Authority

- Official guide: <https://docs.aside.com/help/developers>
- Verified locally on 2026-08-20 with Aside CLI `1.26.717.1619`.

The official guide documents `aside`, account management, `aside mcp`, and
`aside repl`. The page and locator methods below are observed runtime behavior.

## Preflight

```bash
command -v aside
aside --version
aside account status
aside repl --help
```

Record only account availability, never account identity.

## Observed deterministic operations

Session:

- `openTab(url)` for a test-owned tab;
- `listBrowserTabs()` and `attachBrowserTab(targetId)` for a specifically
  authorized existing tab.

Readiness and navigation:

- `waitForLoadState`, `waitForURL`, `waitForSelector`, locator `waitFor`;
- `goto`, `goBack`, `goForward`, `reload`, `url()`, `title()`.

Selection and state:

- `getByRole`, `getByLabel`, `getByText`, `locator`;
- `isVisible`, `isEnabled`, `isDisabled`, `isEditable`, `isChecked`;
- `textContent`, `innerText`, `inputValue`, `getAttribute`, `count`.

Actions:

- `click`, `dblclick`, `fill`, `press`, `selectOption`, `check`, `uncheck`,
  `setChecked`, `setInputFiles`, `focus`, `blur`, `hover`.

Evidence:

- `snapshot`, `screenshot`, `content`, and page/locator `evaluate`.
  Verified on 1.26.717.1619: `snapshot()` resolves to `{ tree: string, refs:
  object }`; slice `snapshot.tree`, never the object (`snapshot.slice(...)`
  throws `TypeError`).

## Invocation economics

Each `aside repl` call pays ~0.5s of process overhead and a fresh JS context
— tabs persist in the browser, variables do not. Batch one test's full
lifecycle (preconditions → action → await → evidence → cleanup) into a single
invocation and guard `p.close()` with `finally` so a failed assertion cannot
leak a test-owned tab:

```javascript
const p = await openTab('TARGET_URL')
try {
  /* act and assert */
} finally {
  await p.close()
}
```

## Result sentinel

The CLI has no verified `--json` mode. Print one machine-identifiable record:

```javascript
console.log("ASIDE_QA_RESULT " + JSON.stringify({
  testId: "FQA-001",
  status: "Pass",
  expected: "...",
  observed: "...",
  evidence: {}
}))
```

Update notices and status text may surround this line.

## Safe form recipe

```bash
aside repl "const p = await openTab('TARGET_URL'); await p.waitForLoadState('domcontentloaded'); const field = p.getByLabel('ITEM_LABEL'); const submit = p.getByRole('button', {name: 'SUBMIT_LABEL'}); await field.fill('UNIQUE_TEST_VALUE'); await submit.click(); await p.getByText('EXPECTED_RESULT').waitFor(); console.log('ASIDE_QA_RESULT ' + JSON.stringify({testId:'FQA-001', url:await p.url(), value:await field.inputValue(), observed:await p.getByText('EXPECTED_RESULT').innerText()})); await p.close();"
```

Replace literal values. Never paste secrets into the command or report.

## Persistence recipe

Capture a stable identifier or visible value, reload, await readiness, then
read the same value again:

```javascript
const before = await p.getByText("EXPECTED_RESULT").innerText()
await p.reload()
await p.waitForLoadState("domcontentloaded")
const after = await p.getByText("EXPECTED_RESULT").innerText()
```

This proves only browser-observable persistence. It does not identify whether
the source is memory, storage, or server data unless separately inspected.

## Unsupported or unverified channels

On the verified version, do not claim these without a fresh successful probe:

- browser-console history or reliable console event collection;
- request/response interception, HAR, status/body capture, or network faults;
- physical viewport resizing through REPL;
- popup/download lifecycle events;
- CLI timeout flags or a stable error taxonomy.

If a requirement needs one of these, use `Not Run` and name the missing
capability. A visible success message does not replace required server evidence.

## Cleanup

- Use unique test-data prefixes.
- Delete created data only when deletion is authorized and itself verified.
- Close only tabs created by the test.
- Never close an attached user tab.
- Record any residual test data and why it remains.
