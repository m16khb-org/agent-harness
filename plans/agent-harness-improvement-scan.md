# Agent Harness Improvement Scan

## TL;DR
> **Summary**: Current checkout has one verified blocking regression and several high-value hardening lanes. First fix the stale response-contract golden/test drift, then tighten command surface consistency, docs/code drift, draft-wiki queue safety, worker MCP parity decisions, and opt-in CodeGraph-first source-search enforcement, and companion-tool hook timing for claude-mem/LLM Wiki.
> **Deliverables**: ranked fix backlog, tests-first tasks, CLI/tmux QA scenarios, docs/contract verification.
> **Effort**: Medium
> **Parallel**: YES - 3 waves
> **Critical Path**: Task 1 -> Task 2 -> Task 5 -> Final Verification

## Context
### Original Request
- "현재 agent-harness에서 개선되어야 할 점이나 보완되어야 할 점을 수색"
- Explicit skill triggers: `omo:start-work`, `omo:ulw-plan`.

### Research Summary
- `go test ./... -count=1` fails in `cmd/harness` at `cmd/harness/response_contract_golden_test.go:200` because `response_contracts.golden.json` is stale relative to current response contracts.
- `self-verify --iterations=10 --seed=100 --target-score=95 --json` fails immediately because the same `go test` gate fails.
- `README.md:135` documents `agent-harness update`, and `cmd/harness/main.go:137` dispatches it, but `internal/adapter/cli/usage.go:43` usage text omits it.
- `AGENTS.md:117` still says the repo has no application code, while `README.md:24` and the actual tree show a functional Go CLI/backend.
- `.agent-harness/ARCHITECTURE.md:154` says command execution is only policy check + fake runner, but `internal/core/policy.go:298` implements real argv-only read-only execution.
- `internal/core/draft_wiki_queue.go:119` reads and rewrites the draft-wiki queue without visible locking around processing.
- Worker CLI exposes `worker run --read-only` (`cmd/harness/worker.go:95`), but MCP adapter worker tools stop at enqueue/status/list/cancel (`internal/adapter/mcp/catalog.go:25`).
- PreToolUse is currently non-blocking and emits `{}` for host output (`cmd/harness/hook_user_prompt.go:133`, `cmd/harness/hook_user_prompt.go:158`), while raw JSON shows `decision: allow`; docs warn blocking requires deterministic policy and host-schema tests (`.agent-harness/AGENT_WORKFLOW.md:45`, `.agent-harness/CAUTIONS.md:167`).

### Metis Review (gaps addressed)
- Resolved mode conflict by treating this turn as discovery + plan artifact only, not source implementation.
- Brownfield assumption: this is an active Go CLI/backend repo, not an initial docs-only repo.
- Mutation boundary: source files and generated golden files are not changed in this plan-generation pass.

## Work Objectives
### Core Objective
Make agent-harness green and reduce operator confusion by aligning tests, command surfaces, docs, and worker/draft-wiki operational contracts.

### Definition of Done
- `go test ./... -count=1` passes.
- `./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json` passes or any remaining failure is a new, named issue with evidence.
- `agent-harness --help`/usage, README, CLI dispatch, and contract snapshots agree on public commands.
- Worker read-only execution parity is intentionally documented or exposed through MCP with matching tests.
- Draft-wiki queue processing has a tested concurrency/atomicity story or a documented single-worker lock limitation.
- Optional CodeGraph-first enforcement blocks raw source-code grep/find searches only when a deterministic PreToolUse policy is explicitly enabled.
- claude-mem and LLM Wiki hints already appear in UserPromptSubmit keyword routing (`internal/core/hook_prompt.go:85`, `internal/core/hook_prompt.go:88`); PostToolUse currently queues draft-wiki candidates from tool output (`cmd/harness/hook_user_prompt.go:192`) and must not invoke companion tools on the hook critical path.

