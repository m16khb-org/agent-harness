---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, AI slop cleanup, feedback loops, and PR/MR drafting.
---

# IssueOps

Use this skill when the user wants a repeatable cycle from a vague problem to a GitHub/GitLab issue, implementation plan, tested change, AI slop cleanup, feedback loop, and PR/MR.

This file is the phase router. Load only the referenced phase document needed for the current step.

## Core Contract

The workflow is advisory and agent-driven. Hooks may suggest this skill, but hooks must not create issues, edit files, run tests, wait on background jobs, or open PRs/MRs by themselves.

The cycle has one durable state record. Use `agent-harness issueops ... --json` or matching MCP tools when the cycle needs to survive compaction, handoff, or another host.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: before remote issue creation, run the issue-preflight gate in `references/issue-preflight.md`; record the raw user request, interpreted intent, success criteria, constraints, non-goals, and ambiguity ledger with `agent-harness issueops intent record`; then create or prepare a GitHub/GitLab issue with problem, acceptance criteria, non-goals, verification, and open decisions.
4. Plan: produce an issue-based implementation plan under the target repo's planning convention, then record the reviewed design, refactor boundary, risks, alternatives, and verification matrix with `agent-harness issueops design review`.
5. Implementation: use TDD for behavior changes and subagents only for bounded independent work. Do not enter implementation until the IssueOps design review is approved and has no open questions.
6. AI slop clean: before PR/MR drafting, load `references/ai-slop-clean.md` and remove lazy agent artifacts such as vague explanations, unverified claims, overbroad abstractions, dead scaffolding, generic comments, noisy generated prose, and brittle shortcuts; keep only evidence-backed, repo-style code/docs/tests.
7. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
8. PR/MR: draft only after the issue URL, provider-linked branch, plan path, and isolated worktree are linked, AI slop cleanup is complete in that worktree, strict PR readiness is green, and relevant verification has run.

## Agent-Harness Phase Assist Map

IssueOps phases are supported by three agent-harness native skills — no external plugin dependencies required. These replace the legacy LazyCodex/OMO mapping. Each skill works standalone or integrated; when an IssueOps cycle exists, state is persisted through `agent-harness` CLI/MCP.

| IssueOps phase | Agent-harness assist |
| --- | --- |
| problem | Use **`archimedes`** when the request spans multiple modules, has unclear scope, or needs a decision-complete plan. Archimedes follows "Explore Before Asking" — it grounds itself in the actual codebase before interviewing the user. |
| grill | Use **`archimedes`** Phase 1 (Ground) for codebase exploration, pattern discovery, and brownfield detection. Use CodeGraph for structural call paths and impact analysis before creating the issue contract. |
| issue | Run the issue-preflight deep-interview gate: use **`archimedes`** Phase 2 (Interview + Clearance Checklist) to reduce ambiguity, rewrite the raw user request into an ideal issue prompt using repo-root `PROMPT.md`, and carry an ambiguity ledger with resolved/deferred/blocking entries. Keep remote writes in the IssueOps remote artifact gates. |
| plan | Use **`archimedes`** Phase 3 (Plan Generation) to produce a decision-complete plan at `.agent-harness/plans/<slug>.md`. Link it with `agent-harness issueops link-plan`. Archimedes plans include a dependency matrix, parallel execution waves, and per-task QA scenarios — no implementation until the clearance checklist passes and the plan is complete. |
| implement | Use **`turing`** for evidence-bound execution with RED→GREEN→SURFACE→CLEAN TDD, per-criterion Manual-QA across 4 channels (HTTP/tmux/browser/computer-use), and quantitative metrics (evidence coverage, rework rate, cycle efficiency). Delegate every code edit, test write, and QA to right-sized workers. Use **`von-neumann`** when the Archimedes plan has 5+ TODOs that can be parallelized into dependency-ordered waves. |
| ai-slop-clean | Use **`turing`** Final Quality Gate step 2 (AI slop clean + re-verify). Inspect the actual worktree diff for lazy agent artifacts, unsupported claims, generic prose, dead scaffolding, unnecessary abstractions, weak comments, and brittle shortcuts. Remove them or record why they are intentional before moving to `pr`. |
| feedback | Use **`turing`** Dynamic Steering to record feedback as structured evidence. For contract-changing feedback, update the remote issue body before continuing. For review feedback, answer in the original thread with verdict, evidence, and next action. |
| pr | Use **`von-neumann`** Verification Gate: spawn a dedicated reviewer agent with the full diff, all success criteria, and all evidence. The reviewer verdict is BINDING — unconditional approval only. Keep Korean remote artifact, label, assignee, and strict readiness checks in IssueOps. |
| cleanup | Use **`turing`** cleanup receipt rules: every QA resource (PIDs, tmux sessions, browser contexts, ports, temp files) must be torn down with a recorded receipt. Keep merge evidence and worktree/branch cleanup decisions in `references/cleanup-state.md`. |

