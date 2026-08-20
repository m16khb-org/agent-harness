# Functional Test Rubric

## References and status

- [W3C WebDriver](https://www.w3.org/TR/webdriver2/) supports reproducible
  browser actions and observations.
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) is normative for applicable
  accessibility outcomes.
- [Using Fetch](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch)
  explains that an HTTP error can still produce a resolved response.
- [HTTP status codes](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status)
  defines browser-observable response status semantics.
- [localStorage](https://developer.mozilla.org/en-US/docs/Web/API/Window/localStorage)
  and [sessionStorage](https://developer.mozilla.org/en-US/docs/Web/API/Window/sessionStorage)
  have different lifetimes.
- ISTQB terminology is recognized testing guidance, not a universal legal
  mandate: <https://glossary.istqb.org/>.

The test statuses and severity bands in this skill are explicit reporting
conventions, not W3C or ISO-defined labels.

## Requirement test design

For each requirement, consider:

| Dimension | Questions |
|---|---|
| Happy path | Can the intended user complete the task and observe the promised result? |
| Input validity | What happens for empty, malformed, duplicate, unauthorized, or conflicting input? |
| Boundaries | Minimum, maximum, just-inside, just-outside, length, count, date/time, and numeric edges |
| State | Loading, success, empty, validation error, server error, timeout, retry, cancellation |
| Repetition | Double click, duplicate submit, refresh-after-submit, repeated retry |
| Navigation | Reload, back/forward, direct URL, new tab, interrupted return |
| Persistence | Expected lifetime across route, reload, tab, session, logout/login |
| Roles | UI visibility and protected action for each relevant role |
| Consistency | Create/read/update/delete, list/detail, counts, filters, sort, pagination |
| Recovery | Error message, preserved input, retry path, cancellation, rollback |

## Test record quality gate

Each test record contains:

- stable test ID and linked requirement IDs;
- priority and role;
- preconditions and unique test data;
- exact browser steps;
- expected visible and persisted outcomes;
- observed outcomes;
- result status;
- evidence IDs;
- cleanup status;
- linked defect IDs.

## Defect quality gate

A defect contains:

- stable ID, severity, and separate priority;
- affected user, requirement, and business impact;
- reproducibility and minimum steps;
- expected versus observed result;
- environment and test data without secrets;
- screenshot/snapshot and any available state evidence;
- workaround, regression surface, and retest state.

## Release decision

- `Ready`: all in-scope release criteria pass and no material gap remains.
- `Ready with minor fixes`: only low-impact defects remain.
- `Conditionally ready`: known material gaps have an explicit accepted
  condition, mitigation, or deferred scope.
- `Not ready`: core criteria fail, evidence is insufficient for a required
  decision, or critical/high-impact risk remains.
