# Combined Web QA Report: <capability or journey>

## Executive summary
- **Verdict**: Ready | Ready with minor fixes | Conditionally ready | Not ready
- **Engagement scope**: <functional capability + visual scope>
- **Summary**: <plain-language conclusion covering both dimensions>
- **Top risks**: <up to three, from either dimension>

## Environment (shared preflight)
| Field | Value |
|---|---|
| Target/build | |
| Executed at | |
| Aside version | |
| Account/role class | |
| Viewport/locale | |
| Test-data prefix | |
| Allowed mutations | |
| Exclusions | |

## Functional results
<Full section following the aside-functional-qa report template:
requirements coverage, test results (FQA-...), defects (BUG-...),
release recommendation with its own phase verdict.>

## Visual results
<Full section following the aside-visual-qa report template:
coverage table, findings (VQA-...), positive observations,
release recommendation with its own phase verdict.>

## Consolidated defects
| ID | Title | Severity | Owner | Cross-links | Impact |
|---|---|---|---|---|---|
| BUG-001 | | Critical/High/Medium/Low | functional | VQA-003 | |
| VQA-003 | | Blocker/High/Medium/Low | visual | BUG-001 | |

## Coverage view
| Dimension | Declared | Executed | Blocked | Not Run |
|---|---|---|---|---|
| Requirements | R-001..R-012 | 10 | 1 | 1 |
| Screens/states | 9 | 8 | 1 | 0 |

## Evidence index
| Evidence ID | Phase | Type | Test/defect | Freshness | Sensitive-data check |
|---|---|---|---|---|---|
| | functional/visual | Aside JSON/snapshot/screenshot | | current build | clear |

## Limitations and cleanup
- **Blocked / Not Run / Inconclusive**: <per phase with causality>
- **Unsupported Aside capability**:
- **Deferred cleanup executed**:
- **Residual test data**:
- **Test-owned tabs closed**:
- **Attached user tabs left open**:
