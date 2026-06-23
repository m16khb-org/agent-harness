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

IssueOps is a main-agent state machine:

```text
problem -> grill -> issue -> plan -> implement -> ai-slop-clean -> feedback -> pr -> cleanup
```

`agent-harness issueops ...` CLI/MCP commands own durable state, phase transitions, readiness, and remote artifact records. Lifecycle hooks are limited to fast, deterministic, inspectable guards and routing hints. A hook may block a clearly invalid tool event, but it must not perform workflow work: no issue creation, provider mutation, file edit, test run, background wait, branch/worktree preparation, PR/MR creation, review reply, merge, or cleanup.

### Flow Boundary Matrix

| Phase area | Automatic main-agent loop | Hook enforcement | Human-in-the-loop |
| --- | --- | --- | --- |
| Problem / Grill | Gather repo/docs/code/runtime evidence; draft intent class; ask only on blocking ambiguity. | UserPromptSubmit routing hint only. | Success criteria, scope, or terminology changes the implementation. |
| Issue Contract | Record intent; run issue-preflight; score related issues and labels; pass Korean artifact gate; create/link the remote issue when credentials, target, owner, and base branch are clear. | Korean remote artifact, VCS linking metadata, missing label, and missing assignee guards. | Credentials, target project, owner, base branch, or selected label is unclear. |
| Large Issue Breakdown | Decide split/no-split; create provider-native child tasks; verify hierarchy, labels, assignee, and parent body. | VCS artifact guard blocks body-only hierarchy when native linking is required. | Child scope is a product decision or provider hierarchy support/permission is missing. |
| Plan / Design Review | Record plan-prep evidence or waivers; write the plan; record approved design review with risks, alternatives, and verification. | No hook enforcement. | Open question, risky alternative, or refactor boundary remains unresolved. |
| Branch / Worktree Prep | Create provider-linked branch; create/link sibling worktree; link plan; run `worktree prepare-tools`; record `execution_decision` before `implement`. | Worktree guard blocks source-checkout mutation and wrong linked-worktree targets. | Base branch is user-selected/unclear/conflicting, or dependency/config linking has secret risk. |
| Implementation | Main agent runs TDD, focused fixes, and verification directly unless `execution_decision` records a net-positive sub-agent plan. | Worktree guard; staged-check and live-command guards where installed. Hooks do not decide sub-agent usage. | Failure classification, destructive migration, live access, product behavior, or sub-agent tradeoff judgment is unclear. |
| AI Slop Clean | Inspect the actual diff, remove lazy artifacts, rerun targeted checks, record cleanup evidence. | Worktree guard remains active. | Cleanup would widen scope or require behavior changes. |
| Feedback | Classify CI/review/user feedback; fix valid items; update remote issue body for contract changes. | Numbered next-action shape and remote issue edit gates. | Contract change, noisy review judgment, or priority requires user choice. |
| PR/MR | Run strict readiness; draft/create PR/MR with target branch, labels, assignee, and Korean body verified. | PR/MR base/target, label, assignee, Korean body, and numbered next-action guards. | Merge approval, target change, reviewer disagreement, or non-green CI waiver. |
| Cleanup | Verify merge/worktree/branch/child state; present cleanup choices before deletion. | Stop hook choice-format relay only. | Worktree/local branch/remote branch deletion, force deletion, parent issue closure. |

