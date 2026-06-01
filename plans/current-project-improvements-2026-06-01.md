# Agent-Harness Current Project Improvement Plan

## TL;DR
> **Summary**: Current agent-harness is a functional Go MVP with a green baseline. The next improvement work should prioritize public contract drift, safe read-only worker parity, draft-wiki queue correctness, runtime verification, then maintainability refactors.
> **Deliverables**: `update` command-surface consistency; MCP `worker_run_read_only`; draft-wiki queue locking/error contract; worker/daemon/race tests; installer deduplication; declarative hook routing; draft-wiki/LLM Wiki promotion QA.
> **Effort**: Large
> **Parallel**: YES - 5 waves
> **Critical Path**: Task 1 -> Task 2 -> Task 3 -> Task 4 -> Final Verification

## Context
### Original Request
- `$omo:ulw-plan 현재 프로젝트를 분석하고 개선 기능을 제안해줘`
- Planner-only output: propose executable improvement work without implementing source changes in this turn.

### Current-State Evidence
- `README.md:24` says the repo is an early but functional MVP with CLI, daemon-backed MCP, policy checker, read-only evidence runner, state checkpoints, project-doc/draft-wiki tools, lifecycle hooks, guard/verify-work/trace gates, API-doc review, native installer, self-verify, and self-augment.
- `go test ./... -count=1` passed on 2026-06-01 during planning across `cmd/harness`, `internal/adapter/*`, `internal/core`, and `internal/port`.
- `cmd/harness/main.go:137` dispatches `update`, but `internal/adapter/cli/usage.go:14-38` omits it from `Commands()` and `internal/adapter/cli/usage.go:78-89` omits it from usage text.
- `cmd/harness/worker.go:95-149` exposes `worker run --read-only`; `internal/core/worker.go:143-166` implements it through `RunReadOnlyCommand`; `internal/adapter/mcp/catalog.go:25-44` exposes only worker lifecycle MCP tools.
- `internal/core/policy.go:298-357` executes read-only argv commands with write/network/shell disabled, policy evaluation, timeout, env allowlist, redaction, and bounded output.
- `internal/core/draft_wiki_queue.go:119-164` reads and rewrites a shared JSONL queue while processing; line 150 ignores running-state rewrite errors; `internal/core/draft_wiki_queue.go:293-296` silently skips malformed queue lines.
- Contract coverage lives in `cmd/harness/contract_golden_test.go`, `cmd/harness/response_contract_golden_test.go`, `cmd/harness/testdata/*.golden.*`, and `internal/adapter/testdata/native_install_contract_matrix.golden.json`.

### Metis Review (gaps addressed)
- Treat the output as a prioritized backlog, not a single speculative feature.
- Define worker parity as read-only evidence parity across CLI/MCP only.
- Require explicit queue concurrency, malformed-line, and persistence-error contracts.
- Preserve dry-run/idempotency for installer refactors.
- Preserve host schemas/templates for hook routing refactors.

## Work Objectives
### Core Objective
Improve reliability and operator trust by eliminating public-surface drift, safely exposing read-only worker evidence through MCP, hardening draft-wiki queue processing, and reducing maintainability risks without weakening security boundaries.

### Deliverables
1. `update` appears consistently in dispatch, command catalog, usage text, README guidance, and golden contracts.
2. MCP exposes a policy-gated `worker_run_read_only` tool. This plan chooses MCP parity rather than CLI-only documentation.
3. Draft-wiki queue processing is serialized or single-worker guarded; malformed lines are reported; running-state persistence failure prevents external `agy` execution.
4. Worker/daemon verification covers timeout, denial/cancellation, stale state, socket permission/start-status-stop, and race-sensitive packages.
5. Codex/Claude installer duplication is reduced through shared helpers without behavior changes.
6. Hook routing moves to a Go-owned declarative rule table while preserving host output schemas and checked-in templates.
7. Draft-wiki promotion and LLM Wiki handoff get integration QA for collision, invalid type, dry-run/confirm, and no upstream core reimplementation.

