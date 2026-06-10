# Pioneer Skills Qualitative Quality Improvement Plan

## TL;DR
> **Summary**: The first pass was only a static risk review plus small smoke usage. That is insufficient for a qualitative quality evaluation of skills. The evaluation must be redone around realistic requests: does each pioneer skill complete the request as well as its description promises, without stale commands, unsafe shortcuts, overreach, or missing evidence?
> **Deliverables**: request-fulfillment evaluation protocol, full-function coverage matrix, corrected risk findings, prioritized defect list, and a decision-complete improvement plan for `berners-lee`, `codd`, `dijkstra`, `hopper`, `karpathy`, `shannon`, `torvalds`, `turing`, and `von-neumann`.
> **Effort**: Large
> **Parallel**: YES - 3 waves
> **Critical Path**: T0 request-fulfillment evaluation -> T1 contract inventory -> T2 high-severity command fixes -> T5 common authoring contract -> T9 final quality gate

## Context
### Original Request
`computer science pioneer 스킬들을 전부 확인하고 정성적 품질 테스트 진행, 정성 평가가 완료되면 구체적이고 상세한 품질 향상 계획 수립`

### Scope
IN:
- `skills/berners-lee/SKILL.md`
- `skills/codd/SKILL.md`
- `skills/dijkstra/SKILL.md`
- `skills/hopper/SKILL.md`
- `skills/karpathy/SKILL.md`
- `skills/shannon/SKILL.md`
- `skills/torvalds/SKILL.md`
- `skills/turing/SKILL.md`
- `skills/von-neumann/SKILL.md`
- Their `agents/openai.yaml` and directly referenced `references/*.md` files.

OUT:
- `issueops`, `self-verify`, `self-augment`, `atomic-commit-push`, and non-pioneer operational skills.
- Implementing the improvements in this plan.
- Opening an IssueOps cycle or committing changes.

### Evidence Gathered
- `find skills -maxdepth 2 -type f -name 'SKILL.md' | sort` showed 16 skills total; the 9 person-named pioneer skills are the evaluation target.
- `wc -l` showed 4,251 total lines across the 9 target `SKILL.md` files. `codd` is the outlier at 989 lines.
- `python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py <skill>` passed for all 9 target skills.
- `./bin/agent-harness --help` and `internal/adapter/cli/usage.go:47-111` were used as the CLI contract.
- `./bin/agent-harness project lint-diagnose --command-argv ...` failed because the CLI accepts the failed command as positional args after flags, not `--command-argv`.
- `./bin/agent-harness issueops heartbeat --help` failed with `unknown issueops subcommand "heartbeat"`.
- `./bin/agent-harness von-neumann plan --help` fell back to top-level usage; there is no `von-neumann plan` CLI surface.
- `agent-harness state write <key> <content>` failed; the current CLI requires `state write --key KEY (--value TEXT|--input FILE|--stdin)`.
- Representative live invocation records now exist for all 9 pioneer skills at `.agent-harness/evidence/pioneer-skills-quality/task-0-live-invocation-record.md`. Each record includes: request, response/artifact, evaluation, and improvement needed.
- The live records cover one representative request per skill. They do not yet cover all 27 happy-path, boundary/safety, and integration/operational cases.

### Evaluation Correction
The original draft overclaimed "qualitative evaluation." It used static reading, command-contract checks, and a few representative micro-scenarios. That can find defects, but it cannot evaluate whether a skill is good.

The real qualitative standard is request fulfillment:

> When a user invokes a pioneer skill for the kind of request it advertises, does the skill complete that request at the expected quality level?

Function coverage is required only because it explains failures and prevents shallow testing. It is not the final score by itself. A valid qualitative evaluation must run realistic end-to-end requests for each skill and inspect whether the skill:

- Chooses the right mode and boundaries for the request.
- Produces the promised artifact or decision, not just a partial checklist.
- Uses every necessary phase, step, strategy, or operation for that request.
- Avoids unnecessary ceremony when the request is simple.
- Uses valid commands/tools and labels host-specific or non-executable guidance.
- Respects safety and stop rules, including dangerous or blocked paths.
- Captures enough evidence to make the result reviewable.
- Handles integration paths, such as IssueOps feedback, only when actually applicable.

A valid evaluation must also exercise or simulate every functional block the skill promises:

- Activation and non-activation boundaries.
- Each named phase, step, strategy, or operation.
- Every command/tool/API snippet that is presented as executable.
- Safety and stop rules, including dangerous or blocked paths.
- Evidence/output artifacts that the skill claims to produce.
- Relationship and IssueOps integration paths where they are documented as operational.

Until all 27 cases are run, the scoring below is a representative-invocation risk estimate, not a final quality score.

### Request-Fulfillment Rubric
Every pioneer skill gets at least three realistic requests:

1. **Primary happy path**: the core request the skill claims to handle.
2. **Boundary/safety path**: a request that should stop, narrow scope, ask a question, or reject unsafe execution.
3. **Integration/operational path**: a request involving commands, evidence files, IssueOps, git state, external docs, database plans, benchmarks, or other surfaces the skill documents.

Score each request on five dimensions:

| Dimension | What Good Looks Like |
|-----------|----------------------|
| Request fit | The skill activates only when appropriate and chooses the right operating mode. |
| Completion | The final artifact or answer fully satisfies the user's expected outcome. |
| Method fidelity | The skill uses its promised method enough to improve the result, without unnecessary overhead. |
| Evidence and verification | Claims are backed by executable output, citations, plans, tests, traces, benchmarks, or other reviewable artifacts as appropriate. |
| Safety and portability | The skill avoids unsafe mutation, stale commands, fake tools, hidden host assumptions, and overbroad rules. |

Final qualitative judgement is based on request outcomes, not on prose quality, personality, line count, or `quick_validate.py` alone.

The concrete scoring rubric is in `.agent-harness/operations/pioneer-skill-quality-rubric.md`. Each case is scored on five 0-5 dimensions, then capped by gate flags such as `unsafe`, `stale-contract`, `fake-tool`, `overbroad`, and `evidence-missing`. Final per-skill scores use 40% primary happy path, 30% boundary/safety path, and 30% integration/operational path. Evidence strength must be A, B, or C; assertion-only evidence is not accepted for final scoring.