When a readiness command reports a missing gate, the automatic loop runs the command that owns that state and retries readiness. The main agent, not the hook, decides whether continuing is safe, reversible, and aligned with the latest user instruction.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: before remote issue creation, run the issue-preflight gate in `references/issue-preflight.md`; record the raw user request, interpreted intent, success criteria, constraints, non-goals, ambiguity ledger, and `--intent-class` with `agent-harness issueops intent record`; then create or prepare a GitHub/GitLab issue with problem, acceptance criteria, non-goals, verification, and open decisions. Before entering the `plan` phase, satisfy the plan-prep evidence gate with `agent-harness issueops plan-prep record`: prior-decision lookup, related-issue scoring, and web research each take evidence or a waive reason. The gate is enforced for non-trivial intent classes (trivial skips it) and blocks `plan`-phase entry, not design review.
4. Large issue breakdown gate: Issue Contract 이후, Plan 이전에 `references/remote-issue.md`의 provider-specific hierarchy rules를 적용한다. Before entering the IssueOps `plan` phase, decide whether the parent issue is too large for one safe work item. If splitting is needed, keep the parent as the umbrella issue, create provider-native child work items with `agent-harness issueops remote create-child`, update the parent body with the child task section, verify hierarchy/labels/assignee/body, and let `create-child` record every verified child in IssueOps state. Use `agent-harness issueops link-child` only as the manual escape hatch for provider-native child work items that already exist and were verified separately. If no split is needed, record the no-split rationale before planning.
5. Plan: produce an issue-based implementation plan under the target repo's planning convention, then record the reviewed design, refactor boundary, risks, alternatives, and verification matrix with `agent-harness issueops design review`.
6. Worktree tool preparation: after linking the plan and before implementation, run `agent-harness issueops worktree prepare-tools --id "$ISSUEOPS_ID" --json` or MCP `issueops_prepare_worktree_tools`. This step persists dependency readiness, supported install action, and CodeGraph readiness on the IssueOps record.
7. Execution decision gate: before entering implementation, record `agent-harness issueops execution decide` or MCP `issueops_record_execution_decision`. Capture auto-proceed boundaries, hook-blocked workflow work, human-in-the-loop gates, and sub-agent usage. Default is `--subagent-use none` with a rationale. `--subagent-use planned` requires each plan to use a `.agent-harness/SUB_AGENT_PATTERNS.md` slug, expected benefit, known tradeoffs, net-positive rationale, scope, verification, and fallback. Use `.agent-harness/research/subagent-tradeoffs.md` for the tradeoff basis.
8. Implementation: the main agent performs TDD directly. Sub-agents are spawned only when the execution decision records a net-positive plan matching the 12 documented patterns. Optimize algorithmic complexity with **`dijkstra`**, design database schemas and indexes with **`codd`**, diagnose failures with **`hopper`**, manage git operations with **`torvalds`** and **`atomic-commit-push`**, and optimize agent prompts with **`karpathy`**. Do not enter implementation until the IssueOps design review is approved and has no open questions, worktree tool preparation evidence is ready, and `execution_decision` is recorded.
9. AI slop clean: before PR/MR drafting, load `references/ai-slop-clean.md` and remove lazy agent artifacts such as vague explanations, unverified claims, overbroad abstractions, dead scaffolding, generic comments, noisy generated prose, and brittle shortcuts; keep only evidence-backed, repo-style code/docs/tests.
10. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
11. PR/MR: draft only after the issue URL, provider-linked branch, plan path, and isolated worktree are linked, AI slop cleanup is complete in that worktree, strict PR readiness is green, and relevant verification has run.

### Large Issue Breakdown Gate

Run this gate after the remote Issue Contract exists and before entering the IssueOps `plan` phase.

```text
Before entering the IssueOps `plan` phase, evaluate whether the current remote issue is too large to implement as one safe work item.

### When To Split

Split the issue into provider-native child tasks when two or more of these are true:

- The issue has multiple independently verifiable acceptance criteria.
- The work touches multiple modules, layers, providers, or runtime concerns.
- The implementation naturally has ordered phases such as routing/config, request shape, lifecycle, usage/cost, migration, or verification.
- A single MR would hide risky behavior changes behind unrelated setup work.
- The issue contains research findings, open assumptions, or external API semantics that need separate implementation validation.
- The estimated work is larger than one focused implementation pass.
- The parent issue reads like an umbrella/epic rather than a directly executable task.

Do not split only because the issue body is long. Split only when the resulting child tasks have independent deliverables, verification, and rollback boundaries.

### Required Behavior

If splitting is needed:

1. Keep the original issue as the umbrella parent.
2. Create provider-native child work items/tasks with `agent-harness issueops remote create-child --id ID --title TEXT --body TEXT --label LABEL --assignee USER --confirm --json`, not ordinary sibling issues.
   - GitHub: create sub-issues if supported by the project workflow.
   - GitLab: create child `Task` work items under the parent issue/work item.
3. Each child task must have:
   - a Korean title
   - a Korean body
   - clear scope
   - acceptance criteria
   - verification commands or evidence
   - non-goals when needed
   - inherited labels from the parent unless explicitly inappropriate
   - assignee matching the parent/current owner
4. Link every child task to the parent using the provider-native hierarchy. The preferred command is `remote create-child`; it creates the child, attaches the hierarchy, verifies labels/assignees, and records the child link. `link-child` is only for an already-created provider-native child URL.
5. Update the parent issue body, not a comment, with:
   - `## 하위 Task`
   - each child task link
   - recommended execution order
   - scope summary per child
   - note that the parent is now the umbrella coordination issue
