# Restore Agent-Harness Full Test Baseline

## TL;DR
> Summary:      Restore the current `go test ./... -count=1` baseline by treating `TestResponseContractsGolden` as the red/green driver, refreshing `cmd/issueops/testdata/response_contracts.golden.json` only after proving the generated contract is correct, and preserving the existing CodeGraph PreToolUse + opt-in `Z.AI Coding Plan` hook changes.
> Deliverables:
> - Red/green evidence for `TestResponseContractsGolden` and `go test ./cmd/issueops -run Golden -count=1`
> - A validated response-contract golden refresh, or a documented no-op if the working tree already contains the correct generated snapshot
> - Focused verification that CodeGraph PreToolUse enforcement still blocks raw repo-source searches only when opted in
> - Focused verification that `Z.AI Coding Plan` hints remain core opt-in while Codex/Claude hook templates explicitly enable them
> - Full Go baseline evidence and one clean atomic commit instruction
> Effort:       Short
> Risk:         Medium - a golden refresh can accidentally hide a contract regression or host-hook schema drift if not diff-reviewed.

## Scope
### Must have
- Preserve the existing uncommitted hook behavior in `cmd/issueops/hook_user_prompt.go`, `internal/core/hook_prompt.go`, `internal/core/lifecycle_state.go`, their tests, and `configs/{codex,claude}` hook templates.
- Characterize the failing baseline before refreshing any golden fixture, using `TestResponseContractsGolden` at `cmd/issueops/response_contract_golden_test.go:18` and its assertion at `cmd/issueops/response_contract_golden_test.go:200`.
- Use the existing golden update mechanism defined by `updateGolden` and `assertGolden` in `cmd/issueops/contract_golden_test.go:12` and `cmd/issueops/contract_golden_test.go:48-63`; do not hand-edit generated JSON unless the generated output has already been reviewed.
- Verify that response-contract dynamic fields remain normalized by the test helpers in `cmd/issueops/response_contract_golden_test.go:356-407` and `cmd/issueops/response_contract_golden_test.go:515-522`.
- Capture task evidence under `evidence/task-<N>-<slug>.txt`.
- Run project-required Go verification from `.issueops/TESTING.md:96-121` after the baseline is restored.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- Do not revert or weaken CodeGraph PreToolUse enforcement in `internal/core/lifecycle_state.go:415-432`.
- Do not make `Z.AI Coding Plan` hints default-on in core; `HookUserPromptRequest.EnableLLMHints` is opt-in in `internal/core/hook_prompt.go:6-13` and tested in `internal/core/hook_prompt_test.go:78-89`.
- Do not add new source abstractions, new hook commands, new MCP tools, or any unrelated refactor.
- Do not write user-level `~/.codex`, `~/.claude`, `.claude/skills`, daemon state, or upstream companion plugin cache files.
- Do not update `cmd/issueops/testdata/response_contracts.golden.json` if the generated output includes unnormalized local paths, timestamps, secret-like values, or unexpected response fields.
- Do not commit `.omo/**`, `.omx/**`, `.omc/**`, build artifacts under `bin/**`, or unrelated local runtime state.

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: TDD + Go `testing`; characterize RED first, then regenerate golden via `-update`, then prove GREEN with targeted and full-suite tests.
- QA policy: every task has agent-executed scenarios.
- Evidence: `evidence/task-<N>-<slug>.txt`

## Execution strategy
### Parallel execution waves
> Target 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks to maximize parallelism.

Wave 1 (no dependencies):
- Task 1: Capture baseline failure and protect working-tree scope
- Task 3: Preserve CodeGraph PreToolUse behavior with focused tests and CLI smoke
- Task 4: Preserve opt-in `Z.AI Coding Plan` hint behavior with focused tests and CLI smoke
- Task 5: Validate Codex/Claude hook template parsing and command strings
- Task 6: Check contract/golden safety constraints before accepting a snapshot refresh

Wave 2 (after Wave 1; intentionally serial because one generated golden file must not be refreshed concurrently):
- Task 2: depends [1, 6] - Regenerate or no-op the response-contract golden after review

