---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, AI slop cleanup, feedback loops, and PR/MR drafting.
---

# IssueOps

Use this skill when the user wants a repeatable cycle from a vague problem to a GitHub/GitLab issue, implementation plan, tested change, AI slop cleanup, feedback loop, and PR/MR.

This file is the phase router. Load only the referenced phase document needed for the current step.

## Core Contract

The workflow is advisory and agent-driven. Hooks may suggest this skill, but hooks must not create issues, edit files, run tests, wait on background jobs, or open PRs/MRs by themselves.

The cycle has one durable state record with one `Execution`. Use `agent-harness issueops ... --json` or the single MCP tool `issueops_execution` when execution state must survive compaction or another host. Execution v1 owns one canonical worktree and one generation-fenced native holder. Its modes are `direct` and `orca`; Orca is a workspace/owner adapter, never a second workflow authority.

The exact lifecycle ID selects one isolated cycle even when several active cycles share the source checkout. Run `agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto` as a preview, then execute its `next_command`, which carries the readiness fingerprint into an otherwise identical confirm. Read back `execution.selection` (`requested_mode`, `resolved_mode`, probe facts/codes, fallback, fingerprint, selection time, and explicit-direct reason). `--mode direct` is exceptional and requires `--direct-reason`; automated children must not use it to bypass Orca readiness. A direct holder continues in the canonical worktree. An Orca owner verifies the sealed issue/context digests and claims the exact generation from its private token file. The active holder implements, verifies, publishes, and calls `issueops execution complete`; merge and destructive cleanup remain separate human-authorized operations.

IssueOps is a main-agent state machine:

```text
problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup
```

`agent-harness issueops ...` CLI/MCP commands own durable state, phase transitions, readiness, and remote artifact records. Lifecycle hooks are limited to fast, deterministic, inspectable guards and routing hints. A hook may block a clearly invalid tool event, but it must not perform workflow work: no issue creation, provider mutation, file edit, test run, background wait, branch/worktree preparation, PR/MR creation, review reply, merge, or cleanup.

### IssueOps Benchmark Artifact Contract

When IssueOps contributes to a benchmark response, include a compact labeled evidence block. The block must describe durable workflow state, not just intentions.

```text
Durable state record: <IssueOps id, phase, readiness gates, state path/tool output>
Phase routing: <problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup decisions>
Flow evidence: <issue, plan, TDD, subagent decision, feedback, PR/MR artifacts>
Hook boundary: <what hooks may suggest/block and what only the main agent/CLI owns>
Cleanup/readiness evidence: <strict readiness, merge/cleanup status, remaining choices>
```

If no IssueOps cycle exists, do not fabricate one. Record that the workflow is not active and route to the appropriate standalone skill.

### Flow Boundary Matrix

| Phase area | Automatic main-agent loop | Hook enforcement | Human-in-the-loop |
| --- | --- | --- | --- |
| Problem / Grill | Gather repo/docs/code/runtime evidence; draft intent class; ask only on blocking ambiguity. | UserPromptSubmit routing hint only. | Success criteria, scope, or terminology changes the implementation. |
| Issue Contract | Record intent; run issue-preflight; score related issues and labels; pass Korean artifact gate; create/link the remote issue when credentials, target, owner, and base branch are clear. | Korean remote artifact, VCS linking metadata, missing label, and missing assignee guards. | Credentials, target project, owner, base branch, or selected label is unclear. |
| Large Issue Breakdown | Decide split/no-split; create provider-native child tasks; verify hierarchy, labels, assignee, and parent body. | VCS artifact guard blocks body-only hierarchy when native linking is required. | Child scope is a product decision or provider hierarchy support/permission is missing. |
| Plan / Design Review | Record plan-prep evidence or waivers and approve the design with risks, alternatives, and verification; write and link the plan inside the resulting worktree. | No hook enforcement. | Open question, risky alternative, or refactor boundary remains unresolved. |
| Branch / Worktree Prep | Create the provider-linked branch with exact base SHA, then preview/confirm `issueops execution prepare --mode auto`; write and link the plan in the returned canonical worktree and record compatibility review before `implement`. | Worktree guard blocks source-checkout mutation and wrong canonical-worktree targets; an Orca execution blocks mutation until the native owner claims. | Base branch is user-selected/unclear/conflicting, dependency/config linking has secret risk, or external mutation recovery is ambiguous. |
| Compatibility Review | Review backward compatibility, side effects, rollback plan, verification evidence, and blockers; record approval with `issueops compatibility review`. | Hook may only block missing durable state/readiness; it does not judge compatibility or side effects. | Any unresolved blocker, public contract risk, migration risk, or rollback uncertainty remains. |
| Implementation | The active native holder runs TDD, focused fixes, and verification. Sub-agents are used only for a documented net-positive pattern under the repository contract. | Worktree guard; staged-check and live-command guards where installed. Hooks do not decide sub-agent usage. | Failure classification, destructive migration, live access, product behavior, or sub-agent tradeoff judgment is unclear. |
| AI Slop Clean | Inspect the actual diff, remove lazy artifacts, rerun targeted checks, record cleanup evidence. | Worktree guard remains active. | Cleanup would widen scope or require behavior changes. |
| Feedback | Classify CI/review/user feedback; fix valid items; update remote issue body for contract changes. | Numbered next-action shape and remote issue edit gates. | Contract change, noisy review judgment, or priority requires user choice. |
| PR/MR | Run strict readiness; draft/create PR/MR with target branch, labels, assignee, and Korean body verified. | PR/MR base/target, label, assignee, Korean body, and numbered next-action guards. | Merge approval, target change, reviewer disagreement, or non-green CI waiver. |
| Cleanup | Verify merge/worktree/branch/child state; present cleanup choices before deletion. | Stop hook choice-format relay only. | Worktree/local branch/remote branch deletion, force deletion, parent issue closure. |