6. Do not leave the child-task plan only in comments. Comments may be used only for temporary coordination if the provider body update fails.
7. Verify after creation:
   - child items are the correct work item type
   - child-parent relationship exists
   - labels are present
   - assignee is present
   - parent body contains the child task section
8. If incorrect sibling issues were accidentally created, do not silently reuse them.
   - Create the correct child tasks.
   - Close the incorrect issues with a short correction note.
   - Reflect only the correct child tasks in the parent body.

### If Not Splitting

If the issue is small enough, record why it remains a single task before entering `plan`.

Use this format:

Large Issue Breakdown Gate: no split

근거:
- <why the issue is directly executable>
- <why acceptance criteria do not need independent child tasks>
- <expected implementation boundary>

### Output Format

After the gate, report exactly one of these:

분리 결정: split

Parent:
- <parent issue URL>

Child tasks:
1. <child task URL> - <scope>
2. <child task URL> - <scope>
3. <child task URL> - <scope>

검증:
- hierarchy verified
- labels verified
- assignee verified
- parent body updated

or

분리 결정: no split

근거:
- <reason>
- <reason>

다음 단계:
- proceed to IssueOps plan phase for <issue URL>
```

## Agent-Harness Phase Assist Map

IssueOps phases are supported by 9 agent-harness native skills covering strategy, research, design, execution, debugging, optimization, git operations, quality measurement, and cleanup. Each skill works standalone or integrated; when an IssueOps cycle exists, state is persisted through `agent-harness` CLI/MCP. Skills form a pipeline from problem discovery through PR/MR completion:

```
problem → grill → issue → plan → implement → ai-slop-clean → feedback → pr → cleanup
   │        │       │       │         │            │            │        │       │
   ▼        ▼       ▼       ▼         ▼            ▼            ▼        ▼       ▼
  von-    berners  von-     von-     turing      shannon      hopper   turing  torvalds
 neumann  -lee   neumann  neumann   dijkstra     (measure)    (diagnose)  (gate)  (cleanup)
                    +codd    +codd    hopper     turing        turing   torvalds
                   +karpathy (schema)  torvalds   (cleanup)    (steering)  +karpathy
                   (prompt)          (commit)                           (adversarial)
