# AI Slop Clean

Use this phase after implementation and before PR/MR drafting. The goal is not broad refactoring; it is a focused cleanup pass that removes lazy agent residue while preserving behavior.

## Evidence Used

This prompt pattern is adapted from established code review and AI-slop detection practices, and aligns with the **`turing`** skill's Final Quality Gate step 2 (AI slop clean + re-verify) and Reviewer Gate:

- **Deletion-first cleanup workflow** (ai-slop-cleaner pattern): lock behavior with tests before editing, classify slop before deleting, run one smell-focused pass at a time, and report evidence densely.
- **Korean prose de-slop** (`fluent-korean` references/slop-patterns.md): the deletion test (if removing a phrase loses no information, it is slop), information preservation, and no fabrication apply to comments, docs, and PR/issue prose as well as code.
- **`turing`** Cleanup Phase and Reviewer Gate: inspect the worktree diff for lazy agent artifacts, unsupported claims, generic prose, dead scaffolding, unnecessary abstractions, weak comments, and brittle shortcuts; every reviewer concern is real and binding.
- Verification principle: no completion claim without fresh command evidence.

## Prompt

Run this from the exact IssueOps worktree, after tests for the implementation phase have passed at least once:

```text
You are running the IssueOps ai-slop-clean phase.

Scope boundary:
- Work only in the expected IssueOps worktree: <EXPECTED_WORKTREE>.
- CLEANUP_BOUNDARY = files in the current task diff plus directly related touched files.
- Every file you edit must be inside CLEANUP_BOUNDARY. A smell found outside it is
  recorded under "Out-of-scope findings" and left untouched. Never widen scope to fix it.

Inputs:
- Issue URL: <ISSUE_URL>
- Plan path: <PLAN_PATH>
- Worktree branch: <BRANCH>
- Diff command: git -C <EXPECTED_WORKTREE> diff --stat && git -C <EXPECTED_WORKTREE> diff
- Verification commands already run: <COMMANDS_AND_RESULTS>

Step 1 - Lock current behavior before editing:
- Identify what must stay the same inside CLEANUP_BOUNDARY.
- Confirm targeted regression tests exist and pass now. If none cover a piece of
  behavior you intend to touch during cleanup, add the narrowest test first or
  record an explicit verification plan before editing.

Step 2 - Classify the slop before deleting anything:
| Category | Signs |
| --- | --- |
| Dead code | Unused helpers, unreachable branches, stale flags, debug logs, console prints, commented-out blocks |
| Duplication | Repeated logic, copy-paste branches, redundant one-off wrappers |
| Needless abstraction | Pass-through layers, speculative indirection, single-use "flexibility" |
| Boundary violations | Hidden coupling, wrong-layer imports, misplaced responsibilities |
| Weak artifacts | Comments that restate code, vague TODOs, placeholder text, "temporary" scaffolding |
| Unsupported claims | "all", "always", "guarantees", "complete", "safe", "verified", "보장", "완전", "검증됨" without fresh evidence |

Step 3 - Run one pass at a time, safest first, re-running targeted checks between passes:
1. Dead-code deletion (only items provably unused).
2. Duplicate consolidation.
3. Naming, error-message, and comment cleanup: keep comments that explain
   non-obvious domain decisions, invariants, migration constraints, or external
   contracts; remove the rest by the deletion test.
4. Claim audit: search the plan, PR draft, commit notes, and issue body for
   unsupported claims from the table above. Downgrade each to precise wording
   backed by current file evidence, or add the missing verification.
5. Minimality check: could the same acceptance criteria be met with fewer changed
   lines, fewer new names, or less custom machinery? Simplify only when behavior
   preservation is clear.

Do not bundle unrelated refactors into one edit set. If a check fails after a
pass, revert that specific change and continue; do not force it through.

Step 4 - Contract check:
- Re-check public API, schema, migration, permission, runtime, and review-thread
  obligations touched by the diff.
- If endpoint/controller/DTO/schema/OpenAPI behavior changed, run the target
  repo's API-doc command first, then the harness API-doc gate against the worktree.

Step 5 - Fresh evidence:
- Run the smallest relevant tests/checks again after the final pass.
- Record exact commands and outcomes.

Output:
- Changed during ai-slop-clean: files and rationale.
- Removed slop: concrete examples per category.
- Preserved intentionally: suspicious-looking code kept on purpose and why.
- Out-of-scope findings: smells outside CLEANUP_BOUNDARY, listed only.
- Verification: fresh commands and results.
- Remaining risks or explicit none.
```

## Gate

Do not move to `pr` until:

- `git -C "$EXPECTED_WORKTREE" diff --check` passes;
- implementation tests or targeted checks have been rerun after the final cleanup pass;
- no unsupported completion claim remains in local plan/PR/commit prose;
- API-doc evidence is recorded when public API behavior changed;
- unrelated edits are reverted or explicitly split out;
- source checkout remains clean for files that should belong only to the worktree.

## Non-Goals

- Do not turn this phase into a full rewrite or style-only review.
- Do not spawn broad review sweeps without path scope, excluded generated paths, and a direct verification fallback.
- Do not let an external LLM approve the work without local file and command evidence.
- Do not delete pre-existing dead code outside CLEANUP_BOUNDARY; surface it instead.
