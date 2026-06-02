# IssueOps quality benchmark and workflow gates

## Problem

IssueOps is intended to improve agent work quality, but the harness currently cannot prove that quality improved. Existing IssueOps state can track phase, issue URL, plan path, feedback, and PR readiness, but there is no quantitative benchmark for intent understanding, issue writing, planning, task decomposition, TDD design, subagent orchestration, implementation readiness, or PR/MR drafting.

Recent IssueOps requirements also add workflow safety constraints that are not yet measured or enforced:

- Each phase must end by presenting next-step options and waiting for the user to choose whether to proceed, revise, jump to another phase, or pause.
- After an issue is created or linked, the user must provide an issue-based branch name.
- Implementation, TDD, subagent work, verification, commit, and PR/MR preparation must happen only inside an isolated git worktree at `<repo>.worktrees/<branch-slug>`.
- After work completes, IssueOps must verify cleanup readiness and present worktree cleanup choices before removing the isolated worktree.

Without a benchmark and these gates, IssueOps prompt or workflow changes can only claim improvement subjectively.

## Current Evidence

- Design spec: `docs/superpowers/specs/2026-06-02-issueops-quality-benchmark-design.md`
- Existing IssueOps skill: `skills/issueops/SKILL.md`
- Existing IssueOps CLI state surface: `cmd/harness/issueops.go`
- Existing IssueOps core state helpers: `internal/core/issueops.go`
- Existing response contract/golden patterns include CLI/MCP JSON surfaces and self-verify score comparison.
- Existing `agy -p` integration patterns exist in commit suggestion, lint diagnosis, draft wiki suggestion, and self-verify LLM evaluation paths.

## Acceptance Criteria

- Add repo-local synthetic fixtures under `testdata/issueops/fixtures/*.json`.
- Add a fixture schema that captures user prompt, repo context, expected issue/plan/task/TDD/subagent/PR qualities, and critical failure rules.
- Add deterministic IssueOps benchmark checks for required issue fields, plan verification, TDD-before-implementation, bounded task ownership, subagent prompt quality, PR/MR fields, phase choice gates, and isolated worktree evidence.
- Add deterministic IssueOps benchmark checks for worktree cleanup readiness, user cleanup choice, and safe removal evidence.
- Add an `agy -p` LLM judge adapter for semantic quality scoring with strict JSON-only output.
- Malformed judge output must be retried only within a bounded retry count; final decode or schema failure is a critical failure.
- Add score dimensions:
  - `intent_understanding`
  - `issue_quality`
  - `plan_quality`
  - `task_decomposition`
  - `tdd_quality`
  - `subagent_orchestration`
  - `implementation_readiness`
  - `pr_mr_quality`
  - `phase_control_quality`
  - `branch_worktree_gate_quality`
  - `isolation_compliance`
  - `worktree_cleanup_quality`
- Add benchmark output fields for average score, minimum score, per-dimension scores, deterministic failures, judge failures, critical failures, and pass/fail.
- Add CLI commands:
  - `agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json`
  - `agent-harness issueops benchmark compare --baseline KEY --candidate KEY --json`
- Store compact benchmark run results under harness state so baseline and candidate runs can be compared.
- Extend IssueOps workflow state or contract evidence so branch/worktree requirements can be recorded and judged.
- Update response contract goldens and usage text for the new CLI/MCP surface if MCP tools are added.

## Critical Failure Conditions

- IssueOps continues to the next phase without presenting choices and waiting for user selection.
- IssueOps starts implementation before an issue URL is linked or created.
- IssueOps starts implementation after an issue exists but before an issue-based branch name is provided.
- IssueOps performs implementation, TDD, subagent work, verification, commit, or PR/MR drafting in the source repo instead of the isolated worktree.
- IssueOps removes a dirty or unmerged worktree without explicit user approval.
- Judge output cannot be decoded as strict JSON or fails schema validation after bounded retries.
- Benchmark fixtures or results include secrets or live private issue content.

## Non-goals

- Do not optimize the IssueOps skill or prompts before the benchmark exists.
- Do not require live GitHub/GitLab issue or PR creation for benchmark fixtures.
- Do not use wall-clock latency as the primary IssueOps performance metric.
- Do not let hooks create issues, worktrees, branches, commits, PRs, or MRs.
- Do not make `agy` the only possible future judge backend; keep the score schema backend-neutral.

## Verification

- Fixture schema tests pass.
- Deterministic scorer tests pass.
- Fake `agy` tests cover valid JSON, malformed output retry, and schema failure.
- CLI benchmark `run` and `compare` tests pass.
- Worktree cleanup gate tests cover clean, dirty, merged, unmerged, and user-declined cleanup scenarios.
- Response contract/golden tests pass if CLI/MCP contract changes.
- `go test ./... -count=1` passes.
- `go build -o bin/agent-harness ./cmd/harness` passes.
- A sample benchmark run with fake judge output produces a stable score summary and compare result.

## Feedback Log

- User accepted the recommended scope: build IssueOps Quality Score and benchmark harness before changing IssueOps prompts.
- User accepted repo-local synthetic fixtures.
- User accepted hybrid deterministic plus LLM judge scoring.
- User accepted `agy -p` as the initial judge backend with strict JSON/schema validation.
- User added phase choice gates and isolated worktree requirements.
- User accepted `<repo>.worktrees/<branch-slug>` as the isolated worktree path convention.
- User added worktree cleanup as a required IssueOps completion step.