```

| IssueOps phase | Agent-harness assist |
| --- | --- |
| **problem** | Use **`von-neumann`** when the request spans multiple modules, has unclear scope, or needs a decision-complete plan. Von Neumann follows "Explore Before Asking" — it grounds itself in the actual codebase before interviewing the user. Classify the intent (Trivial/Standard/Refactoring/Architecture/Research) to determine interview depth. |
| **grill** | Use **`von-neumann`** Phase 1 (Ground) for codebase exploration, pattern discovery, and brownfield detection. Use CodeGraph for structural call paths and impact analysis. Use **`berners-lee`** for external research: competitive analysis, library documentation comparisons, API reference discovery. Berners-Lee's Hyperlink Contract ensures every domain claim is cross-referenced against independent sources before the issue contract. Research reports are saved to `.agent-harness/research/<slug>.md`. |
| **issue** | Run the issue-preflight deep-interview gate: use **`von-neumann`** Phase 2 (Interview + Clearance Checklist) to reduce ambiguity, rewrite the raw user request into an ideal issue prompt using repo-root `PROMPT.md`, and carry an ambiguity ledger with resolved/deferred/blocking entries. For database-heavy work, invoke **`codd`** Step 1 (SURVEY) to capture DDL, row counts, and access patterns — schema constraints become issue constraints. Keep remote writes in the IssueOps remote artifact gates. |
| **plan** | Use **`von-neumann`** Phase 3 (Plan Generation) to produce a decision-complete plan at `.agent-harness/plans/<slug>.md`. Link it with `agent-harness issueops link-plan`. Von Neumann plans include a dependency matrix, parallel execution waves, and per-task QA scenarios. Use **`karpathy`** Phase 1-2 (SPECIFY + DRAFT) to optimize von-neumann's plan-generation prompt — calibrate model-specific constraints, structure primacy/recency zones, and add adversarial hardening so the generated plan is precise, testable, and free of ambiguity. For database schema changes, invoke **`codd`** Step 2 (NORMALIZE) to audit tables against 1NF→BCNF; normalization violations become plan tasks. For algorithmic work, invoke **`dijkstra`** Step 1-2 (ANALYZE + CLASSIFY) to identify the problem class and optimal algorithm — complexity targets become plan acceptance criteria. No implementation until the clearance checklist passes and the design review is approved. |
| **implement** | Use **`turing`** for evidence-bound execution: the main agent performs RED→GREEN→SURFACE→CLEAN TDD directly, drives Manual-QA across 4 channels (HTTP/tmux/browser/computer-use), and tracks quantitative metrics. Sub-agents only per the 12 net-positive patterns (`.agent-harness/SUB_AGENT_PATTERNS.md`). **`dijkstra`** optimizes algorithmic complexity (O(n²)→O(n log n)→O(n)) with benchmark evidence; every optimization must prove complexity class change via scaling tests. **`codd`** Step 3-4 (SCALE + INDEX) designs tables by expected row count and selects indexes with explicit write-penalty justification. **`hopper`** diagnoses test/debug failures via 7-step Hopper Method (REPRODUCE→TRANSLATE→ISOLATE→HYPOTHESIZE→VERIFY→FIX→LEARN). **`torvalds`** handles git operations: worktree creation, atomic commits per Conventional Commit + Lore format, and rebase/cherry-pick as needed. **`atomic-commit-push`** manages staged commits and push safety. |
| **ai-slop-clean** | Use **`shannon`** Phase 0-1 (BASELINE + REGRESSION CHECK) to measure signal-to-noise ratio (SNR), entropy, and redundancy BEFORE cleanup. Use **`turing`** Final Quality Gate step 2 to remove lazy agent artifacts: obvious comments, dead scaffolding, over-defensive code, needless abstraction, duplication, oversized modules. Use **`karpathy`** Phase 3-4 (TEST + DIAGNOSE) to adversarial-test skill prompts used during implementation — detect prompt drift, injection vulnerabilities, and format degradation introduced during coding. Use **`dijkstra`** Step 5 (SIMPLIFY) for structural complexity reduction — replace deep nesting with guard clauses, eliminate modern GOTO patterns. Use **`shannon`** Phase 3 (GATE) to re-measure after cleanup and confirm SNR improved. Record before/after metrics as IssueOps evidence. |
| **feedback** | Use **`turing`** Dynamic Steering to record feedback as structured evidence. For contract-changing feedback, update the remote issue body before continuing. For review feedback, answer in the original thread with verdict, evidence, and next action. Use **`hopper`** to diagnose reported bugs — reproduce the failure exactly, isolate the root cause, and deliver a verified diagnosis. Use **`berners-lee`** to research external root causes (upstream library bugs, known issues, changelog regressions). |
| **pr** | Use **`turing`** Final Quality Gate: spawn an adversarial reviewer sub-agent (pattern #2: Devil's advocate) with the full diff, all success criteria, shannon metrics, and all evidence. Use **`karpathy`** Phase 5 (REFINE) to harden the reviewer sub-agent prompt — add immutability clauses, injection barriers, and output format constraints so the reviewer cannot be redirected by the diff content and always produces a structured verdict. Run targeted verification, AI slop clean, re-verify, and reviewer check. The reviewer verdict is BINDING — unconditional approval only. Use **`torvalds`** for rebase/squash before PR submission, ensuring commit history is clean and atomic. Keep Korean remote artifact, label, assignee, and strict readiness checks in IssueOps. |
| **cleanup** | Use **`turing`** cleanup receipt rules: every QA resource (PIDs, tmux sessions, browser contexts, ports, temp files) must be torn down with a recorded receipt. Use **`torvalds`** for post-merge branch cleanup, worktree removal, and remote branch pruning per safety protocols (verify merged status before delete). Keep merge evidence and worktree/branch cleanup decisions in `references/cleanup-state.md`. |

### Skill-by-Phase Reference

| Skill | Phases involved | Role in IssueOps |
|-------|----------------|------------------|
| **von-neumann** | problem, grill, issue, plan | Strategic planning: intent classification, exploration, interview, decision-complete plan generation |
| **berners-lee** | grill, issue, feedback | External research: parallel web searches, source cross-referencing, competitive analysis, library investigation |
| **codd** | issue, plan, implement | Database design: schema survey, normalization audit, table sizing by row count, indexing, query optimization |
| **dijkstra** | plan, implement, ai-slop-clean | Algorithm optimization: complexity analysis, optimal algorithm selection, O(n²)→O(n log n), structural simplification |
| **hopper** | implement, feedback | Systematic debugging: reproduce, isolate, hypothesize, verify, fix, learn — 7-step method |
| **turing** | implement, ai-slop-clean, feedback, pr, cleanup | Evidence-bound execution engine: RED→GREEN→SURFACE→CLEAN TDD, 4-channel QA, reviewer gate, metrics tracking |
| **shannon** | ai-slop-clean | Quantitative quality measurement: SNR, entropy, redundancy — before/after metrics for ai-slop-clean gate |
| **torvalds** | implement, pr, cleanup | Git operations: worktree, atomic commits, rebase, squash, post-merge cleanup, reflog recovery |
| **karpathy** | plan, ai-slop-clean, pr | Prompt engineering: optimize plan-generation prompts, adversarial-test skill prompts, harden reviewer sub-agent prompt |
| **atomic-commit-push** | implement, pr | Staged commits and push safety: preflight, scope, Conventional Commit + Lore format |

## Reference Map

Load these files only when the phase applies:

- `references/remote-issue.md`: remote issue first, related issue/label scoring, external LLM judge contract, Korean remote artifact gate, issue template.
- `references/issue-preflight.md`: deep-interview ambiguity reduction and `PROMPT.md`-based ideal issue prompt rewrite before remote issue creation.
- `references/evidence-contract.md`: portable domain contract, API documentation, live evidence, review accountability, and completion hygiene rules.
- `references/worktree-context.md`: branch/worktree contract, local config symlink rules, context routing.
- `references/ai-slop-clean.md`: PR/MR-prep cleanup prompt for removing lazy agent residue while preserving behavior. Run **`shannon`** SNR measurement before and after.
- `references/review-feedback.md`: worker prompt requirements, bounded subagent review rules, remote review feedback replies and thread resolution.
- `references/cleanup-state.md`: post-merge cleanup, state commands, benchmark commands, stop conditions.

### Cross-Skill References

Load from other skill directories when the phase involves specialized work:

| Phase | Load from | For |
|-------|-----------|-----|
| grill, issue | `skills/berners-lee/references/report-template.md` | Authority classification, confidence levels for external research |
| plan, implement | `skills/codd/SKILL.md` (Steps 2-4) | Normalization audit, index selection matrix, query optimization |
| implement | `skills/dijkstra/SKILL.md` (Steps 1-5) | Problem classification table, optimization patterns, complexity cheatsheet |
| implement | `skills/hopper/SKILL.md` (Steps 1-7) | Debugging patterns reference, isolation strategies |
| plan, ai-slop-clean, pr | `skills/karpathy/SKILL.md` (Phases 1-5) | Prompt optimization, adversarial testing, model calibration, immutability clauses |
| implement, pr, cleanup | `skills/torvalds/references/rebase-protocol.md` | Pre-rebase checklist, conflict resolution |
| implement, pr, cleanup | `skills/torvalds/references/bisect-protocol.md` | Bisect workflow, when NOT to use bisect |
| ai-slop-clean | `.agent-harness/SUB_AGENT_PATTERNS.md` | 12 net-positive sub-agent patterns, net-negative patterns |
| all phases | `.agent-harness/CONSTITUTION.md` (Sub-Agent 사용 원칙) | Sub-agent usage principles |

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
- Template before remote writes: render or validate remote issue, child task, PR, and MR bodies through `issueops remote render-template` or `create-* --template/--body-file`; confirmed writes fail closed on critical template validation, missing label, missing assignee, Korean artifact failure, and target/base branch mismatch.
- Korean remote hook guard: installed PreToolUse hooks include `--enforce-korean-remote-artifacts` and block `gh issue/pr create/edit` when an inspectable title/body fails the Korean remote artifact gate.
- VCS linking hook guard: installed PreToolUse hooks include `--enforce-vcs-issue-linking` and block `gh`/`glab` issue create/edit when the body carries a `Plan Link` section or, on GitLab, a `Related Issues` body section (related issues belong in native linked items). The same guard blocks remote issue/PR/MR create commands without labels or assignee, and blocks PR/MR create when an active IssueOps cycle has `branch_prepare.base_branch` but the inspected target/base branch is missing or different. Copy linked issue labels for PR/MR create or pass an explicit manual label, assign the artifact to the current user, and target the recorded parent work branch. See `references/remote-issue.md` -> "Provider-Specific Linking And Hierarchy".
- No broad review sweeps: subagent reviews must have explicit included paths, excluded large/generated paths, a time budget, and a fallback direct verification path.
- Cleanup choices: after a PR/MR is merged, verify merge/worktree/branch status and present numbered cleanup choices before deleting local worktrees or branches.
- Numbered next actions: at user decision points and after reporting review/feedback/cleanup status, end with `선택지:` and three numbered choices. Installed Stop hooks with `--enforce-numbered-next-actions` block missing choices and tell the agent to explain the block before presenting context-specific choices.
- Next-action Stop hook: to reduce friction, the Stop hook may re-enter the main agent when the final response contains an explicit `선택지:` section. The hook is not a judge, scorer, classifier, or safety gate; it relays observed facts such as choice count, recommendation count, and recommended text. The main agent must judge safety, reversibility, user-intent alignment, and whether to proceed or ask the user from the current context, then state why it is auto-proceeding or why it is not auto-proceeding and needs user confirmation. Auto-proceed result reports still end with `선택지:` so the next action boundary remains explicit. Mark exactly one recommended option only when the main agent itself judges it safe, reversible, and aligned.
- Worker identity check: every implementation, TDD, review, QA, or subagent worker must first report and verify `pwd`, branch, `HEAD`, and the expected isolated worktree path before inspecting or changing anything.
- Remote artifact ownership: created issues and PRs/MRs must be assigned to the currently authenticated user when the provider supports assignment, and assignment must be verified before reporting readiness.
- Remote issue source of truth: when feedback changes scope, acceptance criteria, non-goals, verification, labels, related links, or implementation contract, update the remote issue body before continuing.
- Review thread accountability: remote review feedback must be answered in the original review thread/discussion with verdict, evidence, and next action; do not report feedback cleared until addressed threads are replied to, resolved when appropriate, and re-checked.
- AI slop clean before PR/MR: after implementation and before PR/MR drafting, inspect the actual worktree diff for lazy agent artifacts, unsupported claims, generic prose, dead scaffolding, unnecessary abstractions, weak comments, and brittle shortcuts. Remove them or record why they are intentional before moving to `pr`. Run **`shannon`** SNR measurement before and after cleanup; record before/after metrics as evidence.
- Shannon gate before ai-slop-clean: measure baseline SNR, entropy, and redundancy before cleanup begins. Re-measure after cleanup; SNR must improve or the cleanup pass is incomplete. Record metrics in IssueOps evidence.
- Dijkstra complexity gate: when implementation touches algorithmic code, profile before optimizing. Every optimization must include before/after benchmark evidence and complexity class confirmation via scaling tests (N=100→1000→10000). Record in commit messages and IssueOps evidence.
- Codd schema gate: when implementation includes DDL changes, the schema must pass normalization audit (1NF→BCNF) or every denormalization must be explicitly justified with read:write ratio trade-off. Every new index must state its write penalty.
- Hopper diagnosis protocol: when a test or bug report arrives, reproduce the failure before diagnosing. Apply the 7-step Hopper Method. Cap hypothesis cycles at 5. Record root cause diagnoses as IssueOps feedback.
- Torvalds commit protocol: every commit must be atomic (one intent per commit). Use Conventional Commit + Lore body format per `.agent-harness/COMMIT_POLICY.md`. Never force-push shared branches. Always create a backup branch before history rewrite.
- Berners-Lee research protocol: during grill and issue phases, external claims must cite sources with retrieval dates. Claims without ≥2 independent sources are flagged as single-sourced. Research reports are committed to `.agent-harness/research/`.
- Completion hygiene: before reporting done, verify the final diff, target branch, remote issue/PR/MR prose freshness, single-commit or declared commit policy, and cleanup/worktree status.
- External LLM wrapper: all IssueOps LLM judging must go through the shared harness external LLM wrapper, defaulting to Z.AI `glm-5-turbo`, and remain read-only judgment.

## Gate Quick Reference

When an IssueOps command reports a missing gate, do not guess a new hidden flag. Use the command that owns that state:

- `intent_contract`: run `issueops intent record` with raw request, interpreted intent, success criteria, constraints/non-goals/ambiguity when known. Pass `--intent-class trivial|standard|refactoring|architecture|research`; an empty class normalizes to `standard` and trivial skips the plan-prep gate.
- `plan_prep_decisions` / `plan_prep_related_issues` / `plan_prep_web_research`: run `issueops plan-prep record` with evidence or a waive reason per item before entering the `plan` phase. Enforced only for non-trivial intent classes; design review does not require it because design review runs inside the plan phase where plan-prep is already satisfied.
- `branch_prepare` / `branch_link_verified`: run `issueops branch prepare` only after provider-visible branch evidence exists. The branch must start with the issue/task number and a hyphen.
- `worktree_path` / `worktree_exists`: create the sibling isolated worktree first, then run `issueops link-worktree`.
- `worktree_tools_prepared` / `worktree_dependencies_ready` / `codegraph_ready`: run `issueops worktree prepare-tools` or MCP `issueops_prepare_worktree_tools` after linking the plan. If the package manager requires manual dependency reuse, symlink, copy, or install, do that in the linked worktree and rerun prepare-tools until the durable evidence is ready.
- `execution_decision`: run `issueops execution decide` or MCP `issueops_record_execution_decision`. Record at least one auto-proceed condition, hook-blocked workflow action, and human gate. For `subagent_use=none`, include a rationale. For `subagent_use=planned`, every plan must include objective, documented pattern slug, expected benefit, tradeoffs, net-positive rationale, scope, verification, and fallback.
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
관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 Z.AI `glm-5-turbo` LLM judge, 실패 시 deterministic fallback으로 진행합니다.
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

### Benchmark Judge Protocol (subagent-first)

When an issueops benchmark needs an LLM judge, use the Z.AI `glm-5-turbo` backend or a fresh-context sub-agent for independent review:

1. Run the deterministic pass first: `agent-harness issueops benchmark run --fixtures <dir> --judge none --json`.
2. The main agent dispatches a **fresh-context** sub-agent (no inherited conversation context, never the author of the artifacts being judged — no self-scoring) with a deterministic input packet: ① the rubric dimension list with one-line definitions, ② the artifact fields to judge, ③ the required output: a `{"<fixtureID>": <IssueOpsBenchmarkScore>}` map as JSON only, no preamble.
3. Feed the returned map through `agent-harness issueops benchmark run --fixtures <dir> --judge file --judge-file <map.json> --json`. The CLI strict-decodes each score and fails closed on missing/unknown fixture keys.

`--judge llm` uses the shared Z.AI external-LLM wrapper. Honesty note: the no-self-approval constraint is a documented orchestration protocol — the Go `--judge file` layer only sees bytes and cannot verify who produced the judgment; enforcement lives in the main agent's dispatch discipline.

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
  --intent-class "$INTENT_CLASS" \
  --json
```

Record the plan-prep evidence gate before entering the `plan` phase (non-trivial intent classes). Each item takes evidence or a mutually-exclusive waive reason:

```bash
agent-harness issueops plan-prep record --id "$ISSUEOPS_ID" \
  --decisions-evidence "$PRIOR_DECISION_LINK_OR_ADR" \
  --related-score-ref "$REMOTE_SCORE_SUMMARY" \
  --web-research-evidence "$RESEARCH_FILE_OR_SOURCE" \
  --json
# or waive an item that is genuinely unnecessary:
#   --decisions-waive "no prior decisions touch this area"
#   --related-waive "no comparable issues exist"
#   --web-research-waive "purely internal refactor, no external semantics"
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

`link-worktree` fails closed until issue-linked branch evidence exists and the worktree path already exists on disk. `link-plan` attaches the reviewed implementation plan but does not enter implementation by itself. It fails closed until the issue is linked, `branch prepare --link-verified` has recorded provider-visible branch evidence, the worktree is linked, the design review is approved with no open questions, and the plan path exists inside that linked worktree. Run `worktree prepare-tools` after `link-plan`, then record the execution decision before `implement`:

```bash
agent-harness issueops execution decide --id "$ISSUEOPS_ID" \
  --auto "implementation may proceed after linked worktree readiness is durable" \
  --hook-block "hooks do not create issues, prepare worktrees, run tests, or decide sub-agent usage" \
  --human-gate "ask before destructive cleanup, live access, or unclear product behavior" \
  --subagent-use none \
  --subagent-rationale "main agent owns this focused implementation" \
  --json
```

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

If linked child tasks exist, close children separately after the child PR/MR is verified merged into the parent work branch. This is child-only cleanup: close the GitHub sub-issue or GitLab child Task, record close verification evidence, and keep the parent issue open as the umbrella until the full umbrella reaches the mainstream merge target.

```bash
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --json
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --confirm --json
```

Record feedback, optionally classifying each item (contract_change, defect, question, noise) so contract-changing feedback is distinguishable:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source review --body "$FEEDBACK" --classification contract_change --json
agent-harness issueops feedback mark-issue-updated --id "$ISSUEOPS_ID" --json
```

Remote scoring runs deterministically and is also available as the MCP tool `issueops_remote_score` for cross-host (Codex/Claude) use; the LLM judge path stays CLI/`remote score --judge llm --model glm-5-turbo`. Benchmark commands (`benchmark run|compare|gate`) are CLI-only developer/autoresearch tooling, not a runtime MCP gate.

## Stop Conditions

Stop and ask before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Stop before implementation if `issueops intent record` or `issueops design review` cannot be completed from evidence. Do not treat a recommended next-action option as permission to continue unless the main agent records why continuation is safe, reversible, and aligned with the user's latest instruction.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `issue_url`, `branch_prepare`, `branch_link_verified`, `plan_path`, `worktree_path`, `worktree_exists`, `branch_match`, `worktree_clean`, `upstream`, `upstream_synced`, `plan_exists`, or `ai_slop_clean`.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `contract_feedback_issue_update`. This means a `contract_change` feedback item was recorded after the remote issue contract changed, and the remote issue body update has not been confirmed with `issueops feedback mark-issue-updated`.

Do not mark an IssueOps loop `done` before it has entered the `pr` phase. Completion reporting happens after PR/MR readiness and review/merge hygiene, not as an escape hatch from planning or implementation.

Before PR/MR create, verify the linked issue labels and pass them to the provider create command. If the linked issue has no labels, create or apply an explicit manual label first, or stop and record label-decision feedback; never create the PR/MR with an empty label set.