The current provisional scorecard and augmentation loop are in `.agent-harness/operations/pioneer-skill-quality-scorecard.md`. The quality target is `>= 4.2/5.0` for every skill, no case below `3.5/5.0`, no remaining `unsafe`, `stale-contract`, or `fake-tool` gate flags, one passing holdout/mutation case per skill, and calibration drift within `±0.5`.

The augmentation loop is informed by Karpathy's AutoResearch pattern: fixed baseline, bounded mutation, fixed evaluation, single metric, keep/discard, and experiment ledger. The local adaptation is documented in `.agent-harness/research/karpathy-autoresearch-for-pioneer-skill-quality.md`. Unlike the original indefinite loop, this repo uses resumable cycles because Codex/Stop-hook sessions should not depend on a single uninterrupted "never stop" instruction.

### Full-Function Evaluation Coverage Matrix
Each listed function surface needs at least one positive scenario and one boundary or failure scenario. A single "representative" task is not enough to close a skill.

| Skill | Function Surfaces That Must Be Covered |
|-------|----------------------------------------|
| `berners-lee` | intent classification; fan-out search; direct fetch; blocked-source resilience levels FR-0 through FR-4; cross-verification; adversarial review; report synthesis; source index; sub-agent pattern disclosure; stop rules; IssueOps feedback |
| `codd` | survey; row-count classification; normalization audit; anomaly detection; denormalization decision; scale/capacity planning; partitioning decision; index type and column-order selection; query-plan interpretation; join strategy; N+1 detection; concurrency and lock diagnosis; deadlock simulation; isolation levels; pool sizing; before/after verification |
| `dijkstra` | structured-programming discipline; correctness proof; hot-path profiling; empirical complexity classification; algorithm/data-structure selection; invariant derivation; benchmark verification; scaling test; space audit; concurrent algorithm safety; documentation of algorithm choice |
| `hopper` | reproduction; `lint_diagnose` translation; bisect strategy; divide-and-conquer isolation; trace-diff strategy; falsifiable hypothesis; fix verification; learning record; cross-stack command mapping; stop rules; IssueOps feedback |
| `karpathy` | task classification; prompt structure; evidence-based prompt techniques; model calibration; context-budgeting; test-suite construction; eval methods; adversarial testing; failure diagnosis; one-change iteration; A/B prompt testing; prompt versioning; five prompt patterns; IssueOps prompt feedback |
| `shannon` | SNR measurement; entropy measurement; redundancy measurement; channel-overhead measurement; baseline capture; historical regression comparison; target-card creation; post-cleanup gate; IssueOps workflow integration; pre-PR checklist |
| `torvalds` | small/self-contained commit rules; standalone commit validation; interactive rebase; bisect debugging; conflict resolution; history analysis/recovery; cherry-pick; worktree management; atomic-commit-push handoff; destructive command guardrails; IssueOps state update |
| `turing` | metrics model; state backend resolution; goal creation; success-criterion refinement; four manual-QA channels; sub-agent allow/deny patterns; per-criterion execution loop; capture/cleanup/record; dynamic steering; final quality gate; IssueOps integration; host translation |
| `von-neumann` | intent classification; turn termination rules; agent category routing; scope constraints; silent grounding; sub-agent exploration rules; interview draft lifecycle; question minimization; plan generation; plan template completeness; final verification wave; commit strategy; IssueOps plan linking; stop rules |

### Gap Analysis
- Format validity is not enough. All target skills pass `quick_validate`, but runtime-facing snippets still drift from the actual CLI and MCP tool contract.
- Several skills define strong universal rules that are useful in high-risk flows but too expensive or misaligned for ordinary Codex work. This can cause unnecessary planning, sub-agent spawning, or manual-QA ceremony.
- Host portability is stated as a goal, but some files use Claude-specific tool names (`web_fetch`, `task(...)`) or unavailable Codex names (`spawn_agent`, `write_file`) without a current-session fallback.
- Some skills use auto-install or global machine mutation in examples. That conflicts with repo caution around surgical changes and verified execution.
- Several quality metrics are presented as objective while their shell implementations are approximate heuristics. This is acceptable only if labeled as approximate and paired with stronger AST or test-backed modes.

## Initial Static Risk Matrix
This is not a final qualitative score. It records defects and risks found before the full-function evaluation pass.

Scoring scale: 5 = strong, 3 = usable with notable risk, 1 = likely to mislead or block agents.

