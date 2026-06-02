# IssueOps Quality Benchmark Design

## Problem

IssueOps should improve the quality of agent work, not just the speed of a CLI cycle. The harness currently has an IssueOps workflow and durable state, but it has no quantitative way to prove that IssueOps understands user intent better, writes better issues, creates better plans, decomposes tasks more safely, applies TDD better, distributes subagent work better, or prepares better PR/MR drafts.

Without a benchmark, prompt or workflow changes can only claim improvement subjectively. The next IssueOps improvement must first create a repeatable quality score and comparison protocol.

## Success Criteria

- IssueOps has repo-local synthetic fixtures that can be run without GitHub/GitLab credentials.
- A benchmark run scores IssueOps artifacts across intent, issue, plan, task, TDD, subagent, implementation-readiness, and PR/MR dimensions.
- Scoring combines deterministic checks with an LLM judge.
- The initial LLM judge backend uses `agy -p`, but the score schema is backend-neutral.
- Judge output must decode as strict JSON and pass schema validation; decode or schema failure is a critical failure.
- Benchmark compare can prove whether a candidate improves over a baseline using average score, minimum score, critical failure count, and per-dimension regressions.
- The benchmark also checks IssueOps workflow contract quality: phase choices after every step and isolated worktree gating after issue creation.
- IssueOps completion includes worktree cleanup readiness and removal guidance after the branch is merged, abandoned, or otherwise safe to dispose.

## Non-Goals

- Do not optimize the IssueOps skill or prompts before the benchmark exists.
- Do not require live GitHub/GitLab issue or PR creation in benchmark fixtures.
- Do not use wall-clock latency as the primary success metric.
- Do not store secrets, credentials, or real private issue bodies in fixtures or benchmark state.
- Do not let hooks create issues, worktrees, commits, branches, PRs, or MRs.

## Domain Language

- **IssueOps Quality Score**: a normalized benchmark score for one IssueOps run over one or more fixtures.
- **Fixture**: a repo-local synthetic problem scenario with user request, repo context, expected quality criteria, and critical failure rules.
- **Artifact bundle**: the IssueOps outputs being judged for a fixture, such as problem summary, issue draft, plan, task breakdown, TDD plan, subagent prompts, implementation notes, and PR/MR draft.
- **Deterministic check**: a parser or rule that verifies required structure, fields, commands, links, phase gates, and isolation evidence.
- **LLM judge check**: an `agy -p` evaluation that scores semantic quality against the fixture rubric.
- **Critical failure**: a failure that prevents passing regardless of average score.
- **Phase choice gate**: the requirement that each IssueOps phase ends by presenting next-step options and waiting for user choice.
- **Isolated worktree gate**: the requirement that implementation work only begins after an issue-based branch is provided and an isolated git worktree is created.
- **Worktree cleanup gate**: the completion requirement that IssueOps verifies the isolated worktree is clean, merged or safely disposable, and then presents cleanup choices before removing it.

## Benchmark Fixture Schema

Fixtures live under `testdata/issueops/fixtures/*.json`.

Each fixture contains:

- `id`: stable fixture id.
- `title`: short human-readable name.
- `user_prompt`: the problem statement given to IssueOps.
- `repo_context`: compact synthetic repository context.
- `expected_issue`: issue-quality requirements.
- `expected_plan`: plan-quality requirements.
- `expected_tasks`: task decomposition requirements.
- `expected_tdd`: TDD requirements.
- `expected_subagents`: subagent distribution and prompt-injection requirements.
- `expected_pr`: PR/MR draft requirements.
- `critical_failures`: fixture-specific critical failures.

The first fixture set should cover at least:

- ambiguous user intent that requires clarification before implementation,
- issue creation followed by branch/worktree gating,
- feedback that changes the issue or plan contract,
- subagent work that must be split without overlapping file ownership.

## Scoring Model

Each dimension is scored from 0 to 5.

Required dimensions:

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

The result includes:

- `average_score`
- `minimum_score`
- `dimension_scores`
- `critical_failures`
- `deterministic_failures`
- `judge_failures`
- `passed`

`passed` is true only when there are no critical failures and the configured score threshold is met.

## Deterministic Checks

The deterministic scorer verifies structure and hard workflow rules:

- Issue draft includes problem, current evidence, acceptance criteria, non-goals, verification, and feedback log.
- Plan includes explicit tests or verification commands.
- Task breakdown has bounded ownership and avoids conflicting file responsibilities.
- TDD artifact identifies failing tests before implementation work.
- Subagent prompts include task ownership, context, expected output, and "not alone in the codebase" coordination guidance.
- PR/MR draft includes intent, changes, verification, risks, and issue link.
- Every completed phase presents next-step choices and does not silently continue.
- After issue creation, implementation is blocked until the user provides an issue-based branch name.
- The branch worktree path follows `<repo>.worktrees/<branch-slug>`.
- Implementation, TDD, and subagent work reference the isolated worktree, not the source repo.
- Completion artifacts include worktree cleanup status and final cleanup choices.

These checks should run without an LLM and should be deterministic enough for CI.

## LLM Judge

The initial judge backend is `agy -p`.

The judge prompt must:

- provide the fixture, artifact bundle, rubric, and output schema,
- instruct the judge to return JSON only,
- prohibit prose before or after JSON,
- require every score to include a short evidence string,
- require critical failures to cite the violated rule.

The harness must:

- execute `agy -p` through an explicit judge adapter,
- parse the first successful strict JSON response,
- validate the schema,
- retry only within a small bounded retry count when output is malformed,
- record decode/schema failure as a critical failure when retries fail.

The score schema must not depend on Gemini-specific fields so another judge backend can be added later.

## CLI Surface

Add benchmark subcommands under `issueops`.

```bash
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json
agent-harness issueops benchmark compare --baseline KEY --candidate KEY --json
```

Expected run output:

- benchmark id or state key,
- fixture count,
- per-fixture scores,
- aggregate scores,
- critical failures,
- judge backend metadata,
- deterministic check summary.

Expected compare output:

- baseline and candidate ids,
- average score delta,
- minimum score delta,
- critical failure delta,
- per-dimension regressions,
- pass/fail status.

## Phase And Worktree Contract

IssueOps must not act like a linear autopilot.

After each phase, it must present choices such as:

- proceed to the recommended next phase,
- revise the current phase,
- jump to another phase,
- pause and wait.

When an issue is created or linked:

1. IssueOps asks the user for the issue-based branch name.
2. IssueOps creates an isolated git worktree at `<repo>.worktrees/<branch-slug>`.
3. IssueOps records the branch and worktree path.
4. IssueOps performs implementation, TDD, subagent work, verification, commit, and PR/MR preparation only inside that worktree.

Skipping this gate is a critical failure.

When the work is complete:

1. IssueOps checks whether the isolated worktree is clean.
2. IssueOps verifies whether the branch has been merged, pushed, or explicitly abandoned.
3. IssueOps presents cleanup choices instead of removing the worktree silently.
4. IssueOps removes the worktree only after the user chooses cleanup and the safety checks pass.

Leaving completed worktrees around without a cleanup prompt is a benchmark regression. Removing a dirty or unmerged worktree without explicit user approval is a critical failure.

## State And Storage

Benchmark fixtures are source-controlled under `testdata/issueops/fixtures`.

Benchmark run outputs should be compact enough to save under harness state, not the target repository. The saved result should include scores, critical failures, judge metadata, and enough fixture/artifact identifiers to compare baseline and candidate runs.

IssueOps workflow state should be extended later to include branch and worktree evidence. Runtime worktree state must remain outside the target repository source tree except for the git worktree itself.

## Verification

Implementation should be verified with:

- fixture schema tests,
- deterministic scorer tests,
- fake `agy` judge tests for valid JSON, malformed output retry, and schema failure,
- CLI benchmark run and compare tests,
- worktree cleanup gate tests for clean, dirty, merged, unmerged, and user-declined cleanup scenarios,
- response contract golden updates,
- `go test ./... -count=1`,
- `go build -o bin/agent-harness ./cmd/harness`,
- a smoke benchmark run against sample fixtures using fake judge output.

## Open Decisions

- Exact default score threshold.
- Whether benchmark results should integrate with `self-verify compare` or remain IssueOps-specific at first.
- Exact wording of the worktree cleanup choices presented to the user.