## Reference Map

Load these files only when the phase applies:

- `references/remote-issue.md`: remote issue first, related issue/label scoring, external LLM judge contract, Korean remote artifact gate, issue template.
- `references/issue-preflight.md`: deep-interview ambiguity reduction and `PROMPT.md`-based ideal issue prompt rewrite before remote issue creation.
- `references/evidence-contract.md`: portable domain contract, API documentation, live evidence, review accountability, and completion hygiene rules.
- `references/worktree-context.md`: branch/worktree contract, local config symlink rules, context routing.
- `references/ai-slop-clean.md`: PR/MR-prep cleanup prompt for removing lazy agent residue while preserving behavior.
- `references/review-feedback.md`: worker prompt requirements, bounded subagent review rules, remote review feedback replies and thread resolution.
- `references/cleanup-state.md`: post-merge cleanup, state commands, benchmark commands, stop conditions.

## Always-On Rules

- Remote issue first: when `$issueops` is explicitly invoked and repo remote, credentials, target project, branch target, and issue ownership are discoverable, create or link the remote issue before planning or implementation.
- Linked branch first: IssueOps branches must start with the issue/task number followed by a hyphen so GitLab links them in the issue Development section. Use names like `2387-fix-grpc-ai-dmm-tag-replication-lag` or `2386-remove-dmm-ranking-ranktype`; do not put `feature/` or `hotfix/` before the issue number.
- Worktree first: after issue link and before implementation, create an isolated worktree under `../<repo>.worktrees/<branch-slug-with-slashes-replaced>` and run implementation from that path.
- Edit-target guard: shell cwd checks are not enough. Before any file edit, ensure the edit tool target path is inside the expected isolated worktree; after the edit, verify the source checkout/main branch remains clean and the worktree owns the change.
- State first: link the issue and plan in IssueOps state before PR/MR drafting.
- Intent contract first: before entering `plan`, record the raw request, interpreted intent, success criteria, constraints, non-goals, and ambiguity ledger with `agent-harness issueops intent record`; the durable state must show the main agent's judgment, not only hook recommendations.
- Design review first: before linking a plan or entering `implement`, record an approved `agent-harness issueops design review` with problem summary, proposed design, refactor plan or boundary, risks, alternatives, and verification. Approved design reviews must not carry open questions.
- TDD first: for behavior changes, write or update focused tests before production changes.
- Evidence contract first: before implementation, record the domain invariant, exact mechanism, equivalent behavior if any, source evidence, changed endpoint/API-doc needs, live runtime matrix needs, review-thread obligations, and completion hygiene checks. Load `references/evidence-contract.md` when any of those surfaces apply.
- Verify before remote writes: run the Korean Remote Artifact Gate before creating or editing remote issues, PRs, or MRs.
- Korean remote hook guard: installed PreToolUse hooks include `--enforce-korean-remote-artifacts` and block `gh issue/pr create/edit` when an inspectable title/body fails the Korean remote artifact gate.
- VCS linking hook guard: installed PreToolUse hooks include `--enforce-vcs-issue-linking` and block `gh`/`glab` issue create/edit when the body carries a `Plan Link` section or, on GitLab, a `Related Issues` body section (related issues belong in native linked items). The same guard blocks remote issue/PR/MR create commands without labels or assignee; copy linked issue labels for PR/MR create or pass an explicit manual label, and assign the artifact to the current user. See `references/remote-issue.md` -> "Provider-Specific Linking And Hierarchy".
- No broad review sweeps: subagent reviews must have explicit included paths, excluded large/generated paths, a time budget, and a fallback direct verification path.
- Cleanup choices: after a PR/MR is merged, verify merge/worktree/branch status and present numbered cleanup choices before deleting local worktrees or branches.
- Numbered next actions: at user decision points and after reporting review/feedback/cleanup status, end with `선택지:` and three numbered choices. Installed Stop hooks with `--enforce-numbered-next-actions` block missing choices and tell the agent to explain the block before presenting context-specific choices.
- Next-action Stop hook: to reduce friction, the Stop hook may re-enter the main agent when the final response contains an explicit `선택지:` section. The hook is not a judge, scorer, classifier, or safety gate; it relays observed facts such as choice count, recommendation count, and recommended text. The main agent must judge safety, reversibility, user-intent alignment, and whether to proceed or ask the user from the current context, then state why it is auto-proceeding or why it is not auto-proceeding and needs user confirmation. Auto-proceed result reports still end with `선택지:` so the next action boundary remains explicit. Mark exactly one recommended option only when the main agent itself judges it safe, reversible, and aligned.
- Worker identity check: every implementation, TDD, review, QA, or subagent worker must first report and verify `pwd`, branch, `HEAD`, and the expected isolated worktree path before inspecting or changing anything.
- Remote artifact ownership: created issues and PRs/MRs must be assigned to the currently authenticated user when the provider supports assignment, and assignment must be verified before reporting readiness.
- Remote issue source of truth: when feedback changes scope, acceptance criteria, non-goals, verification, labels, related links, or implementation contract, update the remote issue body before continuing.
- Review thread accountability: remote review feedback must be answered in the original review thread/discussion with verdict, evidence, and next action; do not report feedback cleared until addressed threads are replied to, resolved when appropriate, and re-checked.
- AI slop clean before PR/MR: after implementation and before PR/MR drafting, inspect the actual worktree diff for lazy agent artifacts, unsupported claims, generic prose, dead scaffolding, unnecessary abstractions, weak comments, and brittle shortcuts. Remove them or record why they are intentional before moving to `pr`.
- Completion hygiene: before reporting done, verify the final diff, target branch, remote issue/PR/MR prose freshness, single-commit or declared commit policy, and cleanup/worktree status.
- External LLM wrapper: all IssueOps `agy -p` usage must go through the shared harness external LLM wrapper and remain read-only judgment.

