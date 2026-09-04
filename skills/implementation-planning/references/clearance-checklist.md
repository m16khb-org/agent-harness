# Implementation Planning Clearance Checklist

The 6-item clearance check runs after EVERY interview turn. ALL must be YES before transitioning to plan generation.

## Checklist

```
CLEARANCE CHECKLIST:
[ ] Core objective clearly defined?
    → The user's request distilled to one sentence. No ambiguity about what "done" means.

[ ] Scope boundaries established (IN/OUT)?
    → Both what the plan WILL deliver and what it explicitly WILL NOT touch.

[ ] No critical ambiguities remaining?
    → Every "it depends" resolved. Every "we could do X or Y" decided.

[ ] Technical approach decided?
    → Specific patterns, libraries, file locations, naming conventions chosen.
    → "We'll use pattern X from src/foo/bar.ts:42 because..."
    → NOT: "We'll implement something like..."

[ ] Test strategy confirmed?
    → TDD / tests-after / no-tests explicitly stated.
    → Test framework named. QA scenarios acknowledged.

[ ] No blocking questions outstanding?
    → Every question asked has been answered.
    → No "I'll figure this out during implementation" deferrals.
```

## Auto-Transition

When ALL items are YES, announce: "All requirements clear. Proceeding to plan generation." Then transition immediately. Do not ask for permission to proceed.

## Partial Clearance

When any item is NO, ask the SPECIFIC question that will resolve it. Do not re-ask already-answered questions. Do not re-explain the entire context.

## Clearance Re-Check

After the user answers a question:
1. Update the draft (`.issueops/drafts/<slug>.md`)
2. Re-run the full checklist
3. If still NO on any item, ask the next specific question
4. Repeat until all YES or explicit "proceed anyway" from user
