# Aside Functional QA Report Template

```markdown
# Functional QA Report: <capability>

## Executive summary
- **Verdict**: Ready | Ready with minor fixes | Conditionally ready | Not ready
- **Capability and build**:
- **Summary**:
- **Top risks**: <up to three>

## Environment
| Field | Value |
|---|---|
| Target/build | |
| Executed at | |
| Aside version | |
| Account/role class | |
| Viewport/locale/time zone | |
| Test-data prefix | |
| Allowed mutations | |
| Exclusions | |

## Requirements coverage
| Requirement | Acceptance criteria | Tests | Result | Gap |
|---|---|---|---|---|
| R-001 | | FQA-001 | Pass | None |

## Test results
### FQA-001 — <test title>
- **Requirements**:
- **Priority**:
- **Preconditions**:
- **Test data**:
- **Steps**:
  1.
- **Expected**:
- **Observed**:
- **Status**: Pass | Fail | Blocked | Not Run | Inconclusive
- **Evidence**:
- **Cleanup**:
- **Defects**:

## Defects
### BUG-001 — <title>
- **Severity**: Critical | High | Medium | Low
- **Priority**:
- **Affected requirement/user**:
- **Impact**:
- **Reproducibility**:
- **Minimum reproduction**:
  1.
- **Expected**:
- **Observed**:
- **Evidence**:
- **Workaround**:
- **Regression surface**:
- **Retest**: Pending | Pass | Fail

## Release recommendation
<decision and impact-based rationale>

## Evidence index
| Evidence ID | Type | Test/defect | Freshness | Sensitive-data check |
|---|---|---|---|---|
| | Aside JSON | snapshot | screenshot | current build | clear |

## Limitations and cleanup
- **Blocked / Not Run / Inconclusive**:
- **Unsupported Aside capability**:
- **Residual test data**:
- **Test-owned tabs closed**:
- **Attached user tabs left open**:
```

Never count a non-Pass status as passed coverage.