## Gate Quick Reference

When an IssueOps command reports a missing gate, do not guess a new hidden flag. Use the command that owns that state:

- `intent_contract`: run `issueops intent record` with raw request, interpreted intent, success criteria, constraints/non-goals/ambiguity when known.
- `branch_prepare` / `branch_link_verified`: run `issueops branch prepare` only after provider-visible branch evidence exists. The branch must start with the issue/task number and a hyphen.
- `worktree_path` / `worktree_exists`: create the sibling isolated worktree first, then run `issueops link-worktree`.
- `design_review`, `design_approval`, `design_review_evidence`, `refactor_plan`, `alternatives`, `risks`, `design_open_questions`: run one full `issueops design review` call. Approval is recorded with the full design review payload; there is no approve-only merge step.
- `plan_path` / `plan_exists` / `plan_in_worktree`: create the plan file inside the linked worktree, then run `issueops link-plan`.
- `ai_slop_clean`: record AI slop cleanup after implementation changes exist in the linked worktree.
- `contract_feedback_issue_update`: update the remote issue body for contract-changing feedback, then run `issueops feedback mark-issue-updated`.

Approved design reviews require `--refactor-plan`, at least one `--alternative`, at least one `--risk`, no `--open-question`, and at least one design-review evidence verification item. `design_review_evidence` is not a separate CLI flag, MCP field, or decision record. Put it in `--verification`, for example:

```bash
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --refactor-plan "$REFACTOR_PLAN" \
  --alternative "$ALTERNATIVE" \
  --risk "$RISK" \
  --verification "design review checked alternatives and risks" \
  --verification "go test ./..." \
  --approved \
  --json
```

## Quality Upgrade Gates

IssueOps must leave an auditable decision trail for labels, large issue breakdown, draft issue completion, and PR/MR review-agent feedback.