### Must NOT Have
- No writable shell worker expansion without explicit safety design.
- No real user-home install mutation during tests; use temp HOME/CODEX_HOME/HARNESS_STATE_DIR or dry-run.
- No generated golden update without explaining the intended contract delta.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD for production changes; characterization tests first for contract/doc-surface drift.
- QA policy: Every task includes a CLI/tmux real-surface scenario.
- Evidence directory: `evidence/agent-harness-improvement-scan/`.

## Execution Strategy
### Parallel Execution Waves
Wave 1: Task 1 only, because failing tests block trusted baseline.
Wave 2: Tasks 2, 3, and 4 in parallel after baseline is green.
Wave 3: Tasks 5, 6, and 7 after command/docs drift is resolved.
Wave 4: Final verification.

### Dependency Matrix
- Task 1 blocks all other tasks.
- Task 2 blocks final command contract verification.
- Task 3 blocks documentation fidelity completion.
- Task 4 informs Task 6 safety wording but can be separate.
- Task 5 depends on Task 1 only.
- Task 6 depends on Tasks 3 and 4.
- Task 7 depends on Task 3 because hook blocking policy must update CAUTIONS/AGENT_WORKFLOW guidance.

## TODOs
- [ ] 1. Restore response-contract golden/test baseline

  **What to do**: Reproduce `TestResponseContractsGolden` failure, inspect the generated diff, decide whether current behavior or the golden is wrong, then either fix the behavior or update `cmd/harness/testdata/response_contracts.golden.json` with rationale.
  **Must NOT do**: Do not blindly update golden without naming the behavior delta.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: all tasks | Blocked By: none

  **References**:
  - Failure: `cmd/harness/response_contract_golden_test.go:200` - golden assertion.
  - Evidence command: `go test ./... -count=1` currently fails in `cmd/harness`.

  **Acceptance Criteria**:
  - [ ] A RED reproduction for `TestResponseContractsGolden` is captured before changes.
  - [ ] `go test ./cmd/harness -run TestResponseContractsGolden -count=1` passes after changes.
  - [ ] `go test ./... -count=1` passes after changes.

  **QA Scenarios**:
  ```
  Scenario: response contract check is green
    Tool: tmux
    Steps: tmux new-session -d -s qa-contract 'cd /Users/m16khb/Workspace/agent-harness && go test ./cmd/harness -run TestResponseContractsGolden -count=1; echo EXIT:$?'; capture pane
    Expected: transcript contains PASS and EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-1-contract.txt

  Scenario: stale golden fails before fix
    Tool: bash
    Steps: go test ./cmd/harness -run TestResponseContractsGolden -count=1 before implementation
    Expected: failure output includes response_contracts.golden.json mismatch
    Evidence: evidence/agent-harness-improvement-scan/task-1-red.txt
  ```

  **Commit**: YES | Message: `test(contract): restore response contract golden baseline` | Files: `cmd/harness/testdata/response_contracts.golden.json` or source fix files

- [ ] 2. Align `update` command across dispatch, help, README, and contracts

  **What to do**: Add or confirm `update` in canonical CLI usage/catalog and contract snapshots, matching `cmd/harness/main.go:137` and README guidance.
  **Must NOT do**: Do not change update semantics unless a failing characterization test proves a behavior bug.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: final verification | Blocked By: Task 1

  **References**:
  - Dispatch: `cmd/harness/main.go:137`.
  - README: `README.md:135`.
  - Missing usage: `internal/adapter/cli/usage.go:43` command usage block.

  **Acceptance Criteria**:
  - [ ] Characterization test proves `update` is dispatchable.
  - [ ] Usage/golden tests include `agent-harness update`.
  - [ ] README and help agree.

  **QA Scenarios**:
  ```
  Scenario: update appears in help
    Tool: tmux
    Steps: tmux new-session -d -s qa-update-help 'cd /Users/m16khb/Workspace/agent-harness && ./bin/agent-harness --help | grep "agent-harness update"; echo EXIT:$?'; capture pane
    Expected: transcript contains agent-harness update and EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-2-help.txt

  Scenario: dry-run update is safe
    Tool: bash
    Steps: HOME=$(mktemp -d) CODEX_HOME=$HOME/.codex ./bin/agent-harness update --dry-run --json
    Expected: JSON reports planned actions without writing real user home
    Evidence: evidence/agent-harness-improvement-scan/task-2-dry-run.json
  ```

  **Commit**: YES | Message: `fix(cli): include update in canonical command surface` | Files: `internal/adapter/cli/usage.go`, golden files, tests

