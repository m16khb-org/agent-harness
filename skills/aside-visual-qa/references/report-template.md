# Aside Visual QA Report Template

```markdown
# UI/UX QA Report: <product or journey>

## Executive summary
- **Verdict**: Ready | Ready with minor fixes | Conditionally ready | Not ready
- **Scope**: <screens, states, and journey>
- **Summary**: <plain-language conclusion>
- **Top risks**: <up to three>

## Environment
| Field | Value |
|---|---|
| Target/build | |
| Reviewed at | |
| Aside version | |
| Account class | signed in | signed out | not required |
| Actual viewport(s) | |
| Locale/color/reduced-motion | |
| Intent sources | |
| Exclusions | |

## Coverage
| Screen/state/viewport | Status | Evidence | Note |
|---|---|---|---|
| | Reviewed | Not Run | Blocked | |

## Findings
### VQA-001 — <title>
- **Severity**: Blocker | High | Medium | Low
- **Disposition**: Confirmed | Needs clarification | Not reproducible
- **Affected task/user**:
- **Location/state/viewport**:
- **Expected / authority**:
- **Observed**:
- **Reproduction**:
  1.
- **Evidence**:
- **Impact**:
- **Recommendation**:
- **Confidence**: High | Medium | Low
- **Retest**: Pending | Pass | Fail

## Positive observations
- <good implementation that should not regress>

## Release recommendation
<decision and impact-based rationale>

## Evidence index
| Evidence ID | Type | Scope | Freshness | Sensitive-data check |
|---|---|---|---|---|
| | Aside JSON | snapshot | screenshot | current build | clear |

## Limitations and cleanup
- **Not Run / Blocked**:
- **Unsupported Aside capability**:
- **Test-owned tabs closed**:
- **Attached user tabs left open**:
```

Do not delete empty coverage rows to make the report look complete. Replace
them with the actual status or remove the row only when it was never in scope.
