# Verification protocol

A candidate becomes a finding only after adversarial verification: a deterministic
prescreen, then skeptics told to REFUTE, run in sequence so a refutation stops the
spend. Each skeptic runs in a fresh context with the candidate JSON, the context pack
and the checkout, under an 8-message budget. It starts from `hunks/<file>.patch` (the
candidate file's diff) and the `## <symbol>` sections of `defs.md`, read together in one message.

## Stage 0 — prescreen (code, no agent)

`workflow.js → prescreen()` refutes at confidence 90, before any skeptic runs, a candidate that
- names a file that is not in the change, or
- sits on a line outside every hunk of that file and is not marked `newly_reachable` by the
  finder (who must then say which changed line makes it reachable), or
- matches (bigram similarity ≥ 0.5 on title or `what`) a `prior_review_lessons` finding.

This is what the former `scoper` skeptic spent an agent on; the remaining scoper question —
"intentional *and* correct per the linked issue?" — belongs to the tracer.

## Blind tracer

The tracer receives the candidate WITHOUT the finder's `evidence`, `upstream` and `downstream`
(path, line, title, what, why, severity, category only). It must open the definitions and walk
the hops itself — a verifier that re-reads the finder's trace tends to confirm it
(OpenCodeReview, arXiv 2608.09290: reflector sees only the diff; Adversarial Review, arXiv
2608.18167: explicit disagreement beats consensus). The reproducer, which runs only when the
tracer failed to refute, sees the full candidate and the tracer's verdict.

## Prescreen from team memory

`<repo>/.agent-harness/parnas/refuted.jsonl` (written by `scripts/record_refuted.py`, committed)
holds refutations a skeptic proved with confidence ≥ 80. A new candidate on the same file whose
title/what token overlap (|∩| / min) is ≥ 0.5 with a recorded one is refuted by the prescreen at
90. `security` and `data` candidates are never suppressed (Greptile/Kodus keep security findings
exempt from learned suppression). Dedup preserves a security/data category so a merged candidate
cannot lose the exemption.

## Severity adjustment

A skeptic's `severity_adjust` is applied only when that verdict carries at least one evidence
entry — unsupported severity scores from a model are noise (Greptile 2025; arXiv 2608.02677).

## The two skeptic lenses

| id | tries to prove | must actually do |
|---|---|---|
| `tracer` | "the claim rests on an inferred shape, a boundary the author didn't check, or a misread intent" | Open the real definition of every symbol in the claim. Walk upstream to the nearest validation boundary (DTO validators, guards, pipes, proto constraints, schema, callers' preconditions) and downstream to the consumer of the result. Report each hop with `path:line`. Check the description and linked issue: intentional *and* correct? (Intentional but wrong per the issue is still a defect.) If any hop or the issue neutralises the scenario, refute. |
| `reproducer` | "the failure scenario cannot actually happen" | Runs only when the tracer failed to refute (it receives the tracer's reason and must not repeat the trace). Try to make it happen: write a throwaway unit test in the worktree that encodes the scenario and run it; or run the repo's targeted typecheck/lint on the file; or execute a small script. Paste the command and its outcome. A scenario that cannot be reproduced and cannot be argued from definitions is refuted. Delete throwaway files after. |

## Skeptic prompt

`references/workflow.js` → `skepticPrompt` is the single source of truth (it embeds the
lens text above, the candidate JSON, the rubric, and the rule that inability to verify is
not a refutation — such a skeptic returns `refuted=false`, confidence ≤ 40, reason
starting with `미확인:`).

## Verdict rule

This is the only place the rule is stated; `workflow.js` implements it — if they differ,
`workflow.js` has a bug.

- Prescreen refusal, or any skeptic with `refuted=true` and confidence ≥ 70, kills the
  candidate; a tracer kill skips the reproducer entirely.
- A non-refuting verdict with confidence ≤ 40 or a reason starting with `미확인:` is
  unavailable evidence, even when its JSON shape is valid; it is excluded from the pair
  and makes the candidate an abstain.
- Fewer than 2 usable verdicts for a non-killed candidate (tool failure or unverified
  response) → kept as an abstain at confidence ≤ 50 (never posted inline; summary only).
- Otherwise confirmed only when both skeptics fail to refute it (a weak refutation,
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