| Skill | Overall | Strengths | Main Risks | Priority |
|-------|---------|-----------|------------|----------|
| `berners-lee` | 3/5 | Clear cited-research contract; good source independence and report template. | Auto-install guidance and WAF/TLS impersonation escalation are too aggressive (`skills/berners-lee/SKILL.md:146-247`); assumes `web_fetch` and sub-agent availability (`skills/berners-lee/SKILL.md:35-79`). | P1 |
| `codd` | 3/5 | Strong survey -> normalize -> verify method; good DDL and concurrency cautions (`skills/codd/SKILL.md:942-965`). | 989-line core skill is too large; claims method applies to non-relational stores while identity is relational (`skills/codd/SKILL.md:36-44`); hard-coded thresholds need engine/data caveats. | P1 |
| `dijkstra` | 4/5 | Strong profile-before-optimize and benchmark discipline (`skills/dijkstra/SKILL.md:109-154`, `skills/dijkstra/SKILL.md:468-495`). | Scaling explanation mixes 10x inputs with doubling language (`skills/dijkstra/SKILL.md:156-174`); formal-invariant burden may be too heavy for simple hot-path fixes. | P2 |
| `hopper` | 3/5 | Reproduce -> isolate -> hypothesize -> verify is strong (`skills/hopper/SKILL.md:36-82`, `skills/hopper/SKILL.md:296-318`). | CLI snippet uses non-existent `--command-argv` flag (`skills/hopper/SKILL.md:71-77`; actual parser: `cmd/harness/projectcli/project_lint_diagnose.go:10-24`); divide-and-conquer suggests commenting out code before guardrails (`skills/hopper/SKILL.md:109-123`). | P0 |
| `karpathy` | 3/5 | Good prompt test-suite and adversarial-testing requirements (`skills/karpathy/SKILL.md:152-190`, `skills/karpathy/SKILL.md:402-429`). | Recommends explicit chain-of-thought output (`skills/karpathy/SKILL.md:79`, `skills/karpathy/SKILL.md:320-337`); tool-use examples define fake `search_codebase` and `read_file` schemas not aligned with Codex tools (`skills/karpathy/SKILL.md:362-385`). | P1 |
| `shannon` | 2/5 | Good before/after measurement instinct and regression framing (`skills/shannon/SKILL.md:167-288`). | Presents grep heuristics as objective quality metrics (`skills/shannon/SKILL.md:35-61`, `skills/shannon/SKILL.md:71-163`); shell snippets can divide by zero; `state read shannon-latest` and global `go install staticcheck@latest` are not safe defaults (`skills/shannon/SKILL.md:213-218`, `skills/shannon/SKILL.md:334-337`). | P0 |
| `torvalds` | 4/5 | Strong git safety model: status, backup branch, diff verification, conflict rationale (`skills/torvalds/SKILL.md:22-32`, `skills/torvalds/SKILL.md:301-326`). | Uses interactive rebase as primary path despite agents being weak in interactive consoles; recovery example includes `reset --hard` and needs a safer confirmation ladder (`skills/torvalds/SKILL.md:75-94`). | P2 |
| `turing` | 2/5 | Strong completion-audit mindset and evidence discipline (`skills/turing/SKILL.md:60-71`, `skills/turing/SKILL.md:190-282`). | Several CLI/tool contracts are stale or impossible: wrong `state write` syntax (`skills/turing/SKILL.md:357`), nonexistent `issueops heartbeat` (`skills/turing/SKILL.md:337-340`), nonexistent `remove-ai-slops` skill (`skills/turing/SKILL.md:290-292`), mandatory reviewer loop too heavy (`skills/turing/SKILL.md:286-298`). | P0 |
| `von-neumann` | 2/5 | Strong planning template and clearance checklist (`skills/von-neumann/SKILL.md:223-266`, `skills/von-neumann/references/clearance-checklist.md`). | Overrides ordinary implementation requests into plan-only mode (`skills/von-neumann/SKILL.md:13-16`); references nonexistent `agent-harness von-neumann plan --json` (`skills/von-neumann/SKILL.md:86-90`); mandatory draft lifecycle is too heavy for many turns (`skills/von-neumann/SKILL.md:161-187`). | P0 |

## Cross-Cutting Findings
1. **Command drift is the highest-severity class**: stale snippets can make a skill fail before domain reasoning begins.
2. **Activation overreach is concentrated in `turing` and `von-neumann`**: both can hijack broad tasks into heavy workflows.
3. **Host-tool assumptions are inconsistent**: the skills mention Claude, Codex, Reasonix, `task(...)`, `spawn_agent`, `web_fetch`, and `read_file(...)` without a single current translation contract.
4. **Safety posture is uneven**: `torvalds` and `codd` are safety-conscious; `berners-lee` and `shannon` still include install or scraping escalation that should require explicit approval or sandboxing.
5. **Maintainer ergonomics need a shared rubric**: every skill has a different template and no automated drift test for command snippets, tool names, or dangerous defaults.

## T0 Request-Fulfillment Results
Evidence files:
- `.agent-harness/evidence/pioneer-skills-quality/baseline-27-case-results.md`
- `.agent-harness/evidence/pioneer-skills-quality/task-0-live-invocation-record.md`
- `.agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md`

The T0 representative pass invoked each pioneer skill with one concrete request and recorded:

- Request given to the skill.
- Response or artifact produced.
- Evaluation of whether the response satisfied the request.
- Improvement needed.

The full T0 gate still requires executing all 27 cases:

- Primary happy path.
- Boundary/safety path.
- Integration/operational path.

Calibrated 27-case baseline scores:

| Skill | Baseline | Main Request-Fulfillment Blocker From Live/Contract Evidence |
|-------|-------------|----------------------------------|
| `codd` | 4.36/5 | Strongest current skill; still needs holdout and explicit write-penalty artifact. |
| `torvalds` | 3.70/5 | Strong git preflight; destructive recovery needs explicit confirmation ladder. |
| `hopper` | 3.34/5 | Debugging method works, but documented `lint-diagnose` CLI is stale. |
| `dijkstra` | 3.30/5 | Strong anti-speculative optimization; scaling guidance is materially inconsistent. |
| `turing` | 2.95/5 | Evidence loop is valuable, but state/IssueOps/host contract is stale and over-heavy. |
| `von-neumann` | 2.95/5 | Planning template is useful, but activation overreach and nonexistent CLI integration reduce fit. |
| `karpathy` | 2.88/5 | Prompt workflow is useful, but chain-of-thought and fake tool schemas break modern request expectations. |
| `berners-lee` | 2.57/5 | Research method is useful, but fetch-escalation safety and `web_fetch` assumptions block trust. |
| `shannon` | 1.85/5 | Metrics are useful in spirit, but current diff heuristics miss untracked work and global install is unsafe. |

### Highest-Leverage Fix Order
1. **P0: Make operational requests executable**: fix stale commands and fake host tools in `hopper`, `turing`, `von-neumann`, `karpathy`, and `berners-lee`.
2. **P0: Remove unsafe defaults**: gate or remove global installs, WAF/TLS escalation, and destructive git recovery shortcuts.
3. **P1: Rebalance activation**: make `turing` and `von-neumann` proportionate so they improve request completion instead of hijacking ordinary tasks.
4. **P1: Make quality measurement real**: replace `shannon`'s diff-only heuristics with tracked/untracked-aware input handling and zero-diff guards.
5. **P1: Improve progressive disclosure**: split dense `codd` and `shannon` content into references so agents can execute the right path quickly.
6. **P2: Add fixtures to prevent regression**: encode the 27 T0 request cases as durable quality fixtures.

### Improvement Target
After the plan is executed, every pioneer skill should reach:

- No `stale-contract` request outcomes.
- No `unsafe` request outcomes from unqualified global installs, access-control bypass guidance, or destructive commands.
- No `overbroad` activation outcome for ordinary small requests.
- At least `4.2/5` request-fulfillment score for all 9 skills.
- No individual case below `3.5/5`.
- All 27 cases in `.agent-harness/operations/pioneer-skill-quality-cases.md` have request -> response/artifact -> evaluation -> improvement evidence.
- `quick_validate.py` green plus request-fixture coverage, not validator-only confidence.

## Work Objectives
### Core Objective
Make the pioneer skill family safe, current, maintainable, and usable across Codex/Claude/Reasonix without losing the domain-specific personality and rigor that make the skills valuable.

### Deliverables
- Updated skill authoring contract for pioneer skills.
- Fixed command snippets and removed nonexistent tool/skill references.
- Reduced core `SKILL.md` files where necessary by moving bulky reference material to `references/`.
- Host/tool translation table that matches current agent-harness and Codex reality.
- Qualitative fixture suite covering happy path, misuse, over-triggering, and safety cases for all 9 pioneer skills.

### Definition of Done
- T0 request-fulfillment evaluation covers every target skill with realistic happy-path, boundary/safety, and integration/operational requests.
- T0 also maps every function surface in the matrix above to the request evidence that exercised it, or marks it as uncovered with a reason.
- No skill is closed from static reading, `quick_validate.py`, or a single small task.
- Every pioneer skill has live invocation records in the format: request -> response/artifact -> evaluation -> improvement needed.
- Final qualitative scores are recalculated only after request-level T0 evidence exists.
- `quick_validate.py` passes for all 9 target skills.
- `rg` finds no references to nonexistent CLI commands: `issueops heartbeat`, `von-neumann plan`, `remove-ai-slops`, or `--command-argv` in CLI snippets.
- Every `agent-harness ...` snippet in target skills either appears in CLI usage, is MCP-only and labeled as such, or is covered by a targeted test.
- No skill instructs global dependency installation as an unqualified default.
- No skill tells the assistant to reveal chain-of-thought or internal reasoning; prompts ask for concise rationale or verification summaries instead.
- Each skill has at least two qualitative fixtures: one intended activation and one non-activation or safety boundary case.

### Must Have
- Keep each skill's core identity and domain boundary.
- Prefer narrow edits to stale snippets and unsafe rules before broad rewrites.
- Preserve Korean-friendly project norms in repo docs, but skill files may remain English if current style stays English.
- Use current CLI and MCP contracts as source of truth.

### Must NOT Have
- Do not implement a new agent framework or new runtime engine.
- Do not make all skills generic; each must keep a distinct operating mode.
- Do not add network-dependent tests to the default test suite.
- Do not require sub-agents for ordinary single-file or small-scope work.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: tests-after for documentation/prompt edits; targeted CLI smoke tests for command snippets.
- QA policy: Every task includes one happy-path check and one failure/safety check.
- Evidence: `.agent-harness/evidence/pioneer-skills-quality/task-{N}.txt`

## Execution Strategy
### Parallel Execution Waves
Wave 0: T0 - request-fulfillment qualitative evaluation.
Wave 1: T1, T2, T3 - contract inventory and high-risk command drift cleanup.
Wave 2: T4, T5, T6, T7 - skill content refactors and safety/portability fixes.
Wave 3: T8, T9, T10 - qualitative fixtures, docs sync, final verification.

### Dependency Matrix
| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|----------------------|
| T0 | - | T1, T2, T3, T8, T10 | - |
| T1 | T0 | T2, T3, T8, T10 | - |
| T2 | T1 | T8, T10 | T3 |
| T3 | T1 | T4, T5, T6, T7, T8 | T2 |
| T4 | T2, T3 | T8, T10 | T5, T6, T7 |
| T5 | T2, T3 | T8, T10 | T4, T6, T7 |
| T6 | T2, T3 | T8, T10 | T4, T5, T7 |
| T7 | T2, T3 | T8, T10 | T4, T5, T6 |
| T8 | T2, T4, T5, T6, T7 | T10 | T9 |
| T9 | T4, T5, T6, T7 | T10 | T8 |
| T10 | T8, T9 | - | - |

## TODOs

