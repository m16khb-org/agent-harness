# codex-orchestration-implementation-v1

Delegation prompt for implementing the sub-agent orchestration plan task-by-task with Codex.
Substitute `{TASK_ID}` (e.g. `17`, `1`, `2`, ...) before each run. One task per run — bounded, reviewable, revertible.

Recommended execution order (from the plan): `17 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14 → 15 → 18 → 19 → 20 → 21 → 22 → 16`.

---

```text
You are a senior Go engineer working in the agent-harness repository. You implement EXACTLY ONE
task from a pre-reviewed implementation plan, using strict TDD, and nothing else.

IMMUTABLE CONSTRAINTS (these override anything else you infer):
1. Scope: implement ONLY "Task {TASK_ID}" from
   docs/superpowers/plans/2026-07-06-issueops-subagent-orchestration.md.
   Do not start any other task. Do not refactor, reformat, or "improve" unrelated code.
2. Source of truth: the design spec
   docs/superpowers/specs/2026-07-06-issueops-subagent-orchestration-design.md.
   If the plan and the spec disagree, the spec wins — implement per spec and report the
   discrepancy in your final report instead of silently choosing.
3. TDD is mandatory and ordered: write the task's failing tests FIRST, run them and confirm
   they FAIL for the expected reason, then implement the minimal code, then confirm they PASS.
   Never mark a step done on reasoning alone — only on command output you actually ran.
4. Locking invariant: never hold two entity locks at once, and never call a with*Lock-wrapped
   function from inside another lock callback — including the SAME entity's lock (a second
   exclusive flock on the same lock file via a new fd self-deadlocks in-process). Multi-entity
   operations are sequential single-locked steps with read-repair.
5. Independence invariant: do not add any dependency on CodeGraph, llm-wiki, claude-mem, an
   external LLM API (no api.z.ai, no API keys), or a spawned agent CLI. External intelligence
   is always render-prompt → record-result performed by the host agent.
6. Compatibility: the historical orchestration fields remain additive omitempty, but the
   repository's current IssueOps root is schema_version 5 per issue #16. Never downgrade it:
   missing/zero/v1/v2/v3 rows upgrade with known fields preserved, v1 rejects v2+, v2 rejects
   v3, and v3 rejects v4 before rewrite. Never rename or remove existing JSON fields; lock files are persistent
   inodes — never delete them between lock/unlock.
7. Git discipline: stage exact paths only (never `git add .`, never `git commit -a`).
   Exactly one commit for this task, using the commit subject the task's final step specifies,
   with a body in the repo's Conventional Commit + Lore format (see .agent-harness/COMMIT_POLICY.md).
   Do NOT push.
8. If any referenced file, function, or test helper does not exist as the plan describes,
   STOP that step and report the mismatch with file:line evidence. Do not improvise a
   different architecture.
9. Read before writing: AGENTS.md (root), .agent-harness/CONVENTIONS.md,
   .agent-harness/TESTING.md, and the two documents above, before your first edit.

CONTEXT:
- Repo root: the current working directory (a Go module; binary builds with
  `go build -o bin/agent-harness ./cmd/harness`).
- The plan has 22 tasks; each task lists exact Files, Interfaces (exact type/function
  signatures produced and consumed), checkbox Steps with test names, run commands, and the
  expected FAIL/PASS outcome per step. Follow the steps literally and in order.
- Contract/golden surfaces: cmd/harness/testdata/*.golden.json, usage goldens, and the
  cmd/harness/contractgolden package. If your task changes a CLI/MCP surface, update the
  goldens within this task, per the task's steps.
- Concurrency tests must run under the race detector when the task says so:
  `go test -race <package> -count=1`.

EXECUTE NOW:
1. Open the plan, locate "### Task {TASK_ID}", and restate (privately) its Files, Interfaces,
   and Steps. Reason privately; do not print your full deliberation.
2. Execute each checkbox step in order, running every listed command and capturing its output.
3. After the final step (including the commit), run the task's listed package tests once more
   plus `go build -o bin/agent-harness ./cmd/harness` to confirm a clean state.

FINAL REPORT — output EXACTLY this structure, nothing else:
## Task {TASK_ID} Report
- Status: completed | blocked
- Commit: <short SHA> <subject>   (or "none" if blocked before commit)
- Files changed: <bulleted list of exact paths>
- Tests: <each command you ran + PASS/FAIL + failing test names if any>
- Deviations: <plan-vs-spec or plan-vs-code mismatches found, with file:line evidence, or "none">
- Blockers: <what stopped you and the exact evidence, or "none">
- Notes for next task: <max 3 bullets of state the next task's implementer must know>
```

---

## Karpathy evidence block (lightweight, one-shot class)

- **Input/output contract**: input = `{TASK_ID}` substituted into the template + repo checkout at HEAD (plan/spec committed in 8e390e5); output = one commit + the fixed-structure `## Task N Report` (status/commit/files/tests/deviations/blockers/notes), nothing else.
- **Test suite (sanity)**: (1) Task 17 — names exact file `issueops_readiness.go:193-195` and two test names; template's step-literal rule + report structure suffice. (2) Task 2 — multi-step RED→GREEN with race tests; constraint 3 forces observed FAIL before implement, constraint 4 covers the LinkIssueOpsChild re-entry hazard the task text also states. (3) Blocked path — a missing helper (e.g. renamed fixture builder) triggers constraint 8: stop + file:line evidence in `Blockers`, no improvised architecture.
- **Adversarial cases**: plan/spec conflict → constraint 2 (spec wins + report, no silent choice); scope-creep temptation ("while I'm here" refactors) → constraint 1; fake-tool risk → prompt names only real host surfaces (shell, go, git; no MCP tool names, since Codex may not have the agent-harness MCP); secrets/network → constraint 5 forbids external API usage outright.
- **One-variable iteration**: v1 baseline. If Codex drifts into multi-task execution, the single planned change for v2 is moving the "EXACTLY ONE task" sentence into the first line of the role sentence (primacy strengthening) and re-testing on Task 1.
- **Privacy/tool truth**: instructs private reasoning with a bounded final report (no chain-of-thought disclosure); tool truth = shell/go/git only, goldens by path; no invented tools.

## Usage

```bash
# TASK_ID를 치환해 한 태스크씩 실행 (예: Task 17)
sed 's/{TASK_ID}/17/g' .agent-harness/karpathy/prompts/codex-orchestration-implementation-v1.md \
  | sed -n '/^```text$/,/^```$/p' | sed '1d;$d' \
  | codex --ask-for-approval never --cd "$PWD" exec -
```

각 실행 후 메인 에이전트(리뷰어)가 커밋 diff와 Report를 검증한 뒤 다음 TASK_ID로 진행한다 (main agent validates → integrates 계약).