### Definition of Done
- `go test ./... -count=1` passes.
- `go test -race ./... -count=1` passes, or a package-specific exception is documented with evidence.
- `go vet ./...` passes.
- `go build -o bin/agent-harness ./cmd/harness` passes.
- `go test ./cmd/harness -run Golden -count=1` passes.
- `go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -count=1` passes.
- CLI smoke confirms `./bin/agent-harness --help` includes `agent-harness update` and `agent-harness worker run --read-only`.
- MCP smoke confirms `worker_run_read_only` exists and denies write/network/shell attempts.
- Draft-wiki worker QA proves exactly-once processing under a two-worker race and safe malformed-line reporting.
- `./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json` passes after changes, or reports a newly documented non-regression with evidence.

### Must Have
- TDD/characterization-first implementation for every production change.
- Tests use temp `HOME`, `CODEX_HOME`, `HARNESS_STATE_DIR`, `HARNESS_WORKER_DIR`, and `HARNESS_DAEMON_DIR`; no real user-home mutation.
- Golden updates only through existing update flags with diff review.
- Redaction preserved for payloads, argv, stdout/stderr, queue material, errors, and MCP responses.
- Command execution remains argv-only, read-only, timeout-bounded, non-shell, non-network, and workspace-confined.

### Must NOT Have
- No writable worker runner.
- No arbitrary shell execution.
- No llm-wiki, CodeGraph, or claude-mem core reimplementation.
- No upstream plugin cache edits.
- No default project-local `.claude/skills`, `.claude/settings.json`, or `.mcp.json` writes.
- No weakening or deleting tests to make suites pass.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD for production changes; characterization tests first for refactors and public contract drift.
- QA policy: Every task has agent-executed scenarios with concrete commands and evidence paths.
- Evidence root: `evidence/current-project-improvements-2026-06-01/`.
- Manual QA channel: tmux for CLI/MCP/user-facing shell surfaces; bash for focused auxiliary checks.

## Execution Strategy
### Parallel Execution Waves
Wave 1: Task 1 only.
Wave 2: Tasks 2 and 3 in parallel after Task 1.
Wave 3: Tasks 4 and 7 in parallel after Tasks 2/3 stabilize runtime and queue contracts.
Wave 4: Tasks 5 and 6 in parallel after public behavior is pinned.
Wave 5: Final verification wave.

### Dependency Matrix
| Task | Depends On | Blocks | Parallel Notes |
| --- | --- | --- | --- |
| 1. Align update command surface | none | 2, 5, final | serial baseline contract fix |
| 2. Add MCP worker read-only parity | 1 | 4, final | can run with Task 3 |
| 3. Harden draft-wiki queue processing | 1 | 4, 7, final | can run with Task 2 |
| 4. Expand worker/daemon/race verification | 2, 3 | final | can run with Task 7 |
| 5. Deduplicate installers behavior-preservingly | 1 | final | can run with Task 6 |
| 6. Declarative hook-routing rules | 1 | final | can run with Task 5 |
| 7. Draft-wiki promotion integration QA | 3 | final | can run with Task 4 |

## TODOs
- [x] 1. Align `update` command across dispatch, catalog, usage, README, and golden contracts

  **What to do**: Add a characterization test that asserts `cliadapter.Commands()` and `cliadapter.Usage()` include `update`, then update catalog/usage and refresh goldens through the existing `-update` path if needed.
  **Must NOT do**: Do not change `runUpdate` semantics or installer behavior.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: 2, 5, final | Blocked By: none

  **References**:
  - Dispatch: `cmd/harness/main.go:137` - routes `update` to `runUpdate`.
  - Catalog: `internal/adapter/cli/usage.go:14` - owns top-level command list.
  - Usage: `internal/adapter/cli/usage.go:78` - install/bootstrap area omits `update`.
  - README: `README.md:135` - recommends `agent-harness update`.
  - Golden: `cmd/harness/testdata/usage.golden.txt`.

  **Acceptance Criteria**:
  - [ ] RED test proves `update` omission before fix.
  - [ ] `internal/adapter/cli/usage.go` includes `update` in `Commands()` and `Usage()`.
  - [ ] `go test ./internal/adapter/cli -count=1` passes.
  - [ ] `go test ./cmd/harness -run Golden -count=1` passes.
  - [ ] `./bin/agent-harness --help` includes `agent-harness update`.

  **QA Scenarios**:
  ```
  Scenario: help advertises update
    Tool: tmux
    Steps: tmux new-session -d -s ah-qa-update-help 'cd /Users/user/Workspace/agent-harness && mkdir -p evidence/current-project-improvements-2026-06-01 && go build -o bin/agent-harness ./cmd/harness && ./bin/agent-harness --help | tee evidence/current-project-improvements-2026-06-01/task-1-help.txt; python3 - <<"PY2"
from pathlib import Path
assert "agent-harness update" in Path("evidence/current-project-improvements-2026-06-01/task-1-help.txt").read_text()
PY2
 echo EXIT:$?'
    Expected: transcript contains `EXIT:0`.
    Evidence: evidence/current-project-improvements-2026-06-01/task-1-help.txt

  Scenario: update dry-run does not mutate real home
    Tool: bash
    Steps: tmp=$(mktemp -d); HOME="$tmp/home" CODEX_HOME="$tmp/codex" HARNESS_STATE_DIR="$tmp/state" ./bin/agent-harness update --dry-run --json > evidence/current-project-improvements-2026-06-01/task-1-update-dry-run.json; test ! -e "$tmp/home/.claude/settings.json"; rm -rf "$tmp"
    Expected: command exits 0, JSON is valid, no real user-home path is written.
    Evidence: evidence/current-project-improvements-2026-06-01/task-1-update-dry-run.json
  ```

  **Commit**: YES | Message: `fix(cli): include update in canonical command surface` | Files: `internal/adapter/cli/usage.go`, `internal/adapter/cli/*_test.go`, `cmd/harness/testdata/usage.golden.txt`, response contract golden if changed

- [x] 2. Add MCP parity for `worker run --read-only` without widening execution powers

  **What to do**: Add MCP tool `worker_run_read_only` mapping to `core.RunReadOnlyWorkerJob` with `kind`, optional `payload`, required `workspace_root`, `cwd`, optional `timeout`, optional `env`, and required `argv`. Force read-only semantics exactly as `RunReadOnlyCommand` does.
  **Must NOT do**: Do not add writable flags, shell string input, network permission, or background execution.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 4, final | Blocked By: 1

  **References**:
  - CLI: `cmd/harness/worker.go:95`.
  - Core: `internal/core/worker.go:143`.
  - Policy: `internal/core/policy.go:298`.
  - MCP catalog: `internal/adapter/mcp/catalog.go:25`.
  - Contract snapshot: `cmd/harness/response_contract_golden_test.go:166`.

  **Acceptance Criteria**:
  - [ ] MCP schema includes `worker_run_read_only` with required `workspace_root`, `cwd`, and `argv`.
  - [ ] MCP handler returns the same `WorkerJob` shape as CLI read-only worker jobs.
  - [ ] Test proves `git status --short` succeeds through MCP.
  - [ ] Test proves write, network, and shell attempts are denied through MCP.
  - [ ] MCP tool/schema and response contract goldens are updated intentionally.

  **QA Scenarios**:
  ```
  Scenario: MCP worker read-only success
    Tool: tmux
    Steps: tmux new-session -d -s ah-qa-worker-mcp 'cd /Users/user/Workspace/agent-harness && tmp=$(mktemp -d) && HARNESS_WORKER_DIR="$tmp/worker" go test ./cmd/harness -run TestMCPWorkerRunReadOnlyAllowsGitStatus -count=1; code=$?; rm -rf "$tmp"; echo EXIT:$code'
    Expected: transcript contains `PASS` and `EXIT:0`.
    Evidence: evidence/current-project-improvements-2026-06-01/task-2-mcp-worker-success.txt

  Scenario: MCP worker denies unsafe commands
    Tool: bash
    Steps: go test ./cmd/harness -run TestMCPWorkerRunReadOnlyDeniesWriteNetworkAndShell -count=1 | tee evidence/current-project-improvements-2026-06-01/task-2-mcp-worker-deny.txt
    Expected: test passes and assertions cover write, network, and shell denial reasons.
    Evidence: evidence/current-project-improvements-2026-06-01/task-2-mcp-worker-deny.txt
  ```

  **Commit**: YES | Message: `feat(worker): expose read-only worker evidence via MCP` | Files: `internal/adapter/mcp/catalog.go`, MCP handler files, `cmd/harness/*_test.go`, `cmd/harness/testdata/*.golden.*`, docs if surface changes

- [x] 3. Harden draft-wiki queue processing for concurrency, malformed lines, and persistence errors

  **What to do**: Add a per-project queue lock or explicit single-worker lease file under project state. Return the running-state rewrite error before invoking `agy`. Report malformed JSONL lines as bounded warnings while continuing valid event processing.
  **Must NOT do**: Do not run real `agy` in tests; use a fake executable. Do not expose `SourceMaterial` in response JSON.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 4, 7, final | Blocked By: 1

  **References**:
  - Queue loop: `internal/core/draft_wiki_queue.go:119`.
  - Ignored rewrite: `internal/core/draft_wiki_queue.go:150`.
  - Malformed skip: `internal/core/draft_wiki_queue.go:293`.
  - Atomic rewrite: `internal/core/draft_wiki_queue.go:302`.
  - Existing tests: `internal/core/draft_wiki_test.go`.

  **Acceptance Criteria**:
  - [ ] Concurrent two-worker test proves one queued event is processed exactly once.
  - [ ] Running-state rewrite failure test proves `agy` is not invoked.
  - [ ] Malformed JSONL line is reported with line number and bounded/redacted text; valid events continue.
  - [ ] Queue responses still omit `SourceMaterial`.
  - [ ] `go test ./internal/core -run 'DraftWikiQueue' -count=1` passes.

  **QA Scenarios**:
  ```
  Scenario: two workers race safely
    Tool: tmux
    Steps: create temp repo/state/fake agy; enqueue one event; run two `./bin/agent-harness worker draft-wiki --repo <repo> --agy-command <fake> --limit 1 --json` commands in parallel tmux panes; capture both panes.
    Expected: exactly one result succeeds, the other is no-op/locked; only one draft file exists.
    Evidence: evidence/current-project-improvements-2026-06-01/task-3-draft-wiki-race.txt

  Scenario: malformed queue line is reported safely
    Tool: bash
    Steps: write invalid JSONL plus one valid queued event into temp project state; run worker with fake agy and capture JSON.
    Expected: JSON has bounded malformed-line warning and no secret/source material; valid event follows report-and-continue behavior.
    Evidence: evidence/current-project-improvements-2026-06-01/task-3-draft-wiki-malformed.json
  ```

  **Commit**: YES | Message: `fix(draft-wiki): serialize queue processing safely` | Files: `internal/core/draft_wiki_queue.go`, `internal/core/draft_wiki_test.go`, response contract golden if response shape changes

- [x] 4. Expand worker, daemon, and race verification around operational risks

  **What to do**: Add tests/QA for worker timeout and denial, daemon start-status-stop, socket permission or stale lock behavior, and race-sensitive packages.
  **Must NOT do**: Do not depend on user daemon state or real user config.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final | Blocked By: 2, 3

  **References**:
  - Worker tests: `internal/core/audit_worker_test.go`.
  - Daemon tests: `cmd/harness/daemon_test.go`.
  - Race policy: `.agent-harness/TESTING.md`.
  - Worker dir override: `internal/core/worker.go:187`.

  **Acceptance Criteria**:
  - [ ] Worker timeout test proves timeout exit code and bounded stderr.
  - [ ] Worker denial test proves denied command produces failed job and no marker file.
  - [ ] Daemon smoke proves start/status/stop under temp `HARNESS_DAEMON_DIR`.
  - [ ] Socket permission or stale lock test covers failure/recovery path.
  - [ ] `go test -race ./... -count=1` evidence is captured.

  **QA Scenarios**:
  ```
  Scenario: isolated daemon lifecycle smoke
    Tool: tmux
    Steps: tmux new-session -d -s ah-qa-daemon 'cd /Users/user/Workspace/agent-harness && tmp=$(mktemp -d) && HARNESS_DAEMON_DIR="$tmp/daemon" ./bin/agent-harness daemon start --json && HARNESS_DAEMON_DIR="$tmp/daemon" ./bin/agent-harness daemon status --json && HARNESS_DAEMON_DIR="$tmp/daemon" ./bin/agent-harness daemon stop --json; code=$?; rm -rf "$tmp"; echo EXIT:$code'
    Expected: transcript contains start/status/stop JSON and `EXIT:0`.
    Evidence: evidence/current-project-improvements-2026-06-01/task-4-daemon-lifecycle.txt

  Scenario: race suite
    Tool: bash
    Steps: go test -race ./... -count=1 | tee evidence/current-project-improvements-2026-06-01/task-4-race.txt
    Expected: all packages pass under race detector.
    Evidence: evidence/current-project-improvements-2026-06-01/task-4-race.txt
  ```

  **Commit**: YES | Message: `test(runtime): expand worker and daemon operational coverage` | Files: `internal/core/*_test.go`, `cmd/harness/daemon_test.go`, docs/testing updates if new required command is adopted

- [x] 5. Deduplicate Codex/Claude installers without behavior changes

  **What to do**: Characterize current installer output for user-global and project-local dry-runs, then extract shared helpers only for common mechanics: skill symlink planning, dry-run action formatting, template write planning, backup naming, and install result assembly. Keep host-specific config schemas in host adapters.
  **Must NOT do**: Do not change hook commands, default targets, project-local opt-in behavior, or compatibility shims.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: final | Blocked By: 1

  **References**:
  - Codex adapter: `internal/adapter/codex/install.go`.
  - Claude adapter: `internal/adapter/claude/install.go`.
  - Shared utilities: `internal/adapter/installutil/install_util.go`.
  - Matrix: `internal/adapter/install_contract_matrix_test.go` and `internal/adapter/testdata/native_install_contract_matrix.golden.json`.

  **Acceptance Criteria**:
  - [ ] Characterization test/golden passes before refactor.
  - [ ] Refactor reduces duplicated helper logic while preserving exact adapter contract matrix.
  - [ ] `go test ./internal/adapter/... -count=1` passes.
  - [ ] Adapter contract matrix test passes without unexpected golden drift.
  - [ ] Dry-run smoke uses temp HOME/CODEX_HOME and writes no real user config.

  **QA Scenarios**:
  ```
  Scenario: native install contract unchanged
    Tool: bash
    Steps: go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -count=1 | tee evidence/current-project-improvements-2026-06-01/task-5-adapter-matrix.txt
    Expected: test passes without unexpected golden drift.
    Evidence: evidence/current-project-improvements-2026-06-01/task-5-adapter-matrix.txt

  Scenario: dry-run install remains user-safe
    Tool: tmux
    Steps: tmux new-session -d -s ah-qa-install 'cd /Users/user/Workspace/agent-harness && tmp=$(mktemp -d) && HOME="$tmp/home" CODEX_HOME="$tmp/codex" ./bin/agent-harness install-native --dry-run --json; code=$?; find "$tmp" -maxdepth 3 -type f | sort; rm -rf "$tmp"; echo EXIT:$code'
    Expected: JSON reports planned actions; no real user home touched; `EXIT:0`.
    Evidence: evidence/current-project-improvements-2026-06-01/task-5-install-dry-run.txt
  ```

  **Commit**: YES | Message: `refactor(install): share native installer mechanics` | Files: `internal/adapter/installutil/*`, `internal/adapter/codex/install.go`, `internal/adapter/claude/install.go`, tests