When a readiness command reports a missing gate, the automatic loop runs the command that owns that state and retries readiness. The main agent, not the hook, decides whether continuing is safe, reversible, and aligned with the latest user instruction.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: before remote issue creation, run the issue-preflight gate in `references/issue-preflight.md`; record the raw user request, interpreted intent, success criteria, constraints, non-goals, ambiguity ledger, and `--intent-class` with `agent-harness issueops intent record`; then create or prepare a GitHub/GitLab issue with problem, acceptance criteria, non-goals, verification, and open decisions. Before entering the `plan` phase, satisfy the plan-prep evidence gate with `agent-harness issueops plan-prep record`: prior-decision lookup, related-issue scoring, web research, and the codebase survey each take evidence or a waive reason. The codebase survey is the anti-"requirements-only" gate: never draft the issue contract or plan from the request text alone — sweep the whole codebase with the available tools (rg, CodeGraph, LSP) for every symbol, file, call path, and existing similar implementation the change touches, and record what was searched and found. The gate is enforced for non-trivial intent classes (trivial skips it) and blocks `plan`-phase entry, not design review.
4. Large issue breakdown gate: Issue Contract 이후, Plan 이전에 `references/remote-issue.md`의 provider-specific hierarchy rules를 적용한다. Before entering the IssueOps `plan` phase, decide whether the parent issue is too large for one safe work item. The default decision is no split. Split only when one issue would be unsafe because the work is genuinely large, risky, or hides independent delivery decisions, or when the user/owners explicitly requested for collaboration. If splitting is needed, keep the parent as the umbrella issue, create provider-native child work items with `agent-harness issueops remote create-child`, update the parent body with the child task section, verify hierarchy/labels/assignee/body, and let `create-child` record every verified child in IssueOps state. Use `agent-harness issueops link-child` only as the manual escape hatch for provider-native child work items that already exist and were verified separately. If no split is needed, record the no-split rationale before planning.
5. Design: record the reviewed design, refactor boundary, risks, alternatives, and verification matrix with `agent-harness issueops design review`. An approved design review is required before a supervised worktree may be created.
6. Execution selection, worktree, and plan: after creating the provider-linked branch with exact base SHA and approving the design, preview and confirm `agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto`. Load `references/execution.md` for the exact direct/Orca contract. `auto` chooses Orca only when readiness succeeds before mutation; otherwise it resolves to direct. Any ambiguity after an external mutation fails closed and uses `issueops execution reconcile`, never another create attempt. Produce and link the issue-based implementation plan inside the returned canonical worktree. Set up dependencies there with the repository's documented command.
7. Compatibility review gate: before entering implementation, record `agent-harness issueops compatibility review`. Capture backward compatibility findings, side effects, rollback plan, verification evidence, blockers, and approval. Approved compatibility reviews must have no blockers.
8. Implementation: the active holder performs TDD directly. Sub-agents are spawned only for a net-positive plan matching the 12 documented patterns; record the objective, pattern slug, benefit, tradeoffs, scope, verification, and fallback in the implementation plan or evidence. Optimize algorithmic complexity with **`dijkstra`**, design database schemas and indexes with **`codd`**, diagnose failures with **`hopper`**, manage git operations with **`torvalds`** and **`atomic-commit-push`**, and optimize agent prompts with **`karpathy`**. Do not enter implementation until the design review is approved with no open questions, the canonical worktree and lease are ready, compatibility review is approved with no blockers, and a devil's-advocate verdict is recorded.
9. AI slop clean: before PR/MR drafting, load `references/ai-slop-clean.md` and remove lazy agent artifacts such as vague explanations, unverified claims, overbroad abstractions, dead scaffolding, generic comments, noisy generated prose, and brittle shortcuts; keep only evidence-backed, repo-style code/docs/tests.
10. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
11. PR/MR and completion: the active generation creates the draft PR/MR only after strict readiness and verification. Read back the durable remote artifact, then run `issueops execution complete` with the exact final HEAD, Turing report, artifact URL, evidence, actor, generation, and `--confirm`.

### Delegated Child Cycles

Delegated child cycles let a parent IssueOps cycle coordinate bounded sub-agent work without asking hooks or the harness to spawn agents. The main agent owns dispatch; `agent-harness` owns durable state, gates, and owner commands.

Delegation preconditions:

- The parent is in `implement phase`.
- Required reviews are approved reviews: design review, compatibility review, and devil's-advocate review are pass or explicitly waived according to the normal implement-entry gate.
- The implementation plan or durable evidence names the documented sub-agent pattern and records scope, verification, fallback, tradeoffs, and net-positive rationale.

Owner commands:

- Start a child: `issueops child start --parent "$ISSUEOPS_ID" --branch "$CHILD_BRANCH" --title "$TITLE" --scope "$SCOPE" --acceptance "$CRITERION" --json`
- Inspect children: `issueops child status --parent "$ISSUEOPS_ID" --json`
- Validate a done child: `issueops child accept --parent "$ISSUEOPS_ID" --child "$CHILD_ID" --evidence "$EVIDENCE" --json`
- Send a child back for redo: `issueops child reject --parent "$ISSUEOPS_ID" --child "$CHILD_ID" --reason "$REASON" --json`
- Remove a child from the parent gate: `issueops child drop --parent "$ISSUEOPS_ID" --child "$CHILD_ID" --reason "$REASON" --json`

Verdict table:

| Verdict | Meaning | Gate effect |
|---|---|---|
| `accepted` | The main agent reviewed the child evidence and accepts it as satisfying the delegated contract. | Child no longer blocks parent PR readiness. |
| `rejected` | The child result is not acceptable yet; dispatch a revised prompt or continue the child cycle. | Parent remains blocked as `child_rejected_unresolved` until accept or drop. |
| `dropped` | The main agent intentionally removes the child from the parent contract with an auditable reason. | Child no longer blocks the parent gate, but the reason remains in state. |

Do not mutate child records from the parent to "fix" them. The child owns its own cycle; the parent owns only the child index and validation verdict. Use `references/orchestration.md` for the child contract prompt, scope-drift stop rule, and validation rubric.

### Large Issue Breakdown Gate

Run this gate after the remote Issue Contract exists and before entering the IssueOps `plan` phase.

```text
Before entering the IssueOps `plan` phase, evaluate whether the current remote issue is too large to implement as one safe work item.

### When To Split

The default decision is no split. A directly executable issue stays as one issue even when it has several checklist items.

Split the issue into provider-native child tasks only when at least one primary split trigger is true:

- One issue would be unsafe because the work is genuinely large, risky, or would hide independent delivery decisions, verification, rollback, or review boundaries.
- The user, product owner, or multiple implementers explicitly requested for collaboration, parallel ownership, or separate assignees.

When splitting, design every child to be `[p] parallelizable` by default. Decompose at the `[p]` unit wherever the scope boundary allows an independent start and independent verification; reserve `[s] sequential` only for a child with a genuinely unavoidable cross-child dependency — where one child's code, schema, remote state, migration, fixture, or decision output is a hard input to another and no contract/interface decoupling can remove that ordering. Before marking a child `[s]`, state the specific unavoidable dependency that blocks parallelization (for example, a shared database migration that must land before dependent code compiles). If you cannot name a concrete hard dependency, the child must be `[p]`. Name each child's prerequisites (`none` for `[p]`) and group children into execution waves. `[p]` means the task can start without another child task's output and its verification can run independently. `[s]` means the task must wait for another child's output. The `[p]`/`[s]` prefix is mandatory in each child title and in the parent child-task section. If this classification is unclear, stop for a product/engineering choice instead of guessing.

Supporting signals are not sufficient by themselves. Use them only as evidence for one of the primary split triggers:

- The issue has multiple independently verifiable acceptance criteria.
- The work touches multiple modules, layers, providers, or runtime concerns.
- The implementation naturally has ordered phases such as routing/config, request shape, lifecycle, usage/cost, migration, or verification.
- A single MR would hide risky behavior changes behind unrelated setup work.
- The issue contains research findings, open assumptions, or external API semantics that need separate implementation validation.
- The estimated work is larger than one focused implementation pass.
- The parent issue reads like an umbrella/epic rather than a directly executable task.

Do not split only because the issue body is long, has multiple bullets, touches multiple files, or can be described as phases. Split only when keeping it as one issue would create concrete delivery risk, or when collaboration requires separate child ownership. If the issue is small enough for one focused owner and one reviewable MR, record a no-split rationale.

### Required Behavior

If splitting is needed:

1. Keep the original issue as the umbrella parent.
2. State the split trigger in the parent body: either `one issue would be unsafe` with the concrete risk, or `explicitly requested for collaboration` with the owner/assignee boundary.
3. Create provider-native child work items/tasks with `agent-harness issueops remote create-child --id ID --title TEXT --body TEXT --label LABEL --assignee USER --confirm --json`, not ordinary sibling issues.
   - GitHub: create sub-issues if supported by the project workflow.
   - GitLab: create child `Task` work items under the parent issue/work item through the GraphQL work-item hierarchy path owned by `remote create-child`. Do not fall back to the REST Issues API `issue_type=task` path or ordinary `glab issue create` as the creation/attachment mechanism.
4. Each child task must have:
   - a Korean title
   - a mandatory title prefix: `[p]` for `parallelizable` (default) or `[s]` for `sequential` (only with a named unavoidable dependency)
   - a Korean body
   - clear scope
   - execution class: `[p] parallelizable` (default) or `[s] sequential` (only when a hard cross-child dependency is stated)
   - prerequisites/dependencies, or `none`
   - for `[s]` only: the specific unavoidable dependency that prevents parallelization (omit for `[p]`)
   - execution wave/order
   - acceptance criteria
   - verification commands or evidence
   - non-goals when needed
   - inherited labels from the parent unless explicitly inappropriate
   - assignee matching the parent/current owner
5. Link every child task to the parent using the provider-native hierarchy. The preferred command is `remote create-child`; it creates the child, attaches the hierarchy, verifies labels/assignees, and records the child link. `link-child` is only for an already-created provider-native child URL.
6. Update the parent issue body, not a comment, with:
   - `## 하위 Task`
   - each child task link
   - recommended execution order
   - `[p]`/`[s]` prefix for every child link
   - execution waves grouping parallelizable (`[p]`) children first, then sequential (`[s]`) children if any
   - prerequisites/dependencies for every child, and the unavoidable dependency that forces each `[s]` child
   - scope summary per child
   - note that the parent is now the umbrella coordination issue
