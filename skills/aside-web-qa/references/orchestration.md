# Aside Web QA Orchestration Reference

Detailed contracts for the orchestration layer of `aside-web-qa`. The two
sub-skills stay authoritative for their phase; this file defines what sits
between them.

## Routing matrix

| Ask | Owner |
|---|---|
| Requirements, state transitions, persistence, authorization | `aside-functional-qa` |
| Design intent, UX, accessibility, responsiveness, coherence | `aside-visual-qa` |
| Combined engagement, release-readiness sweep, 통합/종합 QA, full client QA report | `aside-web-qa` |
| Ambiguous "QA 해줘" with both behavioral and visual stakes | `aside-web-qa` |

When routing down to one sub-skill, say so and stop; do not run the orchestrator
workflow with one phase empty.

## Shared preflight record

One record per engagement, captured before any phase runs:

| Field | Content |
|---|---|
| Aside version / account class | From the browser-contract preflight, no identities |
| Target build / actual URL | What both phases will hit |
| Viewport / locale / role class | Actually available, not requested |
| Test-data prefix | Engagement-wide unique prefix |
| Cleanup plan | Owning phase per item, deferral flags |
| Required REPL methods | Union of both phases' needs |

If any field required by only one phase fails (for example, a role class only
the functional phase needs), block that phase, not the engagement, and record
the split.

## Reachable-state map

Produced by the functional phase, consumed by the visual phase. One row per
reached state:

| Route | State | Entry action | Role | Data dependency | Stability | Evidence |
|---|---|---|---|---|---|---|
| /orders | empty list | fresh login | viewer | none | deterministic | FQA-003 |

The visual phase enumerates its coverage from the declared `SCOPE` plus this
map. A state in `SCOPE` but absent from the map after a functional failure is
`Blocked by FQA-...`, never silently dropped.

## Cleanup deferral contract

- Functional cleanup that would destroy a state the visual phase must review is
  deferred, not skipped: flag it in the shared cleanup plan with the owning
  test ID.
- Deferred cleanup executes after the visual phase, before the merged report is
  considered complete.
- Non-destructive cleanup (closing test-owned tabs is an exception — tabs stay
  open across phases per the engagement tab plan) runs normally.
- If the engagement aborts mid-way, run all outstanding cleanup, then report
  the abort with the cleanup evidence.

## Cross-phase defect routing

| Symptom | Primary owner | Cross-link |
|---|---|---|
| Business rule wrong, data lost, authorization bypassed | Functional `BUG-...` | Visual links UI manifestation if visible |
| Layout, contrast, a11y, responsive, coherence break | Visual `VQA-...` | Functional links blocked tests if any |
| Visual defect blocks a task (obscured/unreachable control) | Visual `VQA-...` owns the cause | Functional records the test as `Blocked by VQA-...` |
| Functional failure hides a screen from review | Functional `BUG-...` owns the cause | Visual records the state as `Blocked by FQA-...` |
| Same root cause, symptoms in both dimensions | One defect under the primary owner | Other phase cross-references, no second ID |

Deduplication rules:

- one root cause = one defect ID, even if it appears in both phases;
- split findings across viewports only when the cause differs;
- praise stays in `Positive observations` and never offsets a defect;
- disputed ownership goes to the dimension whose acceptance criterion or rubric
  the root cause actually violates.

## Verdict merge

| Functional verdict | Visual verdict | Merged verdict |
|---|---|---|
| Ready | Ready | Ready |
| Ready with minor fixes | any | lesser of the two |
| Conditionally ready | any ≤ Conditionally ready | lesser of the two |
| Not ready | any | Not ready |
| Fully Blocked / Not Run | V | V with explicit coverage gap |

The rationale must cite the driving phase's evidence, not the issue count. A
phase that is entirely `Blocked` or `Not Run` cannot produce a merged
"Ready" statement.

Severity rank across phases: functional `Critical` and visual `Blocker` are
the same top rank, followed by `High`, `Medium`, `Low`. Merge by that rank, so
a `Critical` functional defect and a `Blocker` visual defect drive the same
merged verdict; the label difference between the two templates never changes
the order.

## Merged report skeleton

Follow `examples/sample-report.md`. Shape:

1. Executive summary — one verdict, engagement scope, top risks from both
   dimensions.
2. Environment — the shared preflight record, once.
3. Functional results — per the functional report template.
4. Visual results — per the visual report template.
5. Consolidated defects — one table ordered by user impact, primary owner
   column, cross-links.
6. Coverage view — requirements coverage and screen/state coverage side by
   side.
7. Evidence index — one index, per-phase artifact IDs (`FQA-...`, `VQA-...`),
   freshness, sensitive-data check.
8. Limitations and cleanup — union of both phases plus deferral resolution.

## Evidence index conventions

- All structured captures use the `ASIDE_QA_RESULT` JSON sentinel defined in
  each sub-skill's browser contract.
- The environment block appears once; phase sections reference it.
- Cross-phase reuse of an artifact requires matching build, state, viewport,
  and locale, and the reuse is recorded in the freshness column.