- [x] 6. Replace heuristic hook routing internals with a Go-owned declarative rule table

  **What to do**: Add a small core `HookRoutingRule` table for keyword sets, target tools/actions, priority, and compact labels. Refactor `BuildUserPromptMCPHints` to iterate rules. Preserve current outputs through characterization tests, including Codex/Claude host differences and opt-in `agy -p` hints.
  **Must NOT do**: Do not make agy hints default-on. Do not move rules to unchecked external config. Do not inject noisy prose into Codex-visible additional context.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: final | Blocked By: 1

  **References**:
  - Current hook routing: `internal/core/hook_prompt.go`.
  - CLI hook wiring: `cmd/harness/hook_user_prompt.go`.
  - Codex template: `configs/codex/hooks.json`.
  - Claude template: `configs/claude/hooks.settings.json`.
  - Cautions: `.agent-harness/CAUTIONS.md` hook rendering guidance.

  **Acceptance Criteria**:
  - [ ] Characterization tests cover API docs, project docs, CodeGraph, LLM Wiki, claude-mem, and opt-in agy.
  - [ ] Rule table refactor passes same tests without output drift except accepted ordering normalization.
  - [ ] Template/installer tests prove checked-in hook configs match generated command strings.
  - [ ] `--enable-agy-hints` and `HARNESS_ENABLE_AGY_HINTS` remain opt-in only.

  **QA Scenarios**:
  ```
  Scenario: prompt routing remains stable
    Tool: bash
    Steps: printf '{"prompt":"endpoint와 DTO를 추가하고 리뷰해줘"}' | ./bin/agent-harness hook user-prompt --enable-agy-hints --json > evidence/current-project-improvements-2026-06-01/task-6-hook-routing.json; python3 - <<'PY2'
import json
text=open('evidence/current-project-improvements-2026-06-01/task-6-hook-routing.json').read()
assert 'agy -p' in text and 'api' in text.lower()
json.loads(text)
PY2
    Expected: JSON includes expected routing hints and agy only when enabled.
    Evidence: evidence/current-project-improvements-2026-06-01/task-6-hook-routing.json

  Scenario: default agy hint remains absent
    Tool: bash
    Steps: printf '{"prompt":"리뷰해줘"}' | ./bin/agent-harness hook user-prompt --json > evidence/current-project-improvements-2026-06-01/task-6-hook-no-agy.json; python3 - <<'PY2'
text=open('evidence/current-project-improvements-2026-06-01/task-6-hook-no-agy.json').read()
assert 'agy -p' not in text
PY2
    Expected: no agy hint without opt-in.
    Evidence: evidence/current-project-improvements-2026-06-01/task-6-hook-no-agy.json
  ```

  **Commit**: YES | Message: `refactor(hook): centralize prompt routing rules` | Files: `internal/core/hook_prompt.go`, `internal/core/hook_prompt_test.go`, `cmd/harness/hook_user_prompt_test.go`, config/template tests if added

