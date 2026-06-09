# AI Slop Clean

Use this phase after implementation and before PR/MR drafting. The goal is not broad refactoring; it is a focused cleanup pass that removes lazy agent residue while preserving behavior.

## Evidence Used

This prompt pattern is adapted from established code review and AI-slop detection practices, and aligns with the **`turing`** skill's Final Quality Gate step 2 (AI slop clean + re-verify) and Reviewer Gate:

- **`turing`** Cleanup Phase: inspect the worktree diff for lazy agent artifacts, unsupported claims, generic prose, dead scaffolding, unnecessary abstractions, weak comments, and brittle shortcuts.
- **`turing`** Reviewer Gate: spawn a dedicated reviewer agent; every concern is real and binding.
- Code review best practices: verify claims against actual files, look for what is missing, rate findings by severity, separate discovery from filtering.
- Verification principle: no completion claim without fresh command evidence.

## Prompt

Run this from the exact IssueOps worktree, after tests for the implementation phase have passed at least once:

```text
You are running the IssueOps ai-slop-clean phase.

Scope:
- Work only in the expected IssueOps worktree: <EXPECTED_WORKTREE>.
- Inspect only the current task diff and directly related touched files unless a concrete finding requires one-hop context.
- Preserve behavior. Do not add features, broaden scope, rewrite architecture, or reformat unrelated code.

Inputs:
- Issue URL: <ISSUE_URL>
- Plan path: <PLAN_PATH>
- Worktree branch: <BRANCH>
- Diff command: git -C <EXPECTED_WORKTREE> diff --stat && git -C <EXPECTED_WORKTREE> diff
- Verification commands already run: <COMMANDS_AND_RESULTS>

Passes:
1. Diff reality check:
   - Confirm every changed file traces to the issue/plan.
   - Flag unrelated edits, generated noise, broad formatting churn, and files changed only because the agent wandered.
2. Slop removal:
   - Remove obvious comments that restate code.
   - Remove vague TODOs, placeholders, "temporary" scaffolding, unused helpers, dead branches, debug logs, console prints, and speculative abstractions.
   - Replace generic names, overly clever code, nested conditionals, and one-off wrappers when a simpler local pattern exists.
   - Keep comments that explain non-obvious domain decisions, invariants, migration constraints, or external contracts.
3. Claim audit:
   - Search the plan/PR draft/commit notes for claims like "all", "always", "guarantees", "complete", "safe", or "verified".
   - Keep only claims backed by current file evidence or fresh command output.
   - Downgrade unsupported claims to precise wording or add the missing verification.
4. Contract check:
   - Re-check public API, schema, migration, permission, runtime, and review-thread obligations touched by the diff.
   - If endpoint/controller/DTO/schema/OpenAPI behavior changed, run the target repo's API-doc command first, then the harness API-doc gate against the worktree.
5. Minimality check:
   - Ask whether the same issue acceptance criteria could be met with fewer changed lines, fewer new names, or less custom machinery.
   - Simplify only when behavior preservation is clear.
6. Fresh evidence:
   - Run the smallest relevant tests/checks again after cleanup.
   - Record exact commands and outcomes.

Output:
- Changed during ai-slop-clean: files and rationale.
- Removed slop: concrete examples.
- Preserved intentionally: any suspicious-looking code kept and why.
- Verification: fresh commands and results.
- Remaining risks or explicit none.
```

## Gate

Do not move to `pr` until:

- `git -C "$EXPECTED_WORKTREE" diff --check` passes;
- implementation tests or targeted checks have been rerun after cleanup;
- no unsupported completion claim remains in local plan/PR/commit prose;
- API-doc evidence is recorded when public API behavior changed;
- unrelated edits are reverted or explicitly split out;
- source checkout remains clean for files that should belong only to the worktree.

## Non-Goals

- Do not turn this phase into a full rewrite or style-only review.
- Do not spawn broad review sweeps without path scope, excluded generated paths, and a direct verification fallback.
- Do not let an external LLM approve the work without local file and command evidence.