- [ ] 3. Refresh stale project docs that still describe the repo as initial or fake-run only

  **What to do**: Update AGENTS/project docs so they match the current Go codebase, worker MVP, and read-only command execution reality.
  **Must NOT do**: Do not weaken safety warnings around writable shell execution.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: final verification | Blocked By: Task 1

  **References**:
  - Stale initial-state claim: `AGENTS.md:117`.
  - Current status: `README.md:24`.
  - Stale fake-run-only architecture: `.agent-harness/ARCHITECTURE.md:154`.
  - Real read-only execution: `internal/core/policy.go:298`.

  **Acceptance Criteria**:
  - [ ] Docs no longer claim the repo has no application code.
  - [ ] Command policy docs distinguish fake-run from `policy run --read-only`.
  - [ ] `./bin/agent-harness docs --json` includes updated docs without parse errors.

  **QA Scenarios**:
  ```
  Scenario: docs index includes updated source-of-truth text
    Tool: tmux
    Steps: tmux new-session -d -s qa-docs 'cd /Users/m16khb/Workspace/agent-harness && ./bin/agent-harness docs --json | python3 -m json.tool >/tmp/agent-harness-docs.json; grep -q ARCHITECTURE /tmp/agent-harness-docs.json; echo EXIT:$?'; capture pane
    Expected: EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-3-docs.txt

  Scenario: no stale initial-code claim remains
    Tool: bash
    Steps: rg "아직 애플리케이션 코드가 없는|policy check \+ fake runner" AGENTS.md .agent-harness README.md
    Expected: no stale false claim remains, or remaining mention is explicitly historical/qualified
    Evidence: evidence/agent-harness-improvement-scan/task-3-stale-scan.txt
  ```

  **Commit**: YES | Message: `docs(project): align operating docs with current harness implementation` | Files: `AGENTS.md`, `.agent-harness/ARCHITECTURE.md`, related docs

- [ ] 4. Decide and test worker CLI/MCP parity for read-only execution

  **What to do**: Decide whether `worker run --read-only` should remain CLI-only or gain MCP parity. If CLI-only, document the reason and add a contract warning. If MCP parity is desired, add a bounded `worker_run_read_only` MCP tool with tests.
  **Must NOT do**: Do not add writable shell execution.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: Task 6 wording and final verification | Blocked By: Task 1

  **References**:
  - CLI read-only worker: `cmd/harness/worker.go:95`.
  - Core read-only worker: `internal/core/worker.go:143`.
  - MCP worker tools currently omit run: `internal/adapter/mcp/catalog.go:25`.

  **Acceptance Criteria**:
  - [ ] Decision is captured in docs or ADR.
  - [ ] Contract/golden tests prove the chosen surface.
  - [ ] Worker command behavior remains policy-gated and read-only.

  **QA Scenarios**:
  ```
  Scenario: worker read-only command through chosen public surface
    Tool: tmux
    Steps: tmux new-session -d -s qa-worker 'cd /Users/m16khb/Workspace/agent-harness && tmp=$(mktemp -d) && HARNESS_WORKER_DIR=$tmp ./bin/agent-harness worker run --read-only --kind qa --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short; code=$?; rm -rf "$tmp"; echo EXIT:$code'; capture pane
    Expected: JSON has status succeeded and EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-4-worker.txt

  Scenario: writable worker command remains denied
    Tool: bash
    Steps: HARNESS_WORKER_DIR=$(mktemp -d) ./bin/agent-harness worker run --read-only --kind qa --workspace-root "$PWD" --cwd "$PWD" --json -- touch should-not-exist
    Expected: non-zero exit, policy deny reason for write command, file absent
    Evidence: evidence/agent-harness-improvement-scan/task-4-worker-deny.txt
  ```

  **Commit**: YES | Message: `fix(worker): clarify read-only worker public surface` | Files: `internal/adapter/mcp/catalog.go`, `cmd/harness/main.go`, docs/tests as chosen

