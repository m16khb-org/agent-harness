# Research: Aside Client QA Skills

## TL;DR

**Conclusion**: Two separate skills are warranted. `aside-visual-qa` should own
intent, usability, accessibility, responsive behavior, and visual coherence.
`aside-functional-qa` should own requirements traceability, browser-observable
state transitions, negative paths, persistence, and reproducible defects. Both
should use `aside repl` for deterministic browser operations and treat features
not proved by the installed CLI as unavailable.

**Confidence**: High for the installed CLI contract and W3C requirements;
medium for undocumented Aside method stability.

## Method

- Inspected the repository skill conventions and validator.
- Ran the installed Aside CLI and REPL against `https://example.com` and a
  local `data:` page.
- Cross-checked Aside's official developer page, the installed help output,
  and a community setup guide.
- Fanned out independent research for visual/accessibility and functional QA.
- Retrieved the primary W3C, MDN, and Nielsen sources listed below on
  2026-08-20.

## Findings

### 1. Aside exposes a deterministic browser QA surface

- **Official contract**: Aside documents `aside`, `aside --session`,
  `aside account`, `aside mcp`, and `aside repl`; it recommends the REPL for
  direct page inspection, screenshots, downloads, and deterministic steps.
- **Installed contract**: version `1.26.717.1619` exposes `account`, `exec`,
  `repl`, and `mcp`.
- **Observed REPL helpers**: `listBrowserTabs`, `attachBrowserTab`, and
  `openTab`.
- **Observed page operations**: navigation, URL/title/content inspection,
  accessibility snapshot, role/label/text locators, click/fill/press,
  screenshots, PDF, frames, and state-based waits.
- **Observed locator operations**: visibility/state checks, text and attribute
  reads, click/double-click/hover/focus/fill/press/select/check/upload, element
  screenshots, and `waitFor`.
- **Verification**: local `aside --help`, subcommand help, and successful REPL
  calls.

### 2. The skills must distinguish proven and unproven capabilities

- `console.log(JSON.stringify(...))` is a practical structured-output channel,
  but the CLI has no documented `--json` contract.
- `refreshViewportSize()` accurately reported the 1440x900 browser viewport.
  `setCachedViewportSize()` did not change `window.innerWidth` or
  `window.innerHeight`; it is not a viewport resize mechanism.
- The installed page prototype contained event-looking methods, but a console
  event probe returned no messages and `p.console()` was not callable.
- No verified network interception, HAR, browser-console history, popup event,
  download lifecycle, CLI timeout flag, or formal error taxonomy was found.
- **Decision**: the skills must probe current capability before use, mark an
  unsupported channel `Not Run`, and never invent a fallback API.

### 3. Visual QA needs standards plus human judgment

- WCAG 2.2 provides testable accessibility success criteria, while full-page
  and complete-process conformance cannot be inferred from one element or an
  automated scan.
- Browser-observable checks should cover accessible names/roles/states,
  keyboard operation, visible and unobscured focus, reflow, form errors, and
  status messages.
- Nielsen's heuristics add non-normative usability review for system status,
  user control, consistency, error prevention/recovery, recognition, and help.
- Severity is a product-impact judgment; WCAG level is not a defect severity
  scale.

### 4. Functional QA needs explicit traceability and state coverage

- Each requirement should map to observable acceptance criteria, tests,
  evidence, and defects.
- Coverage should include happy, negative, and boundary paths plus loading,
  success, empty, error, unauthorized, expired, retry, and cancelled states.
- Reload, history navigation, deep links, repeated actions, and persistence are
  separate browser-observable behaviors.
- A resolved Fetch promise does not imply HTTP success, so network evidence
  must include response status when the current Aside runtime can expose it.
- `Pass`, `Fail`, `Blocked`, `Not Run`, and `Inconclusive` are reporting
  conventions and must be defined by the skill rather than presented as W3C or
  ISO status names.

### 5. The two skills need a strict ownership boundary

| Concern | Owner |
|---|---|
| Design intent, visual hierarchy, responsive coherence | `aside-visual-qa` |
| Accessibility interaction and perceivability | `aside-visual-qa` |
| Requirements and acceptance-criteria traceability | `aside-functional-qa` |
| Business state transitions and data persistence | `aside-functional-qa` |
| Shared symptom | Primary owner records it; the report cross-links the other skill |

Both skills should produce a client summary followed by developer-actionable
evidence. Neither should modify the product while reviewing it.

## Unresolved and Version-Sensitive Areas

- The official Aside page does not publish the detailed REPL object schema.
  Method names verified on `1.26.717.1619` may change.
- Aside reported an available update (`1.26.810.1915`) during every probe.
  This work does not update the user's installation.
