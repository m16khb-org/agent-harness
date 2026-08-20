# Functional QA Report: Release checklist fixture

## Executive summary

- **Verdict**: Not ready
- **Capability and build**: Local release-checklist fixture
- **Summary**: Adding a valid item works, but two of three explicit
  requirements fail: empty submission provides no validation and added items
  disappear after reload.
- **Top risks**:
  1. Users receive no guidance after invalid submission.
  2. Checklist data is lost on reload.

This is a demonstration report for the repository fixture, not a production
release assessment.

## Environment

| Field | Value |
|---|---|
| Target/build | Local `client-qa-fixture.html` |
| Executed at | 2026-08-20T04:03:43Z |
| Aside version | 1.26.717.1619 |
| Account/role class | Signed in; identity omitted / anonymous fixture |
| Viewport | 1440x900 |
| Test-data prefix | `QA item 2026-08-20` |
| Allowed mutations | In-memory fixture data only |
| Exclusions | Console history, network response evidence |

## Requirements coverage

| Requirement | Acceptance criteria | Tests | Result | Gap |
|---|---|---|---|---|
| R-001 | Non-empty item appears and count increments | FQA-001 | Pass | None |
| R-002 | Empty submit shows linked inline error | FQA-002 | Fail | No error or linkage |
| R-003 | Added items survive reload | FQA-003 | Fail | Data resets |

## Test results

### FQA-001 — Add a valid checklist item

- **Requirements**: R-001
- **Priority**: High
- **Preconditions**: Fresh fixture with zero items
- **Test data**: `QA item 2026-08-20`
- **Steps**:
  1. Fill `Checklist item`.
  2. Activate `Add item`.
  3. Await the exact item text.
- **Expected**: Item appears and count becomes 1.
- **Observed**: Item list contained the exact value, count was `1`, and status
  was `Added QA item 2026-08-20`.
- **Status**: Pass
- **Evidence**: EV-F-001
- **Cleanup**: Reload cleared in-memory data.
- **Defects**: None

### FQA-002 — Submit an empty checklist item

- **Requirements**: R-002
- **Priority**: High
- **Preconditions**: Input empty after FQA-001
- **Test data**: Empty string
- **Steps**:
  1. Activate `Add item` with an empty field.
  2. Inspect status text and input error linkage.
- **Expected**: A linked inline validation error appears.
- **Observed**: The previous success message remained. The input had neither
  `aria-invalid` nor `aria-describedby`, and no error appeared.
- **Status**: Fail
- **Evidence**: EV-F-002
- **Cleanup**: No record was created.
- **Defects**: BUG-001

### FQA-003 — Preserve an added item after reload

- **Requirements**: R-003
- **Priority**: Critical
- **Preconditions**: One item visible from FQA-001
- **Test data**: `QA item 2026-08-20`
- **Steps**:
  1. Confirm count is 1.
  2. Reload the page.
  3. Await DOM readiness and inspect list and count.
- **Expected**: The item remains and count stays 1.
- **Observed**: Count returned to `0`, the list was empty, and status text was
  blank.
- **Status**: Fail
- **Evidence**: EV-F-003
- **Cleanup**: No residual data.
- **Defects**: BUG-002

## Defects

### BUG-001 — Empty submission gives no validation feedback

- **Severity**: Medium
- **Priority**: High
- **Affected requirement/user**: R-002; any user submitting an empty form
- **Impact**: The action appears unresponsive and offers no recovery guidance.
- **Reproducibility**: 1/1
- **Minimum reproduction**:
  1. Open the fixture.
  2. Activate `Add item` with an empty input.
- **Expected**: Linked inline error.
- **Observed**: No error, invalid state, or descriptive linkage.
- **Evidence**: EV-F-002
- **Workaround**: Enter a non-empty value.
- **Regression surface**: Form validation and status messaging.
- **Retest**: Pending

### BUG-002 — Checklist items are lost on reload

- **Severity**: High
- **Priority**: Critical
- **Affected requirement/user**: R-003; users relying on the release checklist
- **Impact**: Completed setup work is lost, making the capability unreliable.
- **Reproducibility**: 1/1
- **Minimum reproduction**:
  1. Add an item.
  2. Reload the page.
- **Expected**: Item and count persist.
- **Observed**: Count resets to zero and the item disappears.
- **Evidence**: EV-F-001, EV-F-003
- **Workaround**: None within the product.
- **Regression surface**: Initial load, persistence, list hydration, counts.
- **Retest**: Pending

## Release recommendation

**Not ready.** R-003 is a core data-retention requirement and fails without a
workaround. Implement persistence and linked validation feedback, then rerun
all three requirements from a clean state.

## Evidence index

| Evidence ID | Type | Test/defect | Freshness | Sensitive-data check |
|---|---|---|---|---|
| EV-F-001 | Aside JSON and screenshot | FQA-001 | Current fixture | Clear |
| EV-F-002 | Aside JSON | FQA-002 / BUG-001 | Current fixture | Clear |
| EV-F-003 | Aside JSON after reload | FQA-003 / BUG-002 | Current fixture | Clear |

Evidence record:
[`evidence.json`](evidence.json).

## Limitations and cleanup

- **Not Run**: Browser console history and network response evidence.
- **Unsupported Aside capability**: No verified console/network collection on
  this installed version.
- **Residual test data**: None.
- **Test-owned tabs closed**: Yes.
- **Attached user tabs left open**: Yes; none were used.