- [ ] 5. Harden draft-wiki queue processing against concurrent workers and stale/racy queue state

  **What to do**: Add a test that simulates concurrent queue processing or interrupted processing, then implement locking/atomic rewrite or an explicit single-worker guard.
  **Must NOT do**: Do not run real `agy` in tests; use a fake executable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final verification | Blocked By: Task 1

  **References**:
  - Queue load/rewrite loop: `internal/core/draft_wiki_queue.go:119`.
  - Running marker write ignored: `internal/core/draft_wiki_queue.go:150`.
  - External command execution: `internal/core/draft_wiki_queue.go:206`.

  **Acceptance Criteria**:
  - [ ] Failing concurrency/stale-state test exists first.
  - [ ] Parallel workers do not process the same queued item twice.
  - [ ] Corrupt or partial queue entries produce bounded errors without losing valid entries.

  **QA Scenarios**:
  ```
  Scenario: two workers race on one queued draft
    Tool: tmux
    Steps: create temp repo/state and fake agy, enqueue one event, run two `worker draft-wiki --limit 1 --json` panes concurrently, capture both panes
    Expected: exactly one succeeded event and no duplicate draft output
    Evidence: evidence/agent-harness-improvement-scan/task-5-race.txt

  Scenario: malformed queue line is reported safely
    Tool: bash
    Steps: write one invalid JSONL line plus one valid queued event, run worker draft-wiki with fake agy
    Expected: error identifies malformed line without secret leakage; valid event handling is defined by test
    Evidence: evidence/agent-harness-improvement-scan/task-5-malformed.txt
  ```

  **Commit**: YES | Message: `fix(draft-wiki): serialize queue processing safely` | Files: `internal/core/draft_wiki_queue.go`, tests

- [ ] 6. Add a small command-surface consistency guard

  **What to do**: Add a test or helper that compares dispatchable top-level commands, adapter `Commands()`, canonical usage text, and response contract command list.
  **Must NOT do**: Do not create a broad reflection framework; keep the check explicit and small.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final verification | Blocked By: Tasks 2-4

  **References**:
  - Dispatch table: `cmd/harness/main.go`.
  - Adapter command list: `internal/adapter/cli/usage.go:14`.
  - Response contract failure area: `cmd/harness/response_contract_golden_test.go:200`.

  **Acceptance Criteria**:
  - [ ] Test fails if a top-level command is added to dispatch but omitted from usage/catalog.
  - [ ] Test covers `update` specifically.

  **QA Scenarios**:
  ```
  Scenario: command consistency test passes
    Tool: tmux
    Steps: tmux new-session -d -s qa-command-surface 'cd /Users/m16khb/Workspace/agent-harness && go test ./cmd/harness ./internal/adapter/cli -run "Command|Usage|Golden" -count=1; echo EXIT:$?'; capture pane
    Expected: PASS and EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-6-command-surface.txt

  Scenario: help and contract mention same command set
    Tool: bash
    Steps: ./bin/agent-harness contract schema --json and ./bin/agent-harness --help, compare top-level command names with a small script
    Expected: no missing command names
    Evidence: evidence/agent-harness-improvement-scan/task-6-contract-help.txt
  ```

  **Commit**: YES | Message: `test(cli): guard top-level command surface consistency` | Files: tests/helpers/golden as needed


- [ ] 7. Enforce CodeGraph-first source search via opt-in PreToolUse policy

  **What to do**: Add an explicit opt-in mode for `agent-harness hook pre-tool-use` that blocks raw source-code search commands (`rg`, `grep`, `git grep`, broad `find` over source extensions) and tells the agent to use CodeGraph first. Keep the default host output non-blocking unless the install/config explicitly enables this policy.
  **Must NOT do**: Do not block exact text searches in docs/golden/test output, and do not require CodeGraph for cases where CodeGraph is not the right tool (e.g. README grep, generated JSON diff inspection, literal error-message search after a failing test).

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final verification | Blocked By: Tasks 1 and 3

  **References**:
  - Current PreToolUse no-op host output: `cmd/harness/hook_user_prompt.go:133` and `cmd/harness/hook_user_prompt.go:158`.
  - Raw allow decision: `internal/core/lifecycle_state.go:413`.
  - Installed hooks already include PreToolUse: `configs/codex/hooks.json`, `configs/claude/hooks.settings.json`.
  - Existing caution against premature blocking: `.agent-harness/CAUTIONS.md:167`.
  - CodeGraph routing hint already exists for symbol/call-impact lookup: `internal/core/hook_prompt.go` CodeGraph hint branch.

  **Acceptance Criteria**:
  - [ ] Default `agent-harness hook pre-tool-use` still emits `{}` and does not block normal tool use.
  - [ ] Opt-in policy blocks a Bash payload like `rg -n "func Run" cmd internal` and returns a host-compatible block response with a reason that names `codegraph_context`, `codegraph_search`, or `codegraph_trace`.
  - [ ] Opt-in policy allows docs/golden/literal evidence searches such as `rg "response_contracts" cmd/harness/testdata README.md`.
  - [ ] Host schema tests cover Codex and Claude behavior before any install template enables blocking.

  **QA Scenarios**:
  ```
  Scenario: source grep is blocked when CodeGraph enforcement is enabled
    Tool: tmux
    Steps: tmux new-session -d -s qa-codegraph-block 'cd /Users/m16khb/Workspace/agent-harness && printf "{\"cwd\":\"$PWD\",\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"rg -n RunReadOnlyCommand internal cmd\"}}" | ./bin/agent-harness hook pre-tool-use --enforce-codegraph-search --json; echo EXIT:$?'; capture pane
    Expected: JSON decision is block/deny and reason tells the agent to use CodeGraph first
    Evidence: evidence/agent-harness-improvement-scan/task-7-codegraph-block.txt

  Scenario: docs/golden literal grep remains allowed
    Tool: bash
    Steps: printf hook JSON for `rg "response_contracts" cmd/harness/testdata README.md` into `./bin/agent-harness hook pre-tool-use --enforce-codegraph-search --json`
    Expected: JSON decision is allow
    Evidence: evidence/agent-harness-improvement-scan/task-7-codegraph-allow.txt
  ```

  **Commit**: YES | Message: `feat(hook): add opt-in CodeGraph-first source search gate` | Files: `cmd/harness/hook_user_prompt.go`, `internal/core/lifecycle_state.go`, hook tests, install templates/docs only if enabling is requested


- [ ] 8. Clarify companion-tool hook timing for claude-mem and LLM Wiki recommendations

  **What to do**: Keep companion-tool guidance as lightweight recommendations, not automatic execution. Use UserPromptSubmit for prompt-triggered hints, SessionStart/PostCompact for stable installed-tool availability/status if needed, and PostToolUse only for bounded draft-wiki queue capture. Document that PreToolUse and Stop are poor places for claude-mem/LLM Wiki recommendations because they are critical-path/control-schema hooks.
  **Must NOT do**: Do not call `claude-mem`, `llm-wiki`, network tools, or LLM summarizers directly from lifecycle hooks. Do not inject broad memory/wiki reminders on every turn.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final verification | Blocked By: Task 3

  **References**:
  - Current claude-mem UserPromptSubmit hint: `internal/core/hook_prompt.go:88`.
  - Current LLM Wiki UserPromptSubmit hint: `internal/core/hook_prompt.go:85`.
  - Current compact rendering labels: `internal/core/hook_prompt.go:241`, `internal/core/hook_prompt.go:268`.
  - SessionStart/PostCompact catalog timing: `cmd/harness/hook_user_prompt.go:301`, `cmd/harness/hook_user_prompt.go:251`.
  - PostToolUse draft-wiki queue capture: `cmd/harness/hook_user_prompt.go:192`.
  - No-reimplementation policy: `.agent-harness/CAUTIONS.md:122`.

  **Acceptance Criteria**:
  - [ ] Prompt mentioning previous-session/repeated-work recall injects `claude-mem only for previous-session/repeated-work recall`.
  - [ ] Prompt mentioning explicit wiki/research/knowledge-base work injects `LLM Wiki for explicit wiki/research work`.
  - [ ] Generic prompts do not inject broad claude-mem/LLM Wiki reminders.
  - [ ] Docs state the recommended hook timing and explain why PreToolUse/Stop are excluded.
  - [ ] PostToolUse draft-wiki queue remains bounded/redacted and asynchronous.

  **QA Scenarios**:
  ```
  Scenario: claude-mem hint appears only for memory recall intent
    Tool: bash
    Steps: printf '{"prompt":"지난번에 이미 해결한 memory 찾아줘","cwd":"/Users/m16khb/Workspace/agent-harness"}' | ./bin/agent-harness hook user-prompt --json
    Expected: additional_context contains `claude-mem only for previous-session/repeated-work recall`
    Evidence: evidence/agent-harness-improvement-scan/task-8-claude-mem-hint.json

  Scenario: LLM Wiki hint appears only for explicit wiki/research intent
    Tool: bash
    Steps: printf '{"prompt":"wiki research knowledge base 정리해줘","cwd":"/Users/m16khb/Workspace/agent-harness"}' | ./bin/agent-harness hook user-prompt --json
    Expected: additional_context contains `LLM Wiki for explicit wiki/research work`
    Evidence: evidence/agent-harness-improvement-scan/task-8-llm-wiki-hint.json

  Scenario: generic code task does not over-recommend companion tools
    Tool: tmux
    Steps: tmux new-session -d -s qa-companion-generic 'cd /Users/m16khb/Workspace/agent-harness && printf "{\"prompt\":\"테스트 실패 고쳐줘\",\"cwd\":\"$PWD\"}" | ./bin/agent-harness hook user-prompt --json; echo EXIT:$?'; capture pane
    Expected: output does not contain claude-mem or LLM Wiki, and EXIT:0
    Evidence: evidence/agent-harness-improvement-scan/task-8-generic.txt
  ```

  **Commit**: YES | Message: `docs(hook): clarify companion tool recommendation timing` | Files: `internal/core/hook_prompt_test.go`, `.agent-harness/AGENT_WORKFLOW.md`, `.agent-harness/OPERATIONS.md`, `.agent-harness/CAUTIONS.md` if behavior/docs change

## Final Verification Wave
- [ ] F1. Plan Compliance Audit: ensure every task's acceptance criteria and QA artifacts exist.
- [ ] F2. Code Quality Review: run a reviewer against final diff with focus on safety, command execution, and docs drift.
- [ ] F3. Real Manual QA: run all tmux/bash scenarios above and record cleanup receipts.
- [ ] F4. Scope Fidelity Check: verify no user-home install mutation, no writable worker runner, and no unrelated refactors.

## Commit Strategy
- One commit per TODO unless a task is docs-only and tightly coupled to the adjacent source fix.
- Do not commit automatically without user approval.

## Success Criteria
- Failing baseline fixed.
- Public command surfaces are internally consistent.
- Docs reflect current implementation.
- Worker/draft-wiki safety gaps are either fixed or explicitly constrained with tests.
- CodeGraph-first enforcement is captured as an opt-in hook hardening task with false-positive safeguards.
- Companion-tool recommendation timing is explicit: UserPromptSubmit for per-turn hints, SessionStart/PostCompact for stable availability/status hints, PostToolUse only for bounded draft-wiki queueing, never PreToolUse/Stop for recommendations.