- Viewport resizing and console/network collection require a future verified
  API before the skills may claim those checks ran.

## Source Index

| Source | Type | Retrieved | Use |
|---|---|---|---|
| [Aside developer tools](https://docs.aside.com/help/developers) | Official | 2026-08-20 | CLI, account, MCP, REPL surface |
| [Aside documentation index](https://docs.aside.com/llms.txt) | Official | 2026-08-20 | Public documentation coverage |
| [Community Aside setup](https://github.com/steggdev/aside-browser-mcp/blob/b0f4e58f9ee32a8b960f9bb4453e0b77408f22f8/ASIDE_BROWSER_SETUP.md) | Community | 2026-08-20 | Cross-check of sandbox and REPL usage |
| [WCAG 2.2](https://www.w3.org/TR/WCAG22/) | W3C Recommendation | 2026-08-20 | Accessibility baseline |
| [WAI-ARIA APG](https://www.w3.org/WAI/ARIA/apg/) | W3C guidance | 2026-08-20 | Names, roles, states, keyboard patterns |
| [Understanding Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html) | W3C guidance | 2026-08-20 | Responsive/reflow checks |
| [Focus Not Obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html) | W3C guidance | 2026-08-20 | Keyboard focus checks |
| [Ten Usability Heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/) | Recognized guidance | 2026-08-20 | Heuristic UX review |
| [WebDriver](https://www.w3.org/TR/webdriver2/) | W3C Recommendation | 2026-08-20 | Reproducible browser actions |
| [Using Fetch](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch) | MDN reference | 2026-08-20 | Network-success semantics |

## Local Evidence Commands

```text
aside --version
aside --help
aside account --help
aside exec --help
aside repl --help
aside mcp --help
aside repl "<listBrowserTabs probe>"
aside repl "<openTab, waitForLoadState, snapshot, screenshot probe>"
aside repl "<role locator and page/locator prototype probe>"
aside repl "<viewport read and cached-size mutation probe>"
aside repl "<console event probe>"
```

## Implementation Contract

### Skill packages

```text
skills/aside-visual-qa/
  SKILL.md
  agents/openai.yaml
  references/aside-browser-contract.md
  references/visual-review-rubric.md
  references/report-template.md
  examples/sample-report.md

skills/aside-functional-qa/
  SKILL.md
  agents/openai.yaml
  references/aside-browser-contract.md
  references/functional-test-rubric.md
  references/report-template.md
  examples/sample-report.md
  testdata/client-qa-fixture.html
```

The browser-contract files are self-contained and skill-specific. They repeat
only the small preflight needed for independent installation; visual evidence
commands and functional state commands remain separate.

### Shared input contract

Required:

- target URL or an explicit way to reach the running application;
- named screen, capability, or user journey;
- intended outcome, requirement, or acceptance criteria.

Optional:

- design reference, story IDs, role/account class, safe test-data boundary,
  allowed mutations, viewport expectations, locale/time zone, exclusions,
  known defects, and output directory.

If intent is missing, the skill inspects repository evidence and existing
product patterns before asking one narrow question. Credentials and tokens are
never accepted into the report.

### Runtime boundary

- Preflight `command -v aside`, version, account status, and the exact help
  needed by the run.
- Prefer a test-owned `openTab()` and close only that tab.
- Attach an existing tab only when authenticated state is required; never close
  or navigate unrelated user tabs.
- Wait on URL, selector, load state, or locator state rather than fixed sleeps.
- Print evidence with an `ASIDE_QA_RESULT ` JSON sentinel.
- Treat unproved APIs as unavailable and mark affected checks `Not Run`.
- Never update Aside, install software, submit destructive actions, or expose
  private browsing data without explicit authorization.

## Report Contract

### Shared evidence fields

Every finding or test result includes a stable ID, scope, exact browser steps,
expected result, observed result, evidence, impact, confidence, and retest
state. Environment metadata records URL, timestamp, Aside version, account
class without identity, viewport, locale/time zone when relevant, and explicit
exclusions.

### Visual result model

- Severity: `Blocker`, `High`, `Medium`, `Low`.
- Disposition: `Confirmed`, `Needs clarification`, `Not reproducible`.
- Positive observations are recorded separately and never inflate defect
  counts.
- Release recommendation: `Ready`, `Ready with minor fixes`,
  `Conditionally ready`, or `Not ready`.

### Functional result model

- Execution status: `Pass`, `Fail`, `Blocked`, `Not Run`, `Inconclusive`.
- Severity: `Critical`, `High`, `Medium`, `Low`; priority is a separate field.
- Each test ID maps to one or more requirement IDs, and each requirement maps
  back to its executed tests.
- Release recommendation uses the same four values as the visual report and is
  based on unmet acceptance criteria and impact, not raw defect count.