- Before any remote issue or PR/MR write, record the **threshold-based label decision**: selected labels, rejected labels, and manual override reason when no label crosses threshold. Use `issueops remote score` first, then apply only selected labels or stop before writing.
- For broad or multi-step work, run the **Large Issue Breakdown Gate**: create provider-native child work items before implementation when the parent issue would otherwise hide independent tasks. Use GitHub sub-issues or GitLab child items, then record each existing child with `agent-harness issueops link-child`.
- On completion, write a **draft issue completion record** in the remote issue or PR/MR-ready notes before reporting done. It must summarize final diff, verification evidence, selected labels, child links, PR/MR URL, cleanup status, and unresolved follow-ups.
- Treat Kodus, Gemini Code Assist, and similar automated reviewers as **review-agent feedback**. Verify each claim, reply in the original thread with verdict and evidence, and resolve only threads whose fix or obsolescence has been verified.

Use this remote issue scoring choice shape before creating or editing an issue:

```text
관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 agy judge, 실패 시 deterministic fallback으로 진행합니다.
```

Use this review thread reply shape:

```text
타당성: 타당

근거:
- <파일:라인 또는 명령 결과 근거>
- <계약/테스트 근거>

다음 조치: <수정 진행|별도 PR 분리|보류 사유>
```

After posting review-thread replies, report numbered next actions:

```text
선택지:
1. 진행: 테스트를 먼저 추가하고 결함을 수정합니다. (추천)
2. 축소 진행: 일부 검증만 먼저 수정하고 나머지는 별도 PR로 분리합니다.
3. 보류: 현재 PR에는 수정하지 않고 리뷰 스레드에 검증 결과만 답변합니다.
```

Use this cleanup choice shape:

```text
선택지:
1. 정리 진행: merged PR/MR worktree와 local branch를 삭제합니다. (추천)
2. 보류: worktree는 유지하고 나중에 확인합니다.
3. 확장 정리: merged/stale IssueOps worktree 전체를 점검하고 정리 후보를 제시합니다.
```

For the "확장 정리" choice, scan abandoned cycles across the repo with multi-signal liveness rather than a single age threshold. Default is report-only; pass `--apply` (CLI) or `apply: true` (MCP `issueops_cleanup_stale`) only after presenting the findings and getting a decision.

```bash
agent-harness issueops cleanup stale --repo "$REPO" --json            # dry-run: classify only
agent-harness issueops cleanup stale --repo "$REPO" --apply --json     # force-release confirmed-stale + likely-done
```

- `confirmed-stale` (worktree deleted / non-git / HEAD≠record.Branch) and `likely-done` (remote branch merged or absent for a pr-stage/artifact-bearing cycle) are `releasable`; `--apply` force-releases them.
- `needs-review` (idle past `--max-age`, default 14 days) is **never** auto-released — surface it for a human decision.
- Starting a new cycle on the same branch whose worktree was deleted auto-resets the stale `implement`/`ai-slop-clean`/`feedback` cycle to a fresh `problem` record (issue linkage preserved); a `pr`-phase cycle is resumed, not reset, so remote linkage survives.

## Background LLM Gates

Remote scoring is a `background_join` LLM gate. It may run while local planning or implementation continues, but the main IssueOps loop must join the result before any remote artifact write: issue create/edit, label create/apply, PR/MR create/edit, assignment, or comment. If label candidates existed but none met threshold, stop before remote writes and choose an explicit manual label or rerun scoring with corrected candidates; do not create an unlabeled issue, PR, or MR.

Do not put polling or waiting in lifecycle hooks. Hooks may surface a status hint only. Completion is decided by the main loop at the join point by checking the stored job/result status and requiring success before the remote write.

External LLM judges are read-only evaluators. Their prompts must forbid workspace inspection, tool execution, file changes, git actions, issue/label/PR/MR mutation, comments, assignment, closing/reopening, or state changes. They may only return judgment JSON that the main loop applies after validation.

## Operational Start

Start or resume state after deriving the issue branch slug. The IssueOps branch must be the issue branch, not the source checkout's current branch:

```bash
branch_slug="3-webhook-delivery"
agent-harness issueops start --repo "$PWD" --branch "$branch_slug" --json
agent-harness issueops status --id "$ISSUEOPS_ID" --json
```

Record the intent contract before plan phase or issue-link auto-advance:

```bash
agent-harness issueops intent record --id "$ISSUEOPS_ID" \
  --raw-request "$RAW_USER_REQUEST" \
  --interpreted-intent "$INTERPRETED_INTENT" \
  --success-criteria "$SUCCESS_CRITERION" \
  --constraint "$CONSTRAINT" \
  --ambiguity "$AMBIGUITY_LEDGER_ENTRY" \
  --non-goal "$NON_GOAL" \
  --json
```