Wave 3 (after Wave 2):
- Task 7: depends [2, 3, 4, 5] - Run full baseline verification and prepare atomic commit

Critical path: Task 1 + Task 6 -> Task 2 -> Task 7

### Dependency matrix
| Task | Depends on | Blocks | Can parallelize with |
|------|------------|--------|----------------------|
| 1    | none       | 2      | 3, 4, 5, 6           |
| 2    | 1, 6       | 7      | none                 |
| 3    | none       | 7      | 1, 4, 5, 6           |
| 4    | none       | 7      | 1, 3, 5, 6           |
| 5    | none       | 7      | 1, 3, 4, 6           |
| 6    | none       | 2      | 1, 3, 4, 5           |
| 7    | 2, 3, 4, 5 | final  | none                 |

## Todos
> Implementation + Test = ONE task. Never separate.
> Every task MUST have: References + Acceptance Criteria + QA Scenarios + Commit.

- [ ] 1. Capture baseline failure and protect working-tree scope

  What to do: Record the current modified-file set, run the narrow response-contract test without `-update`, and save the exact red output. If the golden has already been regenerated in this working tree and the narrow test passes, record that as an already-green state and include the current `git diff -- cmd/issueops/testdata/response_contracts.golden.json` as the baseline delta instead of forcing a failure.
  Must NOT do: Do not edit files, run `-update`, stage files, clean untracked evidence, or revert existing hook changes.

  Parallelization: Can parallel: YES | Wave 1 | Blocks: [2, 6] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:18-200` - `TestResponseContractsGolden` builds CLI/MCP snapshots and compares `response_contracts.golden.json`.
  - Pattern:  `cmd/issueops/contract_golden_test.go:48-63` - golden assertion prints `got` and `want` and points to the `-update` command.
  - Test:     `cmd/issueops/response_contract_golden_test.go` - targeted RED/GREEN driver.
  - Project:  `.issueops/TESTING.md:116-121` - golden files update only for intentional CLI/MCP contract changes.
  - External: `https://go.dev/cmd/go/#hdr-Test_packages` - official `go test` package command behavior.

  Acceptance criteria (agent-executable only):
  - [ ] `evidence/task-1-baseline-red.txt` exists and contains `git status --short`, `git diff --stat`, the exact `go test ./cmd/issueops -run TestResponseContractsGolden -count=1` result, and `exit=<code>`.
  - [ ] The evidence shows either `FAIL: TestResponseContractsGolden` / `golden mismatch for response_contracts.golden.json` or an already-green explanation plus the current golden diff.
  - [ ] `git diff --name-status` still lists only the expected source/test/config/golden/plan/evidence files; no unrelated tracked file appears.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: baseline characterization
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && bash -lc '{ date -Is; git status --short; git diff --stat; go test ./cmd/issueops -run TestResponseContractsGolden -count=1; printf "exit=%s\n" "$?"; } 2>&1 | tee evidence/task-1-baseline-red.txt'
    Expected: evidence file contains either a TestResponseContractsGolden golden mismatch with exit=1, or an already-green `ok issueops/cmd/issueops` plus current golden diff noted by the executor.
    Evidence: evidence/task-1-baseline-red.txt

  Scenario: unrelated-change guard
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && git diff --name-status | tee evidence/task-1-diff-scope.txt
    Expected: output contains only cmd/issueops/hook_user_prompt.go, cmd/issueops/hook_user_prompt_test.go, cmd/issueops/testdata/response_contracts.golden.json, configs/claude/hooks.settings.json, configs/codex/hooks.json, internal/core/hook_prompt.go, internal/core/hook_prompt_test.go, internal/core/lifecycle_state.go, internal/core/lifecycle_state_test.go, and later plan/evidence files.
    Evidence: evidence/task-1-diff-scope.txt
  ```

  Commit: NO | Message: `test(contract): characterize response golden baseline` | Files: [evidence/task-1-baseline-red.txt, evidence/task-1-diff-scope.txt]

- [ ] 2. Regenerate or no-op the response-contract golden after review

  What to do: Review the Task 1 mismatch and Task 6 safety output. If the generated contract is correct, run the existing `-update` path for `cmd/issueops` golden tests. Re-run targeted golden tests twice without `-update`. If the current `cmd/issueops/testdata/response_contracts.golden.json` already matches generated output, record a no-op decision and still run the same green checks.
  Must NOT do: Do not hand-edit JSON, update unrelated golden files, accept volatile values, or change production code to match a stale fixture.

  Parallelization: Can parallel: NO | Wave 2 | Blocks: [7] | Blocked by: [1, 6]

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/contract_golden_test.go:12` - `-update` flag is the approved golden update mechanism.
  - Pattern:  `cmd/issueops/contract_golden_test.go:48-63` - update writes `testdata/<name>` only when `-update` is set.
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:356-407` - contract values are recursively normalized before comparison.
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:515-522` - dynamic time keys are normalized to `$TIMESTAMP`.
  - Test:     `cmd/issueops/testdata/response_contracts.golden.json` - only generated fixture expected to change for this baseline repair.
  - Project:  `.issueops/CONVENTIONS.md:84-87` - schema changes require golden tests, dynamic fields must be normalized.
  - External: `https://pkg.go.dev/cmd/go#hdr-Testing_flags` - official testing flag semantics for `-run` and repeated test invocation.

  Acceptance criteria (agent-executable only):
  - [ ] `go test ./cmd/issueops -run Golden -update -count=1` exits 0 when an update is needed, and `evidence/task-2-golden-update.txt` captures the output.
  - [ ] `go test ./cmd/issueops -run Golden -count=1` exits 0 twice after the update/no-op decision; outputs are saved to `evidence/task-2-golden-green-1.txt` and `evidence/task-2-golden-green-2.txt`.
  - [ ] `git diff -- cmd/issueops/testdata/response_contracts.golden.json` is saved to `evidence/task-2-golden-diff.txt` and contains only reviewed response-contract deltas.
  - [ ] `grep -En '/Users/|/var/folders|TOKEN=secret-value|secret-value|[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:' cmd/issueops/testdata/response_contracts.golden.json` returns no matches.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: approved golden refresh
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && bash -lc 'go test ./cmd/issueops -run Golden -update -count=1' 2>&1 | tee evidence/task-2-golden-update.txt && git diff -- cmd/issueops/testdata/response_contracts.golden.json | tee evidence/task-2-golden-diff.txt
    Expected: update command prints `ok issueops/cmd/issueops`; diff is limited to response-contract generated values reviewed by the executor.
    Evidence: evidence/task-2-golden-update.txt and evidence/task-2-golden-diff.txt

  Scenario: stale-state/flakiness guard
    Tool:     tmux
    Steps:    cd /Users/user/Workspace/issueops && tmux new-session -d -s ah_task2_golden 'cd /Users/user/Workspace/issueops && go test ./cmd/issueops -run Golden -count=1 && go test ./cmd/issueops -run Golden -count=1' && sleep 3 && tmux capture-pane -pt ah_task2_golden | tee evidence/task-2-golden-tmux.txt; tmux kill-session -t ah_task2_golden 2>/dev/null || true
    Expected: captured pane contains two `ok issueops/cmd/issueops` lines and no `FAIL`.
    Evidence: evidence/task-2-golden-tmux.txt
  ```

  Commit: NO | Message: `test(contract): refresh response contract golden` | Files: [cmd/issueops/testdata/response_contracts.golden.json, evidence/task-2-golden-update.txt, evidence/task-2-golden-diff.txt, evidence/task-2-golden-green-1.txt, evidence/task-2-golden-green-2.txt, evidence/task-2-golden-tmux.txt]

- [ ] 3. Preserve hybrid search-routing PreToolUse behavior with focused tests and CLI smoke

  What to do: Run focused core and CLI tests that cover the hybrid search-routing enforcement. If a focused test fails, apply only the minimal correction in `internal/core/lifecycle_state.go` or `cmd/issueops/hook_user_prompt.go`; keep the raw host PreToolUse path no-op unless `--enforce-search-routing` is passed.
  Must NOT do: Do not make PreToolUse block by default, do not remove the block reason, and do not route docs/golden literal searches through CodeGraph.

  Parallelization: Can parallel: YES | Wave 1 | Blocks: [7] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - API/Type: `internal/core/lifecycle_state.go:362-386` - request/result DTOs include `EnforceSearchRouting` and optional `reason`.
  - Pattern:  `internal/core/lifecycle_state.go:415-432` - enforcement changes an allow decision to block only when opt-in policy detects a clear CodeGraph/rg routing mismatch.
  - Pattern:  `internal/core/lifecycle_state.go:435-530` - shell-tool, raw text search, and docs/fixture target classification helpers.
  - Pattern:  `cmd/issueops/hook_user_prompt.go:143-178` - CLI flag wiring and host JSON behavior for PreToolUse.
  - Test:     `internal/core/lifecycle_state_test.go:139-170` - blocks bypass forms and allows docs literal code-name search.
  - Test:     `cmd/issueops/hook_user_prompt_test.go:174-209` - CLI raw JSON and host JSON behavior for enforced source search.
  - Project:  `.issueops/CONVENTIONS.md:210-213` - PreToolUse output must remain event-specific, small, deterministic, and adapter-only.

  Acceptance criteria (agent-executable only):
  - [ ] `go test ./internal/core -run 'TestPreToolUseSearchRouting' -count=1` exits 0 and is saved to `evidence/task-3-search-routing-core.txt`.
  - [ ] `go test ./cmd/issueops -run 'TestRunHookPreToolUse' -count=1` exits 0 and is saved to `evidence/task-3-search-routing-cli-tests.txt`.
  - [ ] A direct CLI smoke with `--enforce-search-routing --json` returns JSON where `decision == "block"` and `reason` points to the more efficient tool.
  - [ ] A direct CLI smoke without `--enforce-search-routing` returns `{}` host JSON for the same raw source-search input.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: enforced source search blocks
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && printf '%s\n' '{"cwd":"/Users/user/Workspace/issueops","tool_name":"Bash","tool_input":{"command":"rg -n \"func Run\" cmd internal"}}' | go run ./cmd/issueops hook pre-tool-use --enforce-search-routing --json | tee evidence/task-3-search-routing-block.json && python3 - <<'PY'
              import json
              data=json.load(open('evidence/task-3-search-routing-block.json'))
              assert data['decision']=='block', data
              assert 'CodeGraph' in data.get('reason','') and 'codegraph_context' in data.get('reason',''), data
              PY
    Expected: Python assertion exits 0; JSON has `decision: block` and CodeGraph guidance.
    Evidence: evidence/task-3-search-routing-block.json

  Scenario: default host output remains no-op
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && printf '%s\n' '{"cwd":"/Users/user/Workspace/issueops","tool_name":"Bash","tool_input":{"command":"rg -n \"func Run\" cmd internal"}}' | go run ./cmd/issueops hook pre-tool-use | tee evidence/task-3-codegraph-default.json && python3 - <<'PY'
              import json
              data=json.load(open('evidence/task-3-codegraph-default.json'))
              assert data == {}, data
              PY
    Expected: Python assertion exits 0; default host JSON is `{}`.
    Evidence: evidence/task-3-codegraph-default.json
  ```

  Commit: NO | Message: `feat(hooks): enforce CodeGraph source search opt-in` | Files: [internal/core/lifecycle_state.go, internal/core/lifecycle_state_test.go, cmd/issueops/hook_user_prompt.go, cmd/issueops/hook_user_prompt_test.go, evidence/task-3-codegraph-core.txt, evidence/task-3-codegraph-cli-tests.txt, evidence/task-3-codegraph-block.json, evidence/task-3-codegraph-default.json]

- [ ] 4. Preserve opt-in `Z.AI Coding Plan` hint behavior with focused tests and CLI smoke

  What to do: Run focused core and CLI tests for `Z.AI Coding Plan` hint routing. If a focused test fails, apply only the minimal correction in `internal/core/hook_prompt.go` or `cmd/issueops/hook_user_prompt.go`; keep core default disabled and require `--enable-llm-hints` or `ISSUEOPS_ENABLE_LLM_HINTS` for injection.
  Must NOT do: Do not inject `Z.AI Coding Plan` hints for every prompt, do not add noisy route/action/profile prose to Codex-visible additional context, and do not remove existing CodeGraph/LLM Wiki/claude-mem routing.

  Parallelization: Can parallel: YES | Wave 1 | Blocks: [7] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - API/Type: `internal/core/hook_prompt.go:6-13` - `HookUserPromptRequest` includes `EnableLLMHints`.
  - Pattern:  `internal/core/hook_prompt.go:88-95` - Z.AI hint is added only when opt-in and the prompt asks for review/analysis/plan/research.
  - Pattern:  `internal/core/hook_prompt.go:208-214` - Z.AI is classified as a secondary hint.
  - Pattern:  `internal/core/hook_prompt.go:269-275` - compact label renders `Z.AI Coding Plan for LLM second-pass review`.
  - Pattern:  `cmd/issueops/hook_user_prompt.go:52-87` - CLI flag/env wiring and host-neutral UserPromptSubmit output.
  - Test:     `internal/core/hook_prompt_test.go:78-89` - core default-off and enabled-on behavior.
  - Test:     `cmd/issueops/hook_user_prompt_test.go:68-79` - CLI default-off and `--enable-llm-hints` behavior.
  - Project:  `.issueops/CAUTIONS.md:143-152` - Codex visible hook context must avoid noisy prose and host-rendering assumptions.

  Acceptance criteria (agent-executable only):
  - [ ] `go test ./internal/core -run TestBuildUserPromptMCPHintsRoutesLLMReviewToLLMWhenEnabled -count=1` exits 0 and is saved to `evidence/task-4-Z.AI-core.txt`.
  - [ ] `go test ./cmd/issueops -run TestRunHookUserPromptLLMHintsAreOptIn -count=1` exits 0 and is saved to `evidence/task-4-Z.AI-cli-tests.txt`.
  - [ ] CLI smoke without `--enable-llm-hints` produces no `Z.AI Coding Plan` in `hookSpecificOutput.additionalContext`.
  - [ ] CLI smoke with `--enable-llm-hints` includes `Z.AI Coding Plan for LLM second-pass review`.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: Z.AI default remains disabled
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && printf '%s\n' '{"prompt":"이 계획을 검토하고 개선점을 분석해줘","cwd":"/Users/user/Workspace/issueops"}' | go run ./cmd/issueops hook user-prompt | tee evidence/task-4-Z.AI-disabled.json && python3 - <<'PY'
              import json
              data=json.load(open('evidence/task-4-Z.AI-disabled.json'))
              ctx=data['hookSpecificOutput'].get('additionalContext','')
              assert 'Z.AI Coding Plan' not in ctx, ctx
              PY
    Expected: Python assertion exits 0; no `Z.AI Coding Plan` hint appears by default.
    Evidence: evidence/task-4-Z.AI-disabled.json

  Scenario: Z.AI enabled flag injects secondary hint
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && printf '%s\n' '{"prompt":"이 계획을 검토하고 개선점을 분석해줘","cwd":"/Users/user/Workspace/issueops"}' | go run ./cmd/issueops hook user-prompt --enable-llm-hints | tee evidence/task-4-Z.AI-enabled.json && python3 - <<'PY'
              import json
              data=json.load(open('evidence/task-4-Z.AI-enabled.json'))
              ctx=data['hookSpecificOutput'].get('additionalContext','')
              assert 'Z.AI Coding Plan for LLM second-pass review' in ctx, ctx
              PY
    Expected: Python assertion exits 0; enabled output includes the secondary Z.AI hint.
    Evidence: evidence/task-4-Z.AI-enabled.json
  ```

  Commit: NO | Message: `feat(hooks): add opt-in Z.AI review hints` | Files: [internal/core/hook_prompt.go, internal/core/hook_prompt_test.go, cmd/issueops/hook_user_prompt.go, cmd/issueops/hook_user_prompt_test.go, evidence/task-4-Z.AI-core.txt, evidence/task-4-Z.AI-cli-tests.txt, evidence/task-4-Z.AI-disabled.json, evidence/task-4-Z.AI-enabled.json]

- [ ] 5. Validate Codex/Claude hook template parsing and command strings

  What to do: Parse both hook config templates as JSON and assert the intended flags are present exactly once in the right event commands. If a config check fails, edit only the affected template command string.
  Must NOT do: Do not write user-level host settings, do not add project-local `.claude` files, and do not change timeouts/matchers unless a parser or command-string assertion proves it is necessary.

  Parallelization: Can parallel: YES | Wave 1 | Blocks: [7] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `configs/codex/hooks.json:36-43` - Codex PreToolUse template invokes `hook pre-tool-use`; search-routing enforcement is opt-in.
  - Pattern:  `configs/codex/hooks.json:69-76` - Codex UserPromptSubmit template invokes `hook user-prompt --host codex --enable-llm-hints`.
  - Pattern:  `configs/claude/hooks.settings.json:37-46` - Claude PreToolUse template invokes `hook pre-tool-use --host claude` under matcher `*`; search-routing enforcement is opt-in.
  - Pattern:  `configs/claude/hooks.settings.json:71-78` - Claude UserPromptSubmit template invokes `hook user-prompt --enable-llm-hints`.
  - Project:  `.issueops/CONSTITUTION.md:50-57` - host adapters must not bypass core policy.
  - Project:  `.issueops/CONVENTIONS.md:210-213` - host-specific hook settings belong in templates; common routing remains in CLI/core.

  Acceptance criteria (agent-executable only):
  - [ ] `python3 -m json.tool configs/codex/hooks.json >/dev/null` exits 0 and is recorded in `evidence/task-5-config-parse.txt`.
  - [ ] `python3 -m json.tool configs/claude/hooks.settings.json >/dev/null` exits 0 and is recorded in `evidence/task-5-config-parse.txt`.
  - [ ] A Python assertion over parsed JSON confirms neither config contains `--enforce-search-routing`; Codex contains `--host codex --enable-llm-hints`; Claude contains `hook user-prompt --enable-llm-hints`.
  - [ ] `git diff -- configs/codex/hooks.json configs/claude/hooks.settings.json` contains no unrelated timeout/matcher/schema churn.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: config JSON parse and command assertions
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && bash -lc 'python3 -m json.tool configs/codex/hooks.json >/dev/null && python3 -m json.tool configs/claude/hooks.settings.json >/dev/null && python3 - <<"PY"
              import json
              codex=json.load(open("configs/codex/hooks.json"))
              claude=json.load(open("configs/claude/hooks.settings.json"))
              codex_text=json.dumps(codex)
              claude_text=json.dumps(claude)
              assert "--enforce-search-routing" not in codex_text, codex_text
              assert "--host codex --enable-llm-hints" in codex_text, codex_text
              assert "--enforce-search-routing" not in claude_text, claude_text
              assert "hook user-prompt --enable-llm-hints" in claude_text, claude_text
              PY' 2>&1 | tee evidence/task-5-config-parse.txt
    Expected: command exits 0; evidence file has no JSON parse error or assertion traceback.
    Evidence: evidence/task-5-config-parse.txt

  Scenario: no user-home mutation
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && git diff --name-status -- configs/codex/hooks.json configs/claude/hooks.settings.json | tee evidence/task-5-config-diff.txt
    Expected: output lists only the two template config files; no `~/.codex`, `~/.claude`, `.claude/`, or plugin cache paths are touched.
    Evidence: evidence/task-5-config-diff.txt
  ```

  Commit: NO | Message: `chore(config): enable hook templates for guarded hints` | Files: [configs/codex/hooks.json, configs/claude/hooks.settings.json, evidence/task-5-config-parse.txt, evidence/task-5-config-diff.txt]

- [ ] 6. Check contract/golden safety constraints before accepting a snapshot refresh

  What to do: Inspect the generated/diffed `response_contracts.golden.json` for deterministic placeholder use, secret redaction, and expected drift size. Confirm the golden delta is a consequence of existing contract-generation code, not a source-code workaround or host-specific output leak.
  Must NOT do: Do not approve Task 2 if the diff includes local absolute paths, unmasked tokens, unsupported hook stdout schema, or unrelated CLI/MCP tool changes.

  Parallelization: Can parallel: YES | Wave 1 | Blocks: [2] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:31-45` - local temp, repo, home, audit, and worker paths are replaced with stable placeholders.
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:356-407` - all nested values are normalized recursively.
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:380-392` - audit IDs, worker IDs, and git hashes receive stable placeholders.
  - Pattern:  `cmd/issueops/response_contract_golden_test.go:467-484` - strings are path-normalized and RFC3339 timestamps become `$TIMESTAMP`.
  - Pattern:  `cmd/issueops/contract.go:103-118` - compatibility contract required fields and verification commands.
  - Project:  `.issueops/TESTING.md:153-190` - response-contract golden scope and normalization expectations.
  - External: `https://go.dev/doc/effective_go#testing` - official Go guidance that package tests define executable behavior.

  Acceptance criteria (agent-executable only):
  - [ ] `evidence/task-6-contract-safety.txt` contains the current `git diff -- cmd/issueops/testdata/response_contracts.golden.json` plus secret/path/timestamp grep results.
  - [ ] The safety grep command exits 1 (no matches) for unnormalized local paths, secret-like values, and raw RFC3339 timestamps.
  - [ ] Task 6 records an explicit approve/stop decision for Task 2 based on reviewed diff scope and safety scan results.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: deterministic fixture safety scan
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && bash -lc '{ git diff -- cmd/issueops/testdata/response_contracts.golden.json; echo "--- safety grep ---"; if grep -En "/Users/|/var/folders|TOKEN=secret-value|secret-value|[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:" cmd/issueops/testdata/response_contracts.golden.json; then echo "unsafe_fixture_values_found"; exit 1; else echo "no unsafe fixture values"; fi; }' 2>&1 | tee evidence/task-6-contract-safety.txt
    Expected: command exits 0; evidence includes `no unsafe fixture values`.
    Evidence: evidence/task-6-contract-safety.txt

  Scenario: contract schema remains internally consistent
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && go run ./cmd/issueops contract check --json | tee evidence/task-6-contract-check.json && python3 - <<'PY'
              import json
              data=json.load(open('evidence/task-6-contract-check.json'))
              assert data['ok'] is True, data
              assert 'go test ./cmd/issueops -run Golden -count=1' in data.get('verification', []), data
              PY
    Expected: Python assertion exits 0; contract check returns `ok: true`.
    Evidence: evidence/task-6-contract-check.json
  ```

  Commit: NO | Message: `test(contract): verify response fixture safety` | Files: [evidence/task-6-contract-safety.txt, evidence/task-6-contract-check.json]

- [ ] 7. Run full baseline verification and prepare atomic commit

  What to do: Run final targeted and full verification commands, collect tmux manual-QA transcript, inspect final diff, and create one atomic commit only after all checks pass. If a full-suite failure remains outside `TestResponseContractsGolden`, stop and open a new red evidence file rather than hiding it with a golden update.
  Must NOT do: Do not skip packages, use cached results, commit unrelated local state, or declare done before F1-F4 approve.

  Parallelization: Can parallel: NO | Wave 3 | Blocks: [final] | Blocked by: [2, 3, 4, 5]

  References (executor has NO interview context - be exhaustive):
  - Project:  `.issueops/TESTING.md:96-121` - required Go verification commands and targeted golden update command.
  - Project:  `.issueops/COMMIT_POLICY.md:12-25` - Conventional Commit subject plus Lore body format.
  - Project:  `.issueops/COMMIT_POLICY.md:61-68` - Lore `Verify` and `Risk` must state actual verification and remaining risks without secrets.
  - Pattern:  `cmd/issueops/main.go:1365-1491` - self-verify already treats contract golden tests as a core verification step.
  - Test:     `cmd/issueops/testdata/response_contracts.golden.json` - generated fixture must be stable after full run.

  Acceptance criteria (agent-executable only):
  - [ ] `go test ./cmd/issueops -run Golden -count=1` exits 0 and is saved to `evidence/task-7-golden-final.txt`.
  - [ ] `go test ./... -count=1` exits 0 and is saved to `evidence/task-7-full-go-test.txt`.
  - [ ] `go vet ./...` exits 0 and is saved to `evidence/task-7-go-vet.txt`.
  - [ ] `go test -race ./... -count=1` exits 0 and is saved to `evidence/task-7-race.txt`.
  - [ ] `go build -o bin/issueops ./cmd/issueops` exits 0 and is saved to `evidence/task-7-build.txt`.
  - [ ] `git diff --check` exits 0 and is saved to `evidence/task-7-diff-check.txt`.
  - [ ] Final `git status --short` contains only intentional source/test/config/golden/plan/evidence changes before commit.

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: full baseline manual-QA transcript
    Tool:     tmux
    Steps:    cd /Users/user/Workspace/issueops && mkdir -p evidence && tmux new-session -d -s ah_task7_full 'cd /Users/user/Workspace/issueops && go test ./cmd/issueops -run Golden -count=1 && go test ./... -count=1' && sleep 10 && tmux capture-pane -pt ah_task7_full | tee evidence/task-7-full-tmux.txt; tmux kill-session -t ah_task7_full 2>/dev/null || true
    Expected: captured pane contains `ok issueops/cmd/issueops` and no `FAIL`; if the command is still running after 10 seconds, poll `tmux capture-pane -pt ah_task7_full` until it exits, then capture final PASS/FAIL and kill the session.
    Evidence: evidence/task-7-full-tmux.txt

  Scenario: commit hygiene preflight
    Tool:     bash
    Steps:    cd /Users/user/Workspace/issueops && bash -lc 'git diff --check && git status --short && git diff --name-status' 2>&1 | tee evidence/task-7-commit-hygiene.txt
    Expected: diff check passes; status/diff list only intentional files for this plan and no runtime-state directories.
    Evidence: evidence/task-7-commit-hygiene.txt
  ```

  Commit: YES | Message: `feat(hooks): guard source search and review hints` | Files: [cmd/issueops/hook_user_prompt.go, cmd/issueops/hook_user_prompt_test.go, cmd/issueops/testdata/response_contracts.golden.json, configs/claude/hooks.settings.json, configs/codex/hooks.json, internal/core/hook_prompt.go, internal/core/hook_prompt_test.go, internal/core/lifecycle_state.go, internal/core/lifecycle_state_test.go, plans/baseline-restore-2026-06-01.md]

## Final verification wave (MANDATORY - after all implementation tasks)
> Runs in PARALLEL. ALL must APPROVE. Surface results to the caller and wait for an explicit "okay" before declaring complete.
- [ ] F1. Plan compliance audit - every task done, every acceptance criterion met
- [ ] F2. Code quality review - diagnostics clean, idioms match, no dead code
- [ ] F3. Real manual QA - every QA scenario executed with evidence captured
- [ ] F4. Scope fidelity - nothing extra shipped beyond Must-Have, nothing Must-NOT-Have introduced

## Commit strategy
- One logical change per commit. Conventional Commits (`<type>(<scope>): <subject>` body + footer).
- Atomic: every commit builds and passes tests on its own.
- No "WIP" / "fix typo squash later" commits on the final branch - clean up before merge.
- Reference the plan file path in the final commit footer: `Plan: plans/baseline-restore-2026-06-01.md`.
- Use this Lore body shape for the final commit:
  ```text
  Lore:
  - Intent: Preserve guarded hook routing and restore the response-contract test baseline.
  - Why: The current full-test baseline is blocked by a stale response-contract golden after CodeGraph PreToolUse and opt-in Z.AI hint changes.
  - Changes:
    - Add/keep opt-in CodeGraph raw source-search enforcement and focused tests.
    - Add/keep opt-in Z.AI review hints and focused tests.
    - Refresh the generated response contract golden only after RED/GREEN review.
  - Verify: go test ./cmd/issueops -run Golden -count=1; go test ./... -count=1; go vet ./...; go test -race ./... -count=1; go build -o bin/issueops ./cmd/issueops
  - Risk: Medium; hook stdout contracts and generated golden fixtures are sensitive to host/runtime schema drift.

  Plan: plans/baseline-restore-2026-06-01.md
  ```

## Success criteria
- All Must-Have shipped; all QA scenarios pass with captured evidence; F1-F4 approved; commit history clean.