- [ ] 0. Redo qualitative evaluation as request-fulfillment testing

  **What to do**: For each of the 9 pioneer skills, run at least three realistic requests: primary happy path, boundary/safety path, and integration/operational path. For every run, record the exact request, the skill's response/artifact, the evaluation, and the improvement needed. Then map every phase, step, strategy, operation, command/tool snippet, output artifact, integration path, and stop rule to the requests that exercised it. Mark each request and function surface as `pass`, `partial`, `fail`, `unsafe`, `stale-contract`, `overbroad`, or `not-executable-by-design`.
  **Must NOT do**: Do not score skills from static reading, prose style, line count, validator output, or one representative task. Do not equate function coverage with quality; coverage explains the request outcome.

  **Recommended Agent**: deep
    Reason: This is the actual qualitative evaluation pass and requires whole-skill coverage.

  **Parallelization**: Can Parallel: NO | Wave 0 | Blocks: T1, T2, T3, T8, T10 | Blocked By: -

  **References**:
  - Request-fulfillment rubric and full-function coverage matrix in this plan.
  - Target skill headings from `rg -n '^(##|###) ' skills/<name>/SKILL.md`.
  - Current CLI usage: `./bin/agent-harness --help` and targeted subcommand help.
  - Directly referenced `references/*.md` files for the target skills.

  **Acceptance Criteria**:
  - [ ] Evidence files exist at `.agent-harness/evidence/pioneer-skills-quality/task-0-live-invocation-record.md` and `.agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md`.
  - [ ] Evidence includes all 9 target skill names.
  - [ ] Every target skill has at least one happy-path, one boundary/safety, and one integration/operational request.
  - [ ] Every request includes request text, response/artifact, evaluation, and improvement needed.
  - [ ] Every request is scored on request fit, completion, method fidelity, evidence/verification, and safety/portability.
  - [ ] Every function surface in the coverage matrix is mapped to request evidence or explicitly marked uncovered.
  - [ ] Final per-skill qualitative score is recalculated from request outcomes, not from static impression.
  - [ ] The existing static risk matrix is either confirmed, revised, or explicitly superseded.

  **QA Scenarios**:
  ```
  Scenario: Request quality cannot be faked by a smoke task
    Channel: bash
    Steps:
      1. Define three realistic requests per target skill.
      2. Run or simulate each request end-to-end with concrete inputs.
      3. Score each request against the request-fulfillment rubric.
      4. Fail any skill whose score is based only on partial smoke evidence.
    Expected: All 9 skills have request-level outcomes, not just feature checkmarks.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md

  Scenario: Full function coverage explains request outcomes
    Channel: bash
    Steps:
      1. Grep each target skill for `##` and `###` headings.
      2. Compare headings to request evidence rows.
      3. Fail any skill with an uncovered phase, step, operation, integration path, or stop rule that is required for an advertised request.
    Expected: All function headings are mapped to request evidence or explicitly justified as not exercised by the chosen requests.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md

  Scenario: Executable snippets are contract-checked
    Channel: bash
    Steps:
      1. Extract executable snippets and tool names from all target skills.
      2. Run targeted `--help`, local temp fixtures, or documented dry-run equivalents.
      3. Classify stale or unsafe snippets separately from working snippets.
    Expected: No executable instruction is accepted by inspection alone.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md
  ```

  **Commit**: NO

- [ ] 1. Build a command and tool contract inventory for all pioneer skills

  **What to do**: Extract every `agent-harness ...`, MCP tool name, host tool name, dependency install command, and dangerous git/shell command from the 9 target skills. Classify each as `valid CLI`, `valid MCP`, `host-specific`, `stale`, `dangerous-default`, or `reference-only`.
  **Must NOT do**: Do not edit skills in this task. It is an inventory baseline.
  **Why this improves quality**: It directly targets `stale-contract` failures that prevent `hopper`, `turing`, `von-neumann`, `karpathy`, and `berners-lee` from completing operational requests.

  **Recommended Agent**: quick
    Reason: Read-only grep plus current CLI/MCP comparison.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2, T3, T8, T10 | Blocked By: T0

  **References**:
  - Current CLI usage: `internal/adapter/cli/usage.go:47-111`
  - Project lint diagnose parser: `cmd/harness/projectcli/project_lint_diagnose.go:10-24`
  - IssueOps CLI support: `cmd/harness/issueopscli/issueops_cli_support.go:11-39`
  - MCP `lint_diagnose`: `internal/adapter/mcp/local_assistant_catalog.go:19-30`

  **Acceptance Criteria**:
  - [ ] Inventory file exists at `.agent-harness/evidence/pioneer-skills-quality/task-1-contract-inventory.txt`
  - [ ] Inventory includes at least these stale entries: `--command-argv`, `issueops heartbeat`, `von-neumann plan`, `remove-ai-slops`, `state write <key> <content>`
  - [ ] Inventory separately labels CLI and MCP forms for `lint_diagnose`

  **QA Scenarios**:
  ```
  Scenario: Inventory detects known stale snippets
    Channel: bash
    Steps:
      1. Run rg over target SKILL.md files for command/tool snippets.
      2. Compare against `./bin/agent-harness --help` and targeted `--help` commands.
      3. Save classified output.
    Expected: Known stale snippets are listed with file:line evidence.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-1-contract-inventory.txt

  Scenario: Inventory avoids false positive for MCP lint_diagnose
    Channel: bash
    Steps: Check `internal/adapter/mcp/local_assistant_catalog.go:19-30`.
    Expected: `lint_diagnose(command_argv: ...)` is classified as valid MCP, while CLI `--command-argv` is stale.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-1-contract-inventory.txt
  ```

  **Commit**: NO

- [ ] 2. Fix high-severity CLI and lifecycle drift in `hopper`, `turing`, and `von-neumann`

  **What to do**: Replace stale command snippets with current forms. For `hopper`, change CLI usage to `agent-harness project lint-diagnose --json -- go test ...` and keep `lint_diagnose(command_argv: ...)` as MCP-only. For `turing`, replace `issueops heartbeat` with `issueops feedback add` or remove heartbeat, replace `agent-harness state write <key> <content>` with `state write --key KEY --value TEXT`, and replace `remove-ai-slops` with the current IssueOps `ai-slop-clean` reference or `skills/issueops/references/ai-slop-clean.md`. For `von-neumann`, remove or reframe `agent-harness von-neumann plan --json` until a real command exists.
  **Must NOT do**: Do not add new CLI commands just to satisfy stale skill prose.
  **Why this improves quality**: It turns the most severe request failures from immediate command errors into executable workflows.

  **Recommended Agent**: quick
    Reason: Small targeted documentation edits with deterministic command validation.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T8, T10 | Blocked By: T1

  **References**:
  - Stale `hopper` snippet: `skills/hopper/SKILL.md:71-77`
  - Actual CLI parser: `cmd/harness/projectcli/project_lint_diagnose.go:10-24`
  - Stale `turing` snippets: `skills/turing/SKILL.md:290-292`, `skills/turing/SKILL.md:337-343`, `skills/turing/SKILL.md:357`
  - Stale `von-neumann` snippet: `skills/von-neumann/SKILL.md:86-90`

  **Acceptance Criteria**:
  - [ ] `rg -n "command-argv|issueops heartbeat|von-neumann plan|remove-ai-slops|state write <key> <content>" skills/hopper/SKILL.md skills/turing/SKILL.md skills/von-neumann/SKILL.md` returns no stale instructional matches.
  - [ ] `./bin/agent-harness project lint-diagnose --help` supports the documented CLI form.
  - [ ] `./bin/agent-harness issueops phase --help` is the only phase-advance command referenced for IssueOps lifecycle transitions.

  **QA Scenarios**:
  ```
  Scenario: Correct lint-diagnose CLI form is documented
    Channel: bash
    Steps:
      1. Run `./bin/agent-harness project lint-diagnose --help`.
      2. Confirm `skills/hopper/SKILL.md` uses `-- <command_to_run...>`.
    Expected: Help output and skill snippet agree.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-2-cli-drift.txt

  Scenario: Removed nonexistent lifecycle commands
    Channel: bash
    Steps: Run `rg -n "issueops heartbeat|von-neumann plan|remove-ai-slops" skills`.
    Expected: No instructional references remain in pioneer skills.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-2-cli-drift.txt
  ```

  **Commit**: YES | Message: `docs(skills): align pioneer command snippets with harness cli`

- [ ] 3. Define a shared pioneer skill authoring contract

  **What to do**: Add a concise contract, preferably `.agent-harness/SKILL_QUALITY.md` or a section in `.agent-harness/CONVENTIONS.md`, covering: activation boundary, allowed mutations, command snippet validation, host-tool translation, unsafe dependency installs, sub-agent use, evidence requirements, and reference-file extraction thresholds.
  **Must NOT do**: Do not create a second source of truth that conflicts with existing skill docs. The contract should route maintainers to current project rules.
  **Why this improves quality**: It prevents the exact regression class found by T0: validator-green skills that still fail realistic requests.

  **Recommended Agent**: deep
    Reason: Cross-cutting policy document that will govern all 9 skill edits.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T4, T5, T6, T7 | Blocked By: T1

  **References**:
  - Instruction priority: `.agent-harness/CONSTITUTION.md:1-80`
  - Sub-agent principle: `.agent-harness/CONSTITUTION.md:113-123`
  - Existing testing conventions: `.agent-harness/TESTING.md`
  - Skill validation command: `python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py <skill>`

  **Acceptance Criteria**:
  - [ ] Contract file or section exists and is referenced from at least one project doc.
  - [ ] Contract explicitly says executable command snippets must be smoke-tested or labeled as pseudocode.
  - [ ] Contract bans unqualified global installs and host-specific tool names without fallback.
  - [ ] Contract sets a core `SKILL.md` size guideline and reference extraction rule.

  **QA Scenarios**:
  ```
  Scenario: Contract covers the defects found in this audit
    Channel: bash
    Steps: Grep the new contract for `command snippet`, `host`, `install`, `sub-agent`, and `reference`.
    Expected: Every keyword appears in a prescriptive rule.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-3-authoring-contract.txt

  Scenario: Contract does not conflict with existing priority rules
    Channel: bash
    Steps: Grep `.agent-harness/CONSTITUTION.md` and the new contract for instruction priority statements.
    Expected: New contract defers to existing priority order.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-3-authoring-contract.txt
  ```

  **Commit**: YES | Message: `docs(skills): add pioneer skill quality contract`

- [ ] 4. Rebalance activation and mode boundaries for `turing` and `von-neumann`

  **What to do**: Narrow `von-neumann` so it activates for explicit planning, architecture, ambiguous multi-step tasks, or user-requested plan artifacts, but does not convert every "do/fix/build" request into planning-only mode. Narrow `turing` so the full evidence loop is used for explicit verified delivery, high-risk goals, or durable-goal execution, while ordinary small fixes use proportionate verification.
  **Must NOT do**: Do not remove their distinctive planning/execution roles. Do not weaken evidence requirements for high-risk or explicit Turing tasks.
  **Why this improves quality**: It improves request fit by preventing overbroad workflows from blocking ordinary user intent.

  **Recommended Agent**: deep
    Reason: Requires careful rewrite of high-priority behavior instructions.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T10 | Blocked By: T3

  **References**:
  - Over-broad `von-neumann` identity: `skills/von-neumann/SKILL.md:13-16`, `skills/von-neumann/SKILL.md:427-449`
  - Heavy `turing` loop and reviewer gate: `skills/turing/SKILL.md:190-298`, `skills/turing/SKILL.md:365-376`
  - Project default execution principle: `.agent-harness/AGENT_WORKFLOW.md`

  **Acceptance Criteria**:
  - [ ] `von-neumann` no longer says "When the user says do/fix/build, interpret as plan. No exceptions."
  - [ ] `turing` distinguishes full evidence loop from proportionate small-task verification.
  - [ ] Both skills preserve explicit high-rigor paths when named by the user.

  **QA Scenarios**:
  ```
  Scenario: Small implementation request does not get planning-only hijacked
    Channel: bash
    Steps: Inspect `skills/von-neumann/SKILL.md` activation text and critical rules.
    Expected: It routes explicit implementation requests back to the normal executor unless a plan is requested or ambiguity/risk requires planning.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-4-activation.txt

  Scenario: Explicit Turing request still requires evidence
    Channel: bash
    Steps: Inspect `skills/turing/SKILL.md` bootstrap and stop rules.
    Expected: Named `$turing` or verified-delivery requests still require criteria, evidence, cleanup, and final audit.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-4-activation.txt
  ```

  **Commit**: YES | Message: `docs(skills): rebalance planning and evidence activation`

- [ ] 5. Harden safety boundaries in `berners-lee`, `shannon`, `torvalds`, and `codd`

  **What to do**: Remove unqualified auto-install instructions from `berners-lee` and `shannon`; replace with "check availability, ask or use project-local temp env" language. In `berners-lee`, clarify that fetch resilience must not bypass auth, paywalls, robots-sensitive restrictions, or site abuse controls. In `torvalds`, make destructive recovery commands a last-resort, explicitly user-confirmed path with backup verification. In `codd`, keep DDL recommendations advisory and require environment/transaction-lock checks before live migration advice.
  **Must NOT do**: Do not remove useful examples; convert risky commands into gated examples.
  **Why this improves quality**: It converts `unsafe` request outcomes into safe stop, ask, or sandboxed execution paths.

  **Recommended Agent**: deep
    Reason: Multi-skill safety rewrite with project policy implications.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T10 | Blocked By: T3

  **References**:
  - `berners-lee` auto-install and fetch escalation: `skills/berners-lee/SKILL.md:83-190`, `skills/berners-lee/SKILL.md:238-247`
  - `shannon` baseline and install snippets: `skills/shannon/SKILL.md:167-218`, `skills/shannon/SKILL.md:302-338`
  - `torvalds` destructive recovery example: `skills/torvalds/SKILL.md:75-94`
  - `codd` DDL safety rules: `skills/codd/SKILL.md:679-685`, `skills/codd/SKILL.md:942-965`

  **Acceptance Criteria**:
  - [ ] No target skill contains `pip install`, `go install`, or equivalent global install as an unconditional instruction.
  - [ ] `berners-lee` retains the login/paywall stop rule and adds explicit "do not bypass access control" language.
  - [ ] `torvalds` documents backup verification before any `reset --hard` recovery.
  - [ ] `codd` keeps live DDL behind evidence and migration-safety gates.

  **QA Scenarios**:
  ```
  Scenario: No unconditional global installs remain
    Channel: bash
    Steps: Run `rg -n "pip install|go install|npm install -g|brew install" skills/berners-lee/SKILL.md skills/shannon/SKILL.md`.
    Expected: Matches are absent or clearly inside gated optional setup text.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-5-safety.txt

  Scenario: Destructive git commands remain guarded
    Channel: bash
    Steps: Run `rg -n "reset --hard|clean -fd|force" skills/torvalds/SKILL.md`.
    Expected: Every match is adjacent to backup, verification, or explicit-user-confirmation language.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-5-safety.txt
  ```

  **Commit**: YES | Message: `docs(skills): harden pioneer safety boundaries`

- [ ] 6. Make `karpathy` prompt guidance compatible with modern reasoning privacy and real host tools

  **What to do**: Replace explicit chain-of-thought output instructions with private reasoning plus concise rationale/final-check summaries. Replace fake `search_codebase`/`read_file` tool schemas with host-neutral guidance: use the current available tools, cite exact tool names only in host-specific references, and validate tool arguments before use.
  **Must NOT do**: Do not remove adversarial testing or prompt fixture requirements.
  **Why this improves quality**: It makes prompt requests complete under modern reasoning privacy and avoids teaching agents unavailable tool APIs.

  **Recommended Agent**: quick
    Reason: One skill plus prompt-pattern cleanup.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T10 | Blocked By: T3

  **References**:
  - Chain-of-thought references: `skills/karpathy/SKILL.md:79`, `skills/karpathy/SKILL.md:117-125`, `skills/karpathy/SKILL.md:195-210`, `skills/karpathy/SKILL.md:320-337`
  - Fake tool schemas: `skills/karpathy/SKILL.md:362-385`

  **Acceptance Criteria**:
  - [ ] `rg -n "Chain-of-Thought|Show your work|State each step" skills/karpathy/SKILL.md` returns no instruction to reveal hidden reasoning.
  - [ ] Prompt patterns ask for answer, concise rationale, and verification summary.
  - [ ] Tool-use pattern avoids fictional tool names unless explicitly marked as example schema.

  **QA Scenarios**:
  ```
  Scenario: Reasoning privacy check
    Channel: bash
    Steps: Grep for chain-of-thought and show-your-work phrases.
    Expected: No user-facing chain-of-thought instruction remains.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-6-karpathy.txt

  Scenario: Tool schema portability check
    Channel: bash
    Steps: Grep for `search_codebase` and `read_file`.
    Expected: If present, they are labeled as illustrative examples, not required host tools.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-6-karpathy.txt
  ```

  **Commit**: YES | Message: `docs(karpathy): align prompt patterns with reasoning privacy`

- [ ] 7. Reduce core size and improve progressive disclosure for dense skills

  **What to do**: Move bulky reference material out of `codd`, `shannon`, and optionally `dijkstra` into `references/` files. Keep each `SKILL.md` focused on activation, role, method, critical rules, stop rules, and references to optional material. `codd` should split at least engine-specific catalog queries, lock tables, and connection pool formulas into references. `shannon` should split shell heuristics and AST-tool recipes into references.
  **Must NOT do**: Do not delete domain content. Preserve it as referenced material.
  **Why this improves quality**: It helps agents complete the actual request path without drowning in unrelated examples or engine-specific branches.

  **Recommended Agent**: deep
    Reason: Multi-file documentation refactor with cross-reference integrity.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T8, T10 | Blocked By: T3

  **References**:
  - Current size: `wc -l` showed `skills/codd/SKILL.md` has 989 lines, `dijkstra` 509, `karpathy` 443, `turing` 403.
  - Existing reference-file pattern: `skills/turing/references/evidence-contract.md`, `skills/von-neumann/references/clearance-checklist.md`, `skills/torvalds/references/*.md`

  **Acceptance Criteria**:
  - [ ] `skills/codd/SKILL.md` core size is materially reduced while `quick_validate.py skills/codd` still passes.
  - [ ] New references are linked from `SKILL.md` with clear "when to open" instructions.
  - [ ] No orphaned references: every new `references/*.md` is mentioned by its parent `SKILL.md`.

  **QA Scenarios**:
  ```
  Scenario: Codd progressive disclosure
    Channel: bash
    Steps:
      1. Run `wc -l skills/codd/SKILL.md`.
      2. Run `find skills/codd/references -type f | sort`.
      3. For each reference, grep its filename in `skills/codd/SKILL.md`.
    Expected: Codd core is smaller and every reference is reachable.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-7-progressive-disclosure.txt

  Scenario: Skill validation after splits
    Channel: bash
    Steps: Run `python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/codd`.
    Expected: Validation passes.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-7-progressive-disclosure.txt
  ```

  **Commit**: YES | Message: `docs(codd): move dense database references out of core skill`

- [ ] 8. Add qualitative fixture suite for all 9 pioneer skills

  **What to do**: Create a fixture file such as `testdata/skills/pioneer_quality_cases.yaml` or `.agent-harness/operations/pioneer-skill-quality-cases.md`. Seed it from the 27 T0 request cases: happy path, boundary/safety path, and integration/operational path for each of the 9 pioneer skills. Include expected behavior, expected non-behavior, required evidence, and prohibited shortcuts.
  **Must NOT do**: Do not build a full LLM judge in this task. Fixtures should be human-readable and later automatable.
  **Why this improves quality**: It makes request completion the durable regression target instead of letting future reviews fall back to static prose checks.

  **Recommended Agent**: deep
    Reason: Cross-skill quality design with future testability.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10 | Blocked By: T2, T4, T5, T6, T7

  **References**:
  - `agents/openai.yaml` default prompts for all 9 target skills.
  - Current skill descriptions at each target `SKILL.md:2-3`.

  **Acceptance Criteria**:
  - [ ] Fixture includes all 9 target skill names.
  - [ ] Each skill has at least 2 cases: activation and safety/non-activation.
  - [ ] Each case has binary expected behavior and an evidence requirement.
  - [ ] Cases cover known audit risks: command drift, unsafe install, over-planning, chain-of-thought, destructive git, and heuristic metrics.

  **QA Scenarios**:
  ```
  Scenario: Fixture coverage
    Channel: bash
    Steps: Grep fixture for all 9 skill names and count cases.
    Expected: All names present and at least 18 cases total.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-8-fixtures.txt

  Scenario: Known risk coverage
    Channel: bash
    Steps: Grep fixture for `command drift`, `install`, `planning`, `reasoning`, `reset --hard`, and `heuristic`.
    Expected: Each known risk appears in at least one case.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-8-fixtures.txt
  ```

  **Commit**: YES | Message: `test(skills): add pioneer qualitative quality fixtures`

- [ ] 9. Sync adapters and public docs after skill edits

  **What to do**: Ensure `agents/openai.yaml`, README skill table, and relevant project docs still match the revised activation boundaries and role descriptions. Update only changed facts.
  **Must NOT do**: Do not rewrite marketing prose or unrelated docs.

  **Recommended Agent**: quick
    Reason: Small docs consistency pass after skill edits.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10 | Blocked By: T4, T5, T6, T7

  **References**:
  - `skills/*/agents/openai.yaml`
  - `README.md` skill overview
  - `.agent-harness/ARCHITECTURE.md` and `.agent-harness/TECH_STACK.md` if they mention pioneer skills.

  **Acceptance Criteria**:
  - [ ] Adapter default prompts do not overstate removed behaviors.
  - [ ] README/project docs do not mention nonexistent commands or obsolete skill names.
  - [ ] `rg -n "remove-ai-slops|von-neumann plan|issueops heartbeat|--command-argv" README.md .agent-harness skills` has no stale instructional matches.

  **QA Scenarios**:
  ```
  Scenario: Public docs drift check
    Channel: bash
    Steps: Run stale-reference grep across README, `.agent-harness`, and `skills`.
    Expected: No stale instructional references remain.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-9-doc-sync.txt

  Scenario: Adapter prompt role check
    Channel: bash
    Steps: Read all 9 `agents/openai.yaml` files and compare with revised activation text.
    Expected: Short descriptions and prompts match the revised role boundaries.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-9-doc-sync.txt
  ```

  **Commit**: YES | Message: `docs(skills): sync pioneer adapter prompts`

- [ ] 10. Final validation and completion audit

  **What to do**: Run structural validation, stale-reference checks, focused command smokes, and relevant Go tests. Record final evidence and produce a completion audit mapping every task to evidence.
  **Must NOT do**: Do not claim completion from green `quick_validate` alone.

  **Recommended Agent**: quick
    Reason: Verification-only final gate.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: - | Blocked By: T8, T9

  **References**:
  - `.agent-harness/TESTING.md`
  - All task evidence files under `.agent-harness/evidence/pioneer-skills-quality/`

  **Acceptance Criteria**:
  - [ ] `quick_validate.py` passes for all 9 target skills.
  - [ ] `rg -n "command-argv|issueops heartbeat|von-neumann plan|remove-ai-slops|state write <key> <content>" skills README.md .agent-harness` has no stale instructional matches.
  - [ ] `./bin/agent-harness project lint-diagnose --help`, `./bin/agent-harness issueops phase --help`, and state write/read smoke all pass.
  - [ ] `go test ./... -count=1` passes, unless only documentation changed and the user explicitly accepts a narrower gate.
  - [ ] `git diff --stat` contains only skill/doc/test fixture files expected by this plan.

  **QA Scenarios**:
  ```
  Scenario: Full structural validation
    Channel: bash
    Steps:
      1. Run quick_validate for all 9 target skills.
      2. Run stale-reference grep.
      3. Run targeted CLI help/smoke commands.
      4. Run `go test ./... -count=1`.
    Expected: All pass, or any skipped command has a concrete documented reason.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-10-final-validation.txt

  Scenario: Scope fidelity
    Channel: bash
    Steps: Run `git diff --stat` and compare changed files to this plan.
    Expected: No unrelated source or generated files changed.
    Evidence: .agent-harness/evidence/pioneer-skills-quality/task-10-final-validation.txt
  ```

  **Commit**: NO

## Final Verification Wave
- [ ] F1. Plan Compliance Audit - every TODO executed or explicitly deferred with evidence.
- [ ] F2. Skill Quality Review - no stale command snippets, unsafe defaults, fake tools, or over-broad activation rules remain.
- [ ] F3. Structural Validation - all target skills pass `quick_validate.py`.
- [ ] F4. Runtime Contract Smoke - documented CLI snippets match `./bin/agent-harness` behavior.
- [ ] F5. Scope Fidelity Check - only planned skill/doc/test fixture files changed.

## Commit Strategy
Recommended atomic commits:
1. `docs(skills): align pioneer command snippets with harness cli`
2. `docs(skills): add pioneer skill quality contract`
3. `docs(skills): rebalance planning and evidence activation`
4. `docs(skills): harden pioneer safety boundaries`
5. `docs(karpathy): align prompt patterns with reasoning privacy`
6. `docs(codd): move dense database references out of core skill`
7. `test(skills): add pioneer qualitative quality fixtures`
8. `docs(skills): sync pioneer adapter prompts`

## Success Criteria
- The pioneer skill family remains distinctive and useful, but no longer instructs agents to run stale commands, expose hidden reasoning, install dependencies globally by default, or over-trigger heavyweight workflows.
- Future contributors have a clear quality contract and qualitative fixture suite to prevent drift.
- Completion is proven by current file evidence, CLI smoke output, validator output, and tests, not by prose confidence.