7. Do not leave the child-task plan only in comments. Comments may be used only for temporary coordination if the provider body update fails.
8. Verify after creation:
   - child items are the correct work item type
   - child-parent relationship exists
   - labels are present
   - assignee is present
   - parent body contains the child task section
9. If incorrect sibling issues were accidentally created, do not silently reuse them.
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
- <why no collaboration split was requested or needed>

### Output Format

After the gate, report exactly one of these:

분리 결정: split

Parent:
- <parent issue URL>

Default decomposition is all-`[p]` parallelizable children; include `[s]` lines only when a child has a stated unavoidable dependency. Omit every `[s]` example below when the split is fully parallelizable.
Child tasks:
1. [p] <child task URL> - <scope> - class: parallelizable - prerequisites: none - wave: <N>
2. [s] <child task URL> - <scope> - class: sequential - prerequisites: <child URLs> - wave: <N>
3. [p|s] <child task URL> - <scope> - class: parallelizable|sequential - prerequisites: <none or child URLs> - wave: <N>

검증:
- hierarchy verified
- labels verified
- assignee verified
- parent body updated
- execution waves and prerequisites documented

or

분리 결정: no split

근거:
- <reason>
- <reason>

다음 단계:
- proceed to IssueOps plan phase for <issue URL>
```

## Agent-Harness Phase Assist Map

IssueOps phases are supported by 11 agent-harness native skills covering strategy, research, design, execution, debugging, optimization, git operations, quality measurement, and cleanup. Each skill works standalone or integrated; when an IssueOps cycle exists, state is persisted through `agent-harness` CLI/MCP. Skills form a pipeline from problem discovery through PR/MR completion:

```
problem → grill → issue → plan → compatibility-review → implement → ai-slop-clean → feedback → pr → cleanup
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
| **grill** | Use **`von-neumann`** Phase 1 (Ground) for codebase exploration, pattern discovery, and brownfield detection. Use **`berners-lee`** for external research: competitive analysis, library documentation comparisons, API reference discovery. Berners-Lee's Hyperlink Contract ensures every domain claim is cross-referenced against independent sources before the issue contract. Research reports are saved to `.agent-harness/research/<slug>.md`. |
| **issue** | Run the issue-preflight deep-interview gate: use **`von-neumann`** Phase 2 (Interview + Clearance Checklist) to reduce ambiguity, rewrite the raw user request into an ideal issue prompt using repo-root `PROMPT.md`, and carry an ambiguity ledger with resolved/deferred/blocking entries. For database-heavy work, invoke **`codd`** Step 1 (SURVEY) to capture DDL, row counts, and access patterns — schema constraints become issue constraints. Keep remote writes in the IssueOps remote artifact gates. |
| **plan** | Use **`von-neumann`** Phase 3 (Plan Generation) to produce a decision-complete plan at `.agent-harness/plans/<slug>.md`. Link it with `agent-harness issueops link-plan`. Von Neumann plans include a dependency matrix, parallel execution waves, and per-task QA scenarios. Use **`karpathy`** Phase 1-2 (SPECIFY + DRAFT) to optimize von-neumann's plan-generation prompt — calibrate model-specific constraints, structure primacy/recency zones, and add adversarial hardening so the generated plan is precise, testable, and free of ambiguity. For database schema changes, invoke **`codd`** Step 2 (NORMALIZE) to audit tables against 1NF→BCNF; normalization violations become plan tasks. For algorithmic work, invoke **`dijkstra`** Step 1-2 (ANALYZE + CLASSIFY) to identify the problem class and optimal algorithm — complexity targets become plan acceptance criteria. **Spawn `brooks` as a devil's-advocate sub-agent** (pattern #4, `devils-advocate-review`) on the completed plan and design review BEFORE implementation: Brooks separates essential from accidental complexity, defends conceptual integrity, exposes the second-system effect (gold-plating, speculative generality), and challenges schedule optimism (Brooks's Law, the mythical man-month). Brooks MUST run as an isolated sub-agent — never inline, because a plan's author carries sunk cost and cannot adversarially review their own plan. If Brooks returns `stop`, **take the feedback loop backward: regress the cycle to `grill`, re-investigate scope/domain, and re-plan** (the `plan` and `compatibility-review` ledger entries are marked stale per the backward-regression rule, retained as audit). A `revise` verdict must be resolved, and any `stop`/`revise` explicitly waived with rationale, before implementation. No implementation until the clearance checklist passes and the design review is approved. |
| **implement** | Use **`turing`** for evidence-bound execution: the main agent performs RED→GREEN→SURFACE→CLEAN TDD directly, drives Manual-QA across 4 channels (HTTP/tmux/browser/computer-use), and tracks quantitative metrics. Sub-agents only per the 12 net-positive patterns (`.agent-harness/SUB_AGENT_PATTERNS.md`). **`dijkstra`** optimizes algorithmic complexity (O(n²)→O(n log n)→O(n)) with benchmark evidence; every optimization must prove complexity class change via scaling tests. **`codd`** Step 3-4 (SCALE + INDEX) designs tables by expected row count and selects indexes with explicit write-penalty justification. **`hopper`** diagnoses test/debug failures via 7-step Hopper Method (REPRODUCE→TRANSLATE→ISOLATE→HYPOTHESIZE→VERIFY→FIX→LEARN). **`torvalds`** handles git operations: worktree creation, atomic commits per Conventional Commit + Lore format, and rebase/cherry-pick as needed. **`atomic-commit-push`** manages staged commits and push safety. |
| **ai-slop-clean** | Use **`shannon`** Phase 0-1 (BASELINE + REGRESSION CHECK) to measure signal-to-noise ratio (SNR), entropy, and redundancy BEFORE cleanup. Use **`turing`** Final Quality Gate step 2 to remove lazy agent artifacts: obvious comments, dead scaffolding, over-defensive code, needless abstraction, duplication, oversized modules. Use **`karpathy`** Phase 3-4 (TEST + DIAGNOSE) to adversarial-test skill prompts used during implementation — detect prompt drift, injection vulnerabilities, and format degradation introduced during coding. Use **`dijkstra`** Step 5 (SIMPLIFY) for structural complexity reduction — replace deep nesting with guard clauses, eliminate modern GOTO patterns. Use **`shannon`** Phase 3 (GATE) to re-measure after cleanup and confirm SNR improved. Record before/after metrics as IssueOps evidence. |
| **feedback** | Use **`turing`** Dynamic Steering to record feedback as structured evidence. For contract-changing feedback, update the remote issue body before continuing. For review feedback, answer in the original thread with verdict, evidence, and next action. Use **`hopper`** to diagnose reported bugs — reproduce the failure exactly, isolate the root cause, and deliver a verified diagnosis. Use **`berners-lee`** to research external root causes (upstream library bugs, known issues, changelog regressions). |
| **pr** | Use **`turing`** Final Quality Gate: spawn an adversarial reviewer sub-agent (pattern #2: Devil's advocate) with the full diff, all success criteria, shannon metrics, and all evidence. Use **`karpathy`** Phase 5 (REFINE) to harden the reviewer sub-agent prompt — add immutability clauses, injection barriers, and output format constraints so the reviewer cannot be redirected by the diff content and always produces a structured verdict. Run targeted verification, AI slop clean, re-verify, and reviewer check. The reviewer verdict is BINDING — unconditional approval only. Use **`torvalds`** for rebase/squash before PR submission, ensuring commit history is clean and atomic. Keep Korean remote artifact, label, assignee, and strict readiness checks in IssueOps. |
| **cleanup** | Use **`turing`** cleanup receipt rules: every QA resource (PIDs, tmux sessions, browser contexts, ports, temp files) must be torn down with a recorded receipt. Use **`torvalds`** for post-merge branch cleanup, worktree removal, and remote branch pruning per safety protocols (verify merged status before delete). Keep merge evidence and worktree/branch cleanup decisions in `references/cleanup-state.md`. |

### Karpathy is cross-cutting prompt augmentation (all phases)

Prompt quality directly drives output quality, so **`karpathy` is not confined to the plan/ai-slop-clean/pr phases — it runs before every sub-agent dispatch and on every authored prompt in any phase.** Before spawning any sub-agent — von-neumann planning, the berners-lee researcher, the dijkstra/codd/hopper specialists, the **brooks** devil's advocate, or the turing reviewer — run `karpathy` (Phase 1-2 SPECIFY+DRAFT, plus Phase 5 REFINE for adversarial/reviewer prompts) to harden the dispatch prompt: state the success contract, add immutability/injection barriers, and constrain the output format. Treat an un-augmented sub-agent prompt as a defect on any quality-affecting task. This is the most leveraged, lowest-cost quality step in the cycle — use it aggressively, not occasionally.

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
| **karpathy** | **all phases (cross-cutting)** | Prompt augmentation before every sub-agent dispatch and on every authored prompt — plan-generation, research, specialist, devil's-advocate, and reviewer prompts. Prompt quality directly drives output quality; harden the dispatch prompt first. Use aggressively, not occasionally. |
| **atomic-commit-push** | implement, pr | Staged commits and push safety: preflight, scope, Conventional Commit + Lore format |
| **brooks** | plan, compatibility-review | Devil's advocate (**sub-agent only**): adversarial plan/design critique before implementation — essential vs accidental complexity, conceptual integrity, second-system effect, schedule honesty. Record the verdict with `issueops devils-advocate review --verdict pass\|revise\|stop`; a recorded pass (or a waived stop/revise with rationale) is a **fail-closed implement-entry gate**. A `stop`'s findings must be reflected into the issue (`issueops remote reflect-devils-advocate --confirm`) before `issueops regress` rewinds the cycle to `grill` for re-plan (feedback loop). |

## Reference Map

Load these files only when the phase applies:

- `references/remote-issue.md`: remote issue first, related issue/label scoring, external LLM judge contract, Korean remote artifact gate, issue template.
- `references/issue-preflight.md`: deep-interview ambiguity reduction and `PROMPT.md`-based ideal issue prompt rewrite before remote issue creation.
- `references/evidence-contract.md`: portable domain contract, API documentation, live evidence, review accountability, and completion hygiene rules.
- `references/worktree-context.md`: branch/worktree contract, local config symlink rules, context routing.
- `references/execution.md`: direct/Orca mode selection, sealed owner claim, generation lease, replacement, reconciliation, publication, and completion.
- `references/orchestration.md`: delegated child-cycle prompt template, scope-drift stop rule, and validation rubric.
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
- Template before remote writes: render remote issue, child-task, and PR/MR bodies through the shared IssueOps template and remote-create wrappers. After ownership transfer, only the acknowledged owner at the canonical worker root may publish the exact verified final head and create the PR/MR for that lifecycle ID. Durable remote-create claims fail closed on ambiguity and must be reconciled against the exact provider project, head, base, title, and rendered body. Hooks never create, merge, or clean remote artifacts.
- Korean remote hook guard: installed PreToolUse hooks include `--enforce-korean-remote-artifacts` and block `gh issue/pr create/edit` when an inspectable title/body fails the Korean remote artifact gate.
- VCS linking hook guard: installed PreToolUse hooks include `--enforce-vcs-issue-linking` and block `gh`/`glab` issue create/edit when the body carries a `Plan Link` section or, on GitLab, a `Related Issues` body section (related issues belong in native linked items). The same guard blocks remote issue/PR/MR create commands without labels or assignee, and blocks PR/MR create when an active IssueOps cycle has `branch_prepare.base_branch` but the inspected target/base branch is missing or different. Copy linked issue labels for PR/MR create or pass an explicit manual label, assign the artifact to the current user, and target the recorded parent work branch. See `references/remote-issue.md` -> "Provider-Specific Linking And Hierarchy".
- No broad review sweeps: subagent reviews must have explicit included paths, excluded large/generated paths, a time budget, and a fallback direct verification path.
- Cleanup choices: after a PR/MR is merged, verify merge/worktree/branch status and present numbered cleanup choices before deleting local worktrees or branches.
- Execution cleanup authority: `issueops execution complete` moves the cycle to `done` and releases its generation, but never merges or deletes resources. After verified merge evidence, perform only the cleanup action the human explicitly authorizes. Record current observations; stale evidence never authorizes worktree removal.
- Numbered next actions: at user decision points and after reporting review/feedback/cleanup status, end with `선택지:` and three numbered choices. Installed Stop hooks with `--enforce-numbered-next-actions` block missing choices and tell the agent to explain the block before presenting context-specific choices.
- Next-action Stop hook: to reduce friction, the Stop hook may re-enter the main agent when the final response contains an explicit `선택지:` section. The hook is not a judge, scorer, classifier, or safety gate; it relays observed facts such as choice count, recommendation count, and recommended text. The main agent must judge safety, reversibility, user-intent alignment, and whether to proceed or ask the user from the current context, then state why it is auto-proceeding or why it is not auto-proceeding and needs user confirmation. Auto-proceed result reports still end with `선택지:` so the next action boundary remains explicit. Mark exactly one recommended option only when the main agent itself judges it safe, reversible, and aligned.
- Worker identity check: every implementation, TDD, review, QA, or subagent worker must first report and verify `pwd`, branch, `HEAD`, and the expected isolated worktree path before inspecting or changing anything.
- Host usage/model user-decision boundary: usage-limit, rate-limit, reset, and model-selection prompts require the user/coordinator. Dismiss or stop and relay; never navigate, confirm, reset usage, or switch models automatically.
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
- Host-agent judge boundary: IssueOps never calls an external LLM service in-process. Render a read-only prompt, dispatch it to a fresh independent host agent, and validate the returned JSON through the `file` backend before any remote artifact write.

## Gate Quick Reference

When an IssueOps command reports a missing gate, do not guess a new hidden flag. Use the command that owns that state:

- `intent_contract`: run `issueops intent record` with raw request, interpreted intent, success criteria, constraints/non-goals/ambiguity when known. Pass `--intent-class trivial|standard|refactoring|architecture|research`; an empty class normalizes to `standard` and trivial skips the plan-prep gate.
- `plan_prep_decisions` / `plan_prep_related_issues` / `plan_prep_web_research` / `plan_prep_codebase_survey`: run `issueops plan-prep record` with evidence or a waive reason per item before entering the `plan` phase. `plan_prep_codebase_survey` takes `--codebase-survey-evidence` (tools used plus the touched symbols/files/call paths and reuse candidates found) or `--codebase-survey-waive` (only when the change creates net-new files with no existing code to survey). Enforced only for non-trivial intent classes; design review does not require it because design review runs inside the plan phase where plan-prep is already satisfied.
- `branch_prepare` / `branch_link_verified`: normally run `issueops branch prepare` only after provider-visible branch evidence exists. GitHub Orca is the exception: record the matching GitHub issue identity and exact base SHA without `--link-verified`, run Orca prepare so its branch remains local-only, then create the linked branch and rerun `branch prepare --link-verified` from the claimed owner before linking the plan or implementing. The branch must start with the issue/task number and a hyphen.
- `worktree_path` / `worktree_exists` / execution lease: preview and confirm `issueops execution prepare --mode auto`. Use the returned canonical worktree and set up dependencies there with the repository's documented command.
- `compatibility_review` / `backward_compatibility` / `side_effects` / `rollback_plan` / `compatibility_verification` / `compatibility_blockers` / `compatibility_approval`: run `issueops compatibility review`. Record backward compatibility findings, side effects, rollback plan, verification evidence, and approval. Do not approve while blockers remain.
- `execution` / `execution_write_lease`: run `issueops execution status`. Prepare a missing execution, claim an Orca generation with the rendered token-file command, or use the generation-CAS replacement/reconciliation command returned by status. Never invent an override.
- `design_review`, `design_approval`, `design_review_evidence`, `refactor_plan`, `alternatives`, `risks`, `design_open_questions`: run one full `issueops design review` call. Approval is recorded with the full design review payload; there is no approve-only merge step.
- `plan_path` / `plan_exists` / `plan_in_worktree`: create the plan file inside the linked worktree, then run `issueops link-plan`.
- `ai_slop_clean`: record AI slop cleanup after implementation changes exist in the linked worktree.
- `contract_feedback_issue_update`: update the remote issue body for contract-changing feedback, then run `issueops feedback mark-issue-updated`.
- `child_incomplete` | `issueops child status`: inspect child phase, heartbeat age, worktree, and latest evidence; continue or recover the child before parent PR readiness.
- `child_unvalidated` | `issueops child accept`: validate the done child with evidence, or reject/drop it with a reason when the result is not acceptable.
- `child_rejected_unresolved` | `issueops child accept` or `issueops child drop`: resolve a rejected child by accepting corrected evidence or dropping it from the parent gate with an auditable reason.
- `children_active` | `issueops child status`: active children prevent parent regression/cleanup shortcuts; inspect children and stop at the owner decision boundary.

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

## Concept → Command Map

The IssueOps skill prose uses vivid domain nouns — phase names, decision verbs, ledger artifact names — that are **not** `issueops` subcommands. The CLI uses generic verbs (`phase`, `remote`, `link-related`). Guessing `issueops <domain-word>` fails. When unsure, run `issueops --help` for the real registry; the CLI also emits a did-you-mean hint for the common confusions below.

| Domain word in this skill | What it actually is | Real CLI command |
|---|---|---|
| `grill` / `problem` / `implement` / `ai-slop-clean` / `feedback` / `pr` | **lifecycle phase** | `issueops phase --id ID --to <phase>` |
| `split` | **breakdown decision** (no-split default; see Large Issue Breakdown Gate) | `issueops remote create-child` to create child tasks, `issueops link-related --type splits-from` to link an existing one |
| `domain` (review) | **grill-phase ledger artifact** | `issueops domain-review record` |
| `compatibility` (review) | **plan-phase ledger artifact** | `issueops compatibility review` |
| `devils-advocate` / `brooks` (verdict) | **fail-closed implement-entry gate** | `issueops devils-advocate review --verdict pass\|revise\|stop` |
| reflect findings to issue (stop) | **regress precondition** | `issueops remote reflect-devils-advocate --confirm` |
| `design` (review) | **plan-phase ledger artifact** | `issueops design review` |
| `intent` (contract) | **problem-phase ledger artifact** | `issueops intent record` |
| `regress` (for replan) | **feedback action** | `issueops regress` |
| `delegated child` | **parent-owned sub-agent cycle reference** | `issueops child start/status/accept/reject/drop` |
| `artifact` (plan/spec/turing-loop 전달) | **prepare 전 스테이징 → materialize/봉인** | `issueops artifact stage/unstage` |
| `implementation review` / 구현 diff brooks | **orca 모드 publication 게이트** | `issueops implementation-review record --verdict pass\|revise\|stop` |
| 다중 사이클 조망 / cleanup 후보 | **read-only 집계 표면** | `issueops list [--repo PATH]` |
| 머지 후 정리(finish) | **record-backed 정리 + 레코드 삭제** | `issueops cleanup finish (--preview \| --apply --confirm --fingerprint SHA)` |
| 완료 기록/이슈 close | **completion 섹션 보존·부모 이슈 close** | `issueops remote reflect-completion` / `issueops remote close-issue` |
| `child validation` | **parent verdict over child evidence** | `issueops child accept` or `issueops child reject` |

## Quality Upgrade Gates

IssueOps must leave an auditable decision trail for labels, large issue breakdown, draft issue completion, and PR/MR review-agent feedback.

- Before any remote issue or PR/MR write, record the **threshold-based label decision**: selected labels, rejected labels, and manual override reason when no label crosses threshold. Use `issueops remote score` first, then apply only selected labels or stop before writing.
- For broad or multi-step work, run the **Large Issue Breakdown Gate**: create provider-native child work items before implementation when the parent issue would otherwise hide independent tasks. Use GitHub sub-issues or GitLab child items, then record each existing child with `agent-harness issueops link-child`.
- Do not split merely because work is broad or multi-step. The gate's default is no split; create provider-native child work items only when one issue would be unsafe or collaboration/parallel ownership was explicitly requested.
- On completion, write a **draft issue completion record** in the remote issue or PR/MR-ready notes before reporting done. It must summarize final diff, verification evidence, selected labels, child links, PR/MR URL, cleanup status, and unresolved follow-ups.
- Treat Kodus, Gemini Code Assist, and similar automated reviewers as **review-agent feedback**. Verify each claim, reply in the original thread with verdict and evidence, and resolve only threads whose fix or obsolescence has been verified.

Use this remote issue scoring choice shape before creating or editing an issue:

```text
관련 이슈/라벨 후보를 먼저 deterministic scorer로 점검하고, read-only prompt를 fresh host agent가 독립 평가한 결과만 `file` backend로 검증해 threshold 이상을 반영하겠습니다.
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

For the "확장 정리" choice, inspect each known cycle and its worktree, branch,
remote artifact, native process, and execution generation. There is no bulk
unsafe release. A live or ambiguous holder stays fenced; recovery uses the
previewed generation-CAS `issueops execution replace` sequence. Worktree and
branch deletion still requires verified merge evidence and a separate human
choice.

## Background LLM Gates

Remote scoring is a `background_join` LLM gate. It may run while local planning or implementation continues, but the main IssueOps loop must join the result before any remote artifact write: issue create/edit, label create/apply, PR/MR create/edit, assignment, or comment. If label candidates existed but none met threshold, stop before remote writes and choose an explicit manual label or rerun scoring with corrected candidates; do not create an unlabeled issue, PR, or MR.

Do not put polling or waiting in lifecycle hooks. Hooks may surface a status hint only. Completion is decided by the main loop at the join point by checking the stored job/result status and requiring success before the remote write.

Host-agent judges are read-only evaluators. Their prompts must forbid workspace inspection, tool execution, file changes, git actions, issue/label/PR/MR mutation, comments, assignment, closing/reopening, or state changes. They may only return judgment JSON that the main loop applies after validation.

### Benchmark Judge Protocol (independent host-agent)

When an IssueOps benchmark needs a semantic judge, use a fresh-context host agent that did not author the artifacts:

1. Run the deterministic pass first: `agent-harness issueops benchmark run --fixtures <dir> --judge none --json`.
2. The main agent dispatches a **fresh-context** sub-agent (no inherited conversation context, never the author of the artifacts being judged — no self-scoring) with a deterministic input packet: ① the rubric dimension list with one-line definitions, ② the artifact fields to judge, ③ the required output: a `{"<fixtureID>": <IssueOpsBenchmarkScore>}` map as JSON only, no preamble.
3. Feed the returned map through `agent-harness issueops benchmark run --fixtures <dir> --judge file --judge-file <map.json> --json`. The CLI strict-decodes each score and fails closed on missing/unknown fixture keys.

The no-self-approval constraint is a documented orchestration protocol: the Go `--judge file` layer only sees bytes and cannot verify who produced the judgment, so enforcement lives in the coordinator's dispatch discipline.

## Operational Start

Start or resume state only after deriving the issue branch slug. The IssueOps branch must be the issue branch, not the source checkout's current branch. The compact command sequence and examples live in `references/operational-start.md`; load that reference only when you are actively running or documenting an IssueOps cycle.

The first-turn essentials are:

- Start/status: `agent-harness issueops start --repo "$PWD" --branch "$branch_slug" --json`, then `agent-harness issueops status --id "$ISSUEOPS_ID" --json`.
- Intent and plan prep: run `agent-harness issueops intent record`; record the raw request, interpreted intent, success criteria, constraints, ambiguity ledger, non-goals, `intent_contract`, `plan_prep_decisions`, `plan_prep_related_issues`, `plan_prep_web_research`, and `plan_prep_codebase_survey` before plan entry.
- Branch/worktree/design: record provider branch linkage and exact base SHA, approve `agent-harness issueops design review`, then preview/confirm `agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto`; create and link the plan inside the returned canonical worktree.
- Implementation gates: inspect `issueops execution status`, set up dependencies in the canonical worktree, record compatibility and devil's-advocate reviews, and enter implementation only when the active generation and design gates are ready.
- Completion gates: `ai_slop_clean` evidence must be current before `pr`; contract-changing feedback must be reflected remotely; the active generation creates and verifies the draft PR/MR, then `issueops execution complete` records the final receipt and releases the lease.

Remote scoring and benchmark commands are CLI-only developer/autoresearch tooling. The only IssueOps MCP surface is `issueops_execution`, whose actions mirror the execution v1 state machine.

## Stop Conditions

Stop and ask before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Stop before implementation if `issueops intent record` or `issueops design review` cannot be completed from evidence. Do not treat a recommended next-action option as permission to continue unless the main agent records why continuation is safe, reversible, and aligned with the user's latest instruction.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `issue_url`, `branch_prepare`, `branch_link_verified`, `plan_path`, `worktree_path`, `worktree_exists`, `branch_match`, `worktree_clean`, `upstream`, `upstream_synced`, `plan_exists`, or `ai_slop_clean`.

Do not move to PR/MR drafting when `issueops pr-readiness --strict` reports missing `contract_feedback_issue_update`. This means a `contract_change` feedback item was recorded after the remote issue contract changed, and the remote issue body update has not been confirmed with `issueops feedback mark-issue-updated`.

Do not mark an IssueOps loop `done` before it has entered the `pr` phase. Completion reporting happens after PR/MR readiness and review/merge hygiene, not as an escape hatch from planning or implementation.

Before PR/MR create, verify the linked issue labels and pass them to the provider create command. If the linked issue has no labels, create or apply an explicit manual label first, or stop and record label-decision feedback; never create the PR/MR with an empty label set.

## Execution ownership

Load `references/execution.md` for the full direct/Orca contract and
`references/cleanup-state.md` for the post-merge boundary. Fence only the exact
lifecycle ID, generation, native holder, canonical worktree, and persisted Orca
resource. The source main worktree remains available before, during, and after
execution for unrelated work. Successful `issueops execution complete` records
`done` and releases the generation; it never merges or removes resources.
