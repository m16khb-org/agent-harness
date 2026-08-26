# Verification protocol

A candidate becomes a finding only after adversarial verification. Every skeptic
is told to REFUTE. Three skeptics, three different lenses; each runs in a fresh
context with the candidate JSON, the context pack paths, and the checkout.

## The three skeptic lenses

| id | tries to prove | must actually do |
|---|---|---|
| `tracer` | "the claim rests on an inferred shape or a boundary the author didn't check" | Open the real definition of every symbol in the claim. Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto constraints, schema, callers' preconditions) and downstream to the consumer of the result. Report each hop with `path:line`. If any hop neutralises the scenario, refute. |
| `reproducer` | "the failure scenario cannot actually happen" | Try to make it happen: write a throwaway unit test in the worktree that encodes the scenario and run it; or run the repo's targeted typecheck/lint on the file; or execute a small script. Paste the command and its outcome. A scenario that cannot be reproduced and cannot be argued from definitions is refuted. Delete throwaway files after. |
| `scoper` | "this is not this change's problem" | Check the cumulative diff hunks: is the defect on added/modified lines, or newly reachable because of them? Check `git log -L`/`git blame` of the lines: pre-existing? Check the description and the other commits in the change: intentional? Check `prior_review_lessons`: was this exact claim refuted before? Refute if pre-existing, intentional and documented, or already refuted. |

## Skeptic prompt

`references/workflow.js` → `skepticPrompt` is the single source of truth (it embeds the
lens text above, the candidate JSON, the rubric, and the rule that inability to verify is
not a refutation — such a skeptic returns `refuted=false`, confidence ≤ 40, reason
starting with `미확인:`).

## Verdict rule

This is the only place the rule is stated; `workflow.js` implements it — if they differ,
`workflow.js` has a bug.

- Fewer than 2 skeptics returned (tool failure) → the candidate is kept as an abstain at
  confidence ≤ 50 (never posted inline; summary only).
- Any skeptic with `refuted=true` and confidence ≥ 70 kills the candidate.
- Otherwise confirmed when at least 2 of the 3 skeptics fail to refute it (a weak refutation,
  confidence < 70, does not outvote a reproduction).
- Final confidence = min(finder confidence, confidences of the non-refuting skeptics).
- One scale everywhere (the rubric below): inline when confidence ≥ 80 (`post_review.py
  --min-confidence`), 60–79 → summary-only list, < 60 → dropped. Gate candidates use the
  same scale: tool failure 95, limit breach 90, pre-existing breach 60, unanchorable 50.
- Severity may only go down or up by one step, and only when a skeptic gives a reason.
- If the tracer or reproducer corrected the line or the suggestion, use the corrected values.

## Scoring rubric (give to every skeptic and use for final confidence)

- 0–25: not verified; inferred from the diff or a single call site; or pre-existing.
- 50: verified real from definitions, but rare or low impact in practice.
- 75: verified real, will be hit on a real path, existing code insufficient.
- 90–100: reproduced (failing test / typecheck error / executed scenario) or directly proven by
  definitions with no escape hatch upstream or downstream.

## Committable suggestions

A `suggestion` replaces lines `new_line..end_line` verbatim. Before it ships, the reproducer
(or the coordinator) applies it in the worktree and runs the fastest relevant check
(`biome check <file>`, `tsc --noEmit -p <project>` scoped, `go vet ./pkg/...`, the targeted test).
A suggestion that does not pass is stripped from the finding (keep the finding, drop the code).

## Learning loop

Every refuted candidate whose refutation depended on a project fact (a signature, a validation
boundary, a documented contract) is a `rule_candidate`. Write it as one Korean sentence naming
the fact and the file that proves it. These go into the summary's "규칙 후보" block so the team
can promote them into `.kody/rules/*.md` or `.agent-harness/CAUTIONS.md` (never auto-write them).