- [x] 7. Expand draft-wiki and LLM Wiki promotion integration QA

  **What to do**: Add tests and CLI/tmux QA for queue suggestion to draft file, approve/reject, promote dry-run/confirm, collision handling, invalid target type, and upstream handoff boundaries. Confirm agent-harness writes only approved raw note/log handoff files and does not compile/query/index llm-wiki.
  **Must NOT do**: Do not call real upstream LLM Wiki or real `agy`; use temp wiki roots and fake agy.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: final | Blocked By: 3

  **References**:
  - Queue worker: `cmd/harness/worker.go:47`.
  - Draft-wiki core: `internal/core/draft_wiki.go`, `internal/core/draft_wiki_suggest.go`, `internal/core/llm_wiki_promote.go`.
  - Existing tests: `internal/core/draft_wiki_test.go`.
  - Policy: `.agent-harness/CAUTIONS.md` LLM Wiki 재구현 금지.

  **Acceptance Criteria**:
  - [ ] Promote dry-run reports planned raw note/log path without writing.
  - [ ] Promote confirm writes only approved draft content to configured temp wiki raw path and log.
  - [ ] Collision returns bounded error and does not overwrite existing raw note.
  - [ ] Invalid target type returns deterministic validation error.
  - [ ] Tests assert no compile/query/index behavior is claimed or invoked.

  **QA Scenarios**:
  ```
  Scenario: approved draft promotes to temp wiki only
    Tool: tmux
    Steps: tmux new-session -d -s ah-qa-draft-promote 'cd /Users/user/Workspace/agent-harness && tmp=$(mktemp -d) && ./bin/agent-harness project draft-wiki init --repo "$tmp/repo" --json && echo prepare approved draft and temp wiki config && ./bin/agent-harness project draft-wiki promote --repo "$tmp/repo" --target-wiki test --confirm --json; code=$?; find "$tmp" -maxdepth 5 -type f | sort; rm -rf "$tmp"; echo EXIT:$code'
    Expected: raw note and log exist under temp wiki; no compile/query/index files/actions; `EXIT:0`.
    Evidence: evidence/current-project-improvements-2026-06-01/task-7-promote.txt

  Scenario: collision refuses overwrite
    Tool: bash
    Steps: prepare temp approved draft and pre-existing raw target; run promote --confirm --json; capture non-zero output.
    Expected: deterministic collision error; existing file content unchanged.
    Evidence: evidence/current-project-improvements-2026-06-01/task-7-collision.txt
  ```

  **Commit**: YES | Message: `test(draft-wiki): cover promotion handoff boundaries` | Files: `internal/core/draft_wiki_test.go`, CLI tests if needed, docs if behavior contract is clarified

## Final Verification Wave (MANDATORY - after ALL implementation tasks)
> ALL must APPROVE. Present consolidated results to user and get explicit okay before completing.
- [ ] F1. Plan Compliance Audit: verify every task acceptance criterion has evidence under `evidence/current-project-improvements-2026-06-01/`.
- [ ] F2. Code Quality Review: spawn `code-reviewer`; focus on worker execution safety, draft-wiki queue locking, host adapter parity, and no companion reimplementation.
- [ ] F3. Real Manual QA: run every tmux/bash scenario and confirm `tmux ls` has no `ah-qa-*` sessions left.
- [ ] F4. Full Verification Commands:
  - `go test ./... -count=1`
  - `go test -race ./... -count=1`
  - `go vet ./...`
  - `go build -o bin/agent-harness ./cmd/harness`
  - `go test ./cmd/harness -run Golden -count=1`
  - `go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -count=1`
  - `./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json`
- [ ] F5. Scope Fidelity Check: no writable shell runner, no upstream companion core reimplementation, no real user-config mutation, and intentional golden diffs only.

## Commit Strategy
- Task 1: `fix(cli): include update in canonical command surface`
- Task 2: `feat(worker): expose read-only worker evidence via MCP`
- Task 3: `fix(draft-wiki): serialize queue processing safely`
- Task 4: `test(runtime): expand worker and daemon operational coverage`
- Task 5: `refactor(install): share native installer mechanics`
- Task 6: `refactor(hook): centralize prompt routing rules`
- Task 7: `test(draft-wiki): cover promotion handoff boundaries`

Each commit must build and pass targeted tests before the next commit. Final implementation commits should include footer `Plan: plans/current-project-improvements-2026-06-01.md`.

## Success Criteria
- Planning is complete when every TODO has concrete references, acceptance criteria, QA scenarios, dependencies, and commit guidance.
- Implementation is complete only when all tasks and final verification wave pass with captured evidence and review approval.