Remote issue and plan linkage:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" --issue-url "$ISSUE_URL" --branch "$branch_slug" --base-branch "$BASE_BRANCH" --link-verified --json
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" --worktree-path "$EXPECTED_WORKTREE" --json
agent-harness issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "$PROBLEM_SUMMARY" \
  --proposed-design "$PROPOSED_DESIGN" \
  --refactor-plan "$REFACTOR_PLAN" \
  --risk "$RISK" \
  --alternative "$ALTERNATIVE" \
  --verification "$VERIFICATION_STEP" \
  --approved \
  --json
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$EXPECTED_WORKTREE/$PLAN_REL_PATH" --json
agent-harness issueops link-child --id "$ISSUEOPS_ID" --child-url "$CHILD_ISSUE_URL" --title "$CHILD_TITLE" --json
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json
```

`branch prepare` records the required provider-linked branch contract before local worktree creation: use the provider MCP first, use the provider API/CLI fallback second, and fail closed if both cannot create a branch that the issue shows as linked. For GitLab, branch names must start with the issue or task number followed by a hyphen, for example `123-fix-login`.

`link-worktree` fails closed until issue-linked branch evidence exists and the worktree path already exists on disk. `link-plan` is the transition into implementation. It fails closed until the issue is linked, `branch prepare --link-verified` has recorded provider-visible branch evidence, the worktree is linked, the design review is approved with no open questions, and the plan path exists inside that linked worktree.

`link-child` records a provider-native child work item after it exists remotely. On GitHub that child should be a sub-issue; on GitLab it should be a child item/task. The command does not create remote issues and must not be used as a substitute for the provider-specific hierarchy rules.

Advance the lifecycle phase (problem, grill, plan, implement, ai-slop-clean, feedback, pr, done). The `ai-slop-clean` phase requires linked issue, provider-linked branch, plan, an existing linked worktree, and implementation changes under that worktree. The `pr` phase requires strict PR readiness, including ai-slop-clean evidence. The `done` phase requires the loop to have already entered `pr` and a verified remote PR/MR artifact with provider URL, label, and assignee evidence:

```bash
agent-harness issueops phase --id "$ISSUEOPS_ID" --to grill --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to ai-slop-clean --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to pr --json
agent-harness issueops remote verify-artifact --id "$ISSUEOPS_ID" --provider "$PROVIDER" --kind pr|mr --url "$PR_URL" --label "$LABEL" --assignee "$ASSIGNEE" --json
agent-harness issueops phase --id "$ISSUEOPS_ID" --to done --json
```

Post-merge cleanup status is a read-only verification step. Run it after the provider reports the PR/MR merged and before deleting worktrees or branches:

```bash
agent-harness issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
```

Record feedback, optionally classifying each item (contract_change, defect, question, noise) so contract-changing feedback is distinguishable:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source review --body "$FEEDBACK" --classification contract_change --json
agent-harness issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" --json
```

Remote scoring runs deterministically and is also available as the MCP tool `issueops_remote_score` for cross-host (Codex/Claude) use; the agy judge path stays CLI/`remote score --judge agy`. Benchmark commands (`benchmark run|compare|gate`) are CLI-only developer/autoresearch tooling, not a runtime MCP gate.

## Stop Conditions

Stop and ask before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Stop before implementation if `issueops intent record` or `issueops design review` cannot be completed from evidence. Do not treat a recommended next-action option as permission to continue unless the main agent records why continuation is safe, reversible, and aligned with the user's latest instruction.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `issue_url`, `branch_prepare`, `branch_link_verified`, `plan_path`, `worktree_path`, `worktree_exists`, `branch_match`, `worktree_clean`, `upstream`, `upstream_synced`, `plan_exists`, or `ai_slop_clean`.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `contract_feedback_issue_update`. This means a `contract_change` feedback item was recorded after the remote issue contract changed, and the remote issue body update has not been confirmed with `issueops feedback mark-issue-updated`.

Do not mark an IssueOps loop `done` before it has entered the `pr` phase. Completion reporting happens after PR/MR readiness and review/merge hygiene, not as an escape hatch from planning or implementation.

Before PR/MR create, verify the linked issue labels and pass them to the provider create command. If the linked issue has no labels, create or apply an explicit manual label first, or stop and record label-decision feedback; never create the PR/MR with an empty label set.
