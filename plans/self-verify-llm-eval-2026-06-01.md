# Self-Verify Opt-In LLM Evaluation via Z.AI Coding Plan

## TL;DR
> Summary:      Add opt-in LLM evaluation to `issueops self-verify` by invoking `Z.AI Coding Plan` after the deterministic loop, without changing default deterministic behavior.
> Deliverables:
> - `--llm-eval`, `--llm-eval-mode=advisory|gate`, `--model`, and `internal default timeout` across CLI usage, tests, and MCP parity.
> - Structured `llm_eval` JSON with bounded prompt/output/error handling and fake Z.AI tests/QA.
> - Narrow documentation exception for opt-in post-loop LLM evaluation, preserving the “no model calls on default/hook paths” rule.
> Effort:       Medium
> Risk:         Medium - current worktree already has partial uncommitted implementation and this feature touches CLI, MCP schema, goldens, docs, and process execution semantics.

## Scope
### Must have
- Preserve default `self-verify` output and deterministic behavior: no `zai`, no model/network dependency, and no `llm_eval` field unless `--llm-eval` or matching MCP argument is explicitly enabled.
- Invoke exactly `Z.AI Coding Plan <bounded-evidence-packet>` for LLM evaluation, using configurable command path and timeout; tests and QA must use fake Z.AI scripts only.
- Return structured `llm_eval` in immediate JSON output with fields for `ok`, `mode`, `score`, `summary`, `blockers`, `risks`, `recommended_next_actions`, `evidence_packet_bytes`, and bounded `error`.
- Make advisory mode the default: malformed output or `zai` failure is recorded in `llm_eval` and does not fail deterministic self-verify when deterministic self-verify passed.
- Support gate mode: LLM `ok=false`, score below target, blockers, command failure, timeout, or parse failure make the overall self-verify result not OK and return a gate error while still emitting structured JSON when `--json` is used.
- Keep LLM evaluation once-after-deterministic-loop; do not add LLM calls inside the 10-iteration self-verify loop.
- Keep CLI and MCP argument/result contracts aligned and update contract/golden fixtures.
- Update `.issueops/` docs to describe this as a narrow opt-in exception, not a general harness-owned LLM-advisor surface.
- Capture tmux QA artifacts under `evidence/`.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- Must not call real `zai` in tests or automated QA; use fake scripts with deterministic JSON.
- Must not add `--dangerously-skip-permissions` or any other implicit `zai` flags beyond `-p` unless the user explicitly approves a new requirement.
- Must not change deterministic scoring, minimum 10-iteration rule, candidate export, history, compare, or promote semantics except where the immediate `self-verify` result records opt-in `llm_eval`.
- Must not persist raw LLM prompt/output or secrets in state, logs, docs, tests, fixtures, or MCP responses.
- Must not make hooks, native install/update, daemon status, or default MCP tool calls invoke `zai`.
- Must not overwrite or revert unrelated dirty worktree changes; current dirty files include code, goldens, adapters, docs plans, and untracked `cmd/issueops/self_verify_llm_eval.go`.
- Must not reimplement LLM Wiki or broader agent-advisor behavior.

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- Test decision: TDD + Go `testing`; each implementation task starts by adding/fixing the failing test or golden assertion, captures RED evidence against the pre-fix state or temporary clean worktree, then captures GREEN evidence after code changes.
- QA policy: every task has agent-executed scenarios using fake Z.AI where LLM evaluation is involved.
- Evidence: `evidence/task-<N>-<slug>.<ext>`

## Execution strategy
### Parallel execution waves
> Target 5-8 tasks per wave. <3 per wave (except final) = under-splitting.
> Extract shared dependencies as Wave-1 tasks to maximize parallelism.

This feature is centered on `cmd/issueops/main.go` plus one untracked companion file, so implementation parallelism must favor file ownership over maximum worker count to avoid conflicts. Documentation can run in parallel after Task 1 records the agreed contract.

Wave 1 (no dependencies):
- Task 1: Baseline worktree, schema/default-off contract, and TDD guard

Wave 2 (after Wave 1):
- Task 2: depends [1] - Harden fake Z.AI execution, prompt budget, parser, and advisory/gate semantics
- Task 5: depends [1] - Document narrow opt-in architecture exception

Wave 3 (after Wave 2):
- Task 3: depends [2] - Finish CLI flags, usage, save-state boundary, and golden coverage
- Task 4: depends [2] - Add MCP `self_verify` LLM-eval parity and contract coverage

Wave 4 (after Wave 3):
- Task 6: depends [3, 4, 5] - Run fake Z.AI tmux QA and cleanup receipts

Critical path: Task 1 -> Task 2 -> Task 3 -> Task 6

### Dependency matrix
| Task | Depends on | Blocks | Can parallelize with |
|------|------------|--------|----------------------|
| 1    | none       | 2, 5   | none                 |
| 2    | 1          | 3, 4   | 5                    |
| 3    | 2          | 6      | 4                    |
| 4    | 2          | 6      | 3                    |
| 5    | 1          | 6      | 2                    |
| 6    | 3, 4, 5    | final  | none                 |

## Todos
> Implementation + Test = ONE task. Never separate.
> Every task MUST have: References + Acceptance Criteria + QA Scenarios + Commit.

- [ ] 1. Baseline worktree, schema/default-off contract, and TDD guard

  What to do: Capture current dirty worktree status before touching anything; preserve existing partial uncommitted work instead of reverting it. Lock the result schema and default-off behavior with tests first. Treat the untracked `cmd/issueops/self_verify_llm_eval.go` as current work-in-progress to review, not as a clean accepted implementation. Ensure `SelfAugmentResult.LLMEval` remains `omitempty` and no default JSON contains `llm_eval`.
  Must NOT do: Do not run real `zai`; do not edit docs or MCP schema in this task; do not stage unrelated dirty files.

  Parallelization: Can parallel: NO | Wave 1 | Blocks: [2, 5] | Blocked by: []

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/main.go:646-671` - current partial CLI flags call `applySelfVerifyLLMEval`; verify defaults are opt-in and no LLM path runs when `--llm-eval` is absent.
  - API/Type: `cmd/issueops/main.go:1030-1043` - `SelfAugmentResult` currently has `LLMEval *SelfVerifyLLMEvalResult` with `json:"llm_eval,omitempty"`.
  - API/Type: `cmd/issueops/self_verify_llm_eval.go:17-35` - current untracked option/result DTO definitions for the LLM eval surface.
  - Test:     `cmd/issueops/self_augment_summary_test.go:956-980` - existing partial default omission and advisory success tests; extend rather than duplicate.
  - Test:     `cmd/issueops/contract_golden_test.go:14-28` - golden helper and `-update` convention for usage goldens.
  - Project:  `.issueops/TESTING.md:125-149` - deterministic/fake tests are required for process and model-like behavior.

  Acceptance criteria (agent-executable only):
  - [ ] Baseline dirty state captured before edits:
    `mkdir -p evidence && git status --short > evidence/task-1-self-verify-llm-eval-baseline-status.txt && git diff -- cmd/issueops/main.go cmd/issueops/self_verify_llm_eval.go cmd/issueops/self_augment_summary_test.go > evidence/task-1-self-verify-llm-eval-baseline-diff.patch`
  - [ ] TDD RED captured by applying only the schema/default tests to a temporary clean `HEAD` worktree and running:
    `go test ./cmd/issueops -run 'TestSelfVerifyLLMEvalDefaultOmittedFromJSON|TestSelfVerifyLLMEvalAdvisorySuccess' -count=1`
    with failure output saved to `evidence/task-1-self-verify-llm-eval-red.txt`.
  - [ ] GREEN captured in the working tree:
    `go test ./cmd/issueops -run 'TestSelfVerifyLLMEvalDefaultOmittedFromJSON|TestSelfVerifyLLMEvalAdvisorySuccess' -count=1 | tee evidence/task-1-self-verify-llm-eval-green.txt`
  - [ ] Static contract check passes:
    `python3 - <<'PY'
from pathlib import Path
main = Path('cmd/issueops/main.go').read_text()
assert 'LLMEval             *SelfVerifyLLMEvalResult    `json:"llm_eval,omitempty"`' in main
assert 'llm-eval' in main
print('ok')
PY`

  QA scenarios (MANDATORY - task incomplete without these):
  > Name the exact tool AND its exact invocation - not "verify it works". Browser use: use Chrome to drive the page; if Chrome is not available, download and use agent-browser (https://github.com/vercel-labs/agent-browser). Computer use: OS-level GUI automation for a non-browser desktop app.
  ```
  Scenario: Default JSON omits llm_eval
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestSelfVerifyLLMEvalDefaultOmittedFromJSON -count=1 | tee evidence/task-1-self-verify-llm-eval-default.txt
    Expected: command exits 0 and test output includes PASS; the test fails if marshalled default JSON contains llm_eval.
    Evidence: evidence/task-1-self-verify-llm-eval-default.txt

  Scenario: Advisory fake Z.AI result is structured
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestSelfVerifyLLMEvalAdvisorySuccess -count=1 | tee evidence/task-1-self-verify-llm-eval-advisory.txt
    Expected: command exits 0; the fake reviewer score, risks, next actions, and evidence_packet_bytes are preserved in llm_eval.
    Evidence: evidence/task-1-self-verify-llm-eval-advisory.txt
  ```

  Commit: YES | Message: `test(self-verify): lock llm eval contract` | Files: [`cmd/issueops/main.go`, `cmd/issueops/self_verify_llm_eval.go`, `cmd/issueops/self_augment_summary_test.go`, `evidence/task-1-self-verify-llm-eval-*.txt`, `evidence/task-1-self-verify-llm-eval-baseline-diff.patch`]

- [ ] 2. Harden fake Z.AI execution, prompt budget, parser, and advisory/gate semantics

  What to do: Make `cmd/issueops/self_verify_llm_eval.go` execute `Z.AI Coding Plan` exactly, with `context.WithTimeout`, bounded evidence packet, strict single-JSON-object decoding, bounded error strings, and deterministic advisory/gate behavior. Update fake Z.AI tests so the first argument must be `-p`; remove the current `--dangerously-skip-permissions` expectation from `cmd/issueops/self_augment_summary_test.go:1016-1024` and from execution at `cmd/issueops/self_verify_llm_eval.go:91-94`. Add missing tests for command failure, timeout, extra JSON values, prompt budget, default timeout, empty command fallback, and gate failure on parse/command errors.
  Must NOT do: Do not add unrequested prompt configurability; do not include raw full self-verify runs in the prompt; do not loosen JSON parsing to accept prose around JSON.

  Parallelization: Can parallel: YES | Wave 2 | Blocks: [3, 4] | Blocked by: [1]

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `internal/core/draft_wiki_queue.go:218-223` - existing safe `exec.CommandContext(ctx, Z.AICommand, "-p", prompt).CombinedOutput()` pattern.
  - Pattern:  `internal/core/draft_wiki_test.go:244-319` - fake Z.AI script pattern that checks `$1 = -p` and records prompt content.
  - API/Type: `cmd/issueops/self_verify_llm_eval.go:14-35` - budget constants and result DTO fields.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:64-112` - current apply function; revise command argv, timeout, error, and gate handling here.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:115-140` - current bounded prompt builder; keep bounded and focused on summary/last run.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:143-155` - strict decoder rejects extra JSON values.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:168-188` - gate semantics currently flips OK and termination eligibility.
  - Test:     `cmd/issueops/self_augment_summary_test.go:982-1007` - malformed output and gate failure tests to expand.
  - External: local `Z.AI --help` evidence from `$HOME/.local/bin/Z.AI` shows `-p, --print` and `--print-timeout`, so this feature should pass prompt via `-p` and keep timeout owned by harness.

  Acceptance criteria (agent-executable only):
  - [ ] RED tests captured before implementation changes for all new execution edge cases:
    `go test ./cmd/issueops -run 'TestSelfVerifyLLMEvalCommandFailureIsStructured|TestSelfVerifyLLMEvalRejectsExtraJSON|TestSelfVerifyLLMEvalTimeoutIsBounded|TestSelfVerifyLLMEvalPromptIsBounded|TestSelfVerifyLLMEvalGateFailsOnMalformedOutput|TestSelfVerifyLLMEvalUsesDashPOnly' -count=1 | tee evidence/task-2-self-verify-llm-eval-red.txt`
  - [ ] GREEN tests pass after implementation:
    `go test ./cmd/issueops -run 'TestSelfVerifyLLMEvalMalformedOutputIsStructured|TestSelfVerifyLLMEvalGateFailsOnBlocker|TestSelfVerifyLLMEvalCommandFailureIsStructured|TestSelfVerifyLLMEvalRejectsExtraJSON|TestSelfVerifyLLMEvalTimeoutIsBounded|TestSelfVerifyLLMEvalPromptIsBounded|TestSelfVerifyLLMEvalGateFailsOnMalformedOutput|TestSelfVerifyLLMEvalUsesDashPOnly' -count=1 | tee evidence/task-2-self-verify-llm-eval-green.txt`
  - [ ] Static argv check proves no implicit permission bypass:
    `python3 - <<'PY'
from pathlib import Path
src = Path('cmd/issueops/self_verify_llm_eval.go').read_text()
assert 'exec.CommandContext(ctx, Z.AICommand, "-p", evidencePacket)' in src
assert '--dangerously-skip-permissions' not in src
print('ok')
PY`
  - [ ] Error and evidence budgets are asserted by tests and static constants remain present:
    `python3 - <<'PY'
from pathlib import Path
src = Path('cmd/issueops/self_verify_llm_eval.go').read_text()
assert 'selfVerifyLLMEvalEvidenceBudgetBytes' in src
assert 'selfVerifyLLMEvalErrorBudgetBytes' in src
assert 'decoder.Decode(&extra)' in src
print('ok')
PY`

  QA scenarios (MANDATORY - task incomplete without these):
  ```
  Scenario: Malformed fake Z.AI is advisory-only structured failure
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestSelfVerifyLLMEvalMalformedOutputIsStructured -count=1 | tee evidence/task-2-self-verify-llm-eval-malformed.txt
    Expected: command exits 0; result remains OK in advisory mode and llm_eval.ok=false with bounded parse error.
    Evidence: evidence/task-2-self-verify-llm-eval-malformed.txt

  Scenario: Gate mode converts fake Z.AI blocker into self-verify gate failure
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestSelfVerifyLLMEvalGateFailsOnBlocker -count=1 | tee evidence/task-2-self-verify-llm-eval-gate.txt
    Expected: command exits 0; test asserts overall OK=false, termination_eligible=false, and error contains LLM evaluation gate failed.
    Evidence: evidence/task-2-self-verify-llm-eval-gate.txt
  ```

  Commit: YES | Message: `fix(self-verify): harden Z.AI eval execution` | Files: [`cmd/issueops/self_verify_llm_eval.go`, `cmd/issueops/self_augment_summary_test.go`, `evidence/task-2-self-verify-llm-eval-*.txt`]

- [ ] 3. Finish CLI flags, usage, save-state boundary, and golden coverage

  What to do: Complete the CLI surface so `--llm-eval`, `--llm-eval-mode`, `--model`, and `internal default timeout` are discoverable, validated, covered by tests, and represented in usage goldens. Keep `--save-state` compact-summary behavior unchanged unless a test explicitly proves current summaries need a boolean marker; immediate JSON should contain `llm_eval`, history/compare/promote should not store raw LLM review data.
  Must NOT do: Do not make `--llm-eval-mode` meaningful without `--llm-eval` beyond validation; do not silently run LLM eval during `history`, `compare`, `promote`, or `candidates`.

  Parallelization: Can parallel: YES | Wave 3 | Blocks: [6] | Blocked by: [2]

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/main.go:628-687` - `runSelfVerify` flag parsing, mode validation, post-loop LLM eval call, save-state, JSON output ordering.
  - Pattern:  `cmd/issueops/main.go:1477-1479` - deterministic self-verify returns before LLM eval; preserve once-after-loop placement.
  - Pattern:  `internal/adapter/cli/usage.go:76-88` - current usage string includes LLM flags but omits `internal default timeout`.
  - Test:     `cmd/issueops/testdata/usage.golden.txt:38` - usage golden currently omits `internal default timeout`.
  - Test:     `cmd/issueops/contract_golden_test.go:14-28` - golden file comparison/update pattern.
  - Test:     `cmd/issueops/self_augment_summary_test.go:1009-1014` - current invalid mode test.
  - Project:  `.issueops/CONVENTIONS.md:78-86` - CLI/MCP schemas and JSON outputs must stay host-neutral and golden-covered.

  Acceptance criteria (agent-executable only):
  - [ ] RED captured for CLI usage/flag behavior before fixes:
    `go test ./cmd/issueops -run 'TestUsageGolden|TestRunSelfVerifyRejectsUnknownLLMEvalMode|TestRunSelfVerifyLLMEvalAdvisoryFakeLLM|TestRunSelfVerifyLLMEvalGateFakeLLM|TestRunSelfVerifyLLMEvalTimeoutFlag' -count=1 | tee evidence/task-3-self-verify-llm-eval-red.txt`
  - [ ] GREEN captured after fixes:
    `go test ./cmd/issueops -run 'TestUsageGolden|TestRunSelfVerifyRejectsUnknownLLMEvalMode|TestRunSelfVerifyLLMEvalAdvisoryFakeLLM|TestRunSelfVerifyLLMEvalGateFakeLLM|TestRunSelfVerifyLLMEvalTimeoutFlag' -count=1 | tee evidence/task-3-self-verify-llm-eval-green.txt`
  - [ ] Usage source and golden both expose timeout:
    `python3 - <<'PY'
from pathlib import Path
for path in ['internal/adapter/cli/usage.go','cmd/issueops/testdata/usage.golden.txt']:
    text = Path(path).read_text()
    assert 'internal default timeout' in text, path
print('ok')
PY`
  - [ ] Full command package still passes:
    `go test ./cmd/issueops -count=1 | tee evidence/task-3-self-verify-llm-eval-cmd-tests.txt`

  QA scenarios (MANDATORY - task incomplete without these):
  ```
  Scenario: CLI advisory fake Z.AI emits llm_eval in JSON
    Tool:     bash
    Steps:    go build -o bin/issueops ./cmd/issueops && tmp="$(mktemp -d)" && cat > "$tmp/Z.AI" <<'SH'
#!/bin/sh
[ "$1" = "-p" ] || { echo "expected -p" >&2; exit 9; }
printf '{"ok":true,"score":98,"summary":"fake advisory ok","blockers":[],"risks":[],"recommended_next_actions":[]}\n'
SH
chmod +x "$tmp/Z.AI" && ./bin/issueops self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval --model "$tmp/Z.AI" internal default timeout=2s --json > evidence/task-3-self-verify-llm-eval-cli.json && python3 - <<'PY'
import json
from pathlib import Path
obj = json.loads(Path('evidence/task-3-self-verify-llm-eval-cli.json').read_text())
assert obj['llm_eval']['ok'] is True
assert obj['llm_eval']['mode'] == 'advisory'
assert obj['llm_eval']['score'] == 98
print('ok')
PY
    Expected: command exits 0; JSON has llm_eval.mode=advisory, llm_eval.ok=true, score 98.
    Evidence: evidence/task-3-self-verify-llm-eval-cli.json

  Scenario: CLI invalid mode fails before fake Z.AI is needed
    Tool:     bash
    Steps:    set +e; ./bin/issueops self-verify --llm-eval --llm-eval-mode=unknown --json > evidence/task-3-self-verify-llm-eval-invalid.out 2>&1; code=$?; set -e; python3 - "$code" <<'PY'
import sys
from pathlib import Path
code = int(sys.argv[1])
text = Path('evidence/task-3-self-verify-llm-eval-invalid.out').read_text()
assert code != 0, code
assert 'llm-eval-mode' in text
print('ok')
PY
    Expected: assertion exits 0 because the CLI command exited nonzero and output mentions llm-eval-mode.
    Evidence: evidence/task-3-self-verify-llm-eval-invalid.out
  ```

  Commit: YES | Message: `feat(cli): expose self-verify llm eval flags` | Files: [`cmd/issueops/main.go`, `cmd/issueops/self_verify_llm_eval.go`, `cmd/issueops/self_augment_summary_test.go`, `internal/adapter/cli/usage.go`, `cmd/issueops/testdata/usage.golden.txt`, `evidence/task-3-self-verify-llm-eval-*`]

- [ ] 4. Add MCP `self_verify` LLM-eval parity and contract coverage

  What to do: Add `llm_eval`, `llm_eval_mode`, `model`, and `llm_eval_timeout` to the MCP `self_verify` input schema and handler. Route MCP through the same LLM eval function as CLI after deterministic `selfVerify(...)`. Validate durations and mode exactly as CLI. Preserve default MCP behavior when `llm_eval` is absent. Update MCP tool and response contract goldens.
  Must NOT do: Do not create a separate MCP-only DTO or schema drift; do not return JSON-RPC errors for advisory LLM parse/command failures; gate failures should return the structured self-verify payload consistently with existing self-verification gate handling unless existing MCP contract tests prove otherwise.

  Parallelization: Can parallel: YES | Wave 3 | Blocks: [6] | Blocked by: [2]

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/main.go:4803-4811` - current MCP `self_verify` tool schema lacks LLM args.
  - Pattern:  `cmd/issueops/main.go:5120-5135` - current MCP handler calls deterministic `selfVerify` and saves compact summary.
  - Pattern:  `cmd/issueops/main.go:646-671` - CLI LLM eval option parsing to mirror.
  - Pattern:  `cmd/issueops/main.go:4580-4595` - MCP tool schema list is serialized into goldens.
  - Test:     `cmd/issueops/response_contract_golden_test.go:18-207` - response contract and MCP tool golden assertions.
  - Test:     `cmd/issueops/testdata/mcp_tools.golden.json` - update to include new self_verify arguments.
  - Test:     `cmd/issueops/testdata/response_contracts.golden.json` - update if `llm_eval` changes response contracts.
  - Project:  `.issueops/CAUTIONS.md:78-85` - avoid CLI/MCP schema drift.
  - Project:  `.issueops/ADR.md:290-307` - CLI/MCP/worker DTOs should remain shared and host-neutral.

  Acceptance criteria (agent-executable only):
  - [ ] RED captured for MCP schema/handler tests:
    `go test ./cmd/issueops -run 'TestMCPToolsGolden|TestResponseContractsGolden|TestMCPSelfVerifyLLMEvalDefaultOmitted|TestMCPSelfVerifyLLMEvalAdvisoryFakeLLM|TestMCPSelfVerifyLLMEvalGateFakeLLM|TestMCPSelfVerifyLLMEvalInvalidTimeout' -count=1 | tee evidence/task-4-self-verify-llm-eval-red.txt`
  - [ ] GREEN captured after implementation:
    `go test ./cmd/issueops -run 'TestMCPToolsGolden|TestResponseContractsGolden|TestMCPSelfVerifyLLMEvalDefaultOmitted|TestMCPSelfVerifyLLMEvalAdvisoryFakeLLM|TestMCPSelfVerifyLLMEvalGateFakeLLM|TestMCPSelfVerifyLLMEvalInvalidTimeout' -count=1 | tee evidence/task-4-self-verify-llm-eval-green.txt`
  - [ ] MCP golden contains all new inputs:
    `python3 - <<'PY'
import json
from pathlib import Path
tools = json.loads(Path('cmd/issueops/testdata/mcp_tools.golden.json').read_text())
self_verify = next(t for t in tools if t['name'] == 'self_verify')
props = self_verify['inputSchema']['properties']
for key in ['llm_eval','llm_eval_mode','model','llm_eval_timeout']:
    assert key in props, key
print('ok')
PY`
  - [ ] Full command package still passes after golden updates:
    `go test ./cmd/issueops -count=1 | tee evidence/task-4-self-verify-llm-eval-cmd-tests.txt`

  QA scenarios (MANDATORY - task incomplete without these):
  ```
  Scenario: MCP self_verify default remains deterministic
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestMCPSelfVerifyLLMEvalDefaultOmitted -count=1 | tee evidence/task-4-self-verify-llm-eval-mcp-default.txt
    Expected: command exits 0; MCP payload omits llm_eval when llm_eval argument is absent.
    Evidence: evidence/task-4-self-verify-llm-eval-mcp-default.txt

  Scenario: MCP self_verify advisory fake Z.AI returns structured llm_eval
    Tool:     bash
    Steps:    go test ./cmd/issueops -run TestMCPSelfVerifyLLMEvalAdvisoryFakeLLM -count=1 | tee evidence/task-4-self-verify-llm-eval-mcp-advisory.txt
    Expected: command exits 0; MCP payload contains llm_eval.mode=advisory and fake score.
    Evidence: evidence/task-4-self-verify-llm-eval-mcp-advisory.txt
  ```

  Commit: YES | Message: `feat(mcp): mirror self-verify llm eval options` | Files: [`cmd/issueops/main.go`, `cmd/issueops/self_verify_llm_eval.go`, `cmd/issueops/response_contract_golden_test.go`, `cmd/issueops/testdata/mcp_tools.golden.json`, `cmd/issueops/testdata/response_contracts.golden.json`, `evidence/task-4-self-verify-llm-eval-*`]

- [ ] 5. Document narrow opt-in architecture exception

  What to do: Update project docs to explicitly allow only this post-loop, opt-in `self-verify --llm-eval` path to invoke `Z.AI Coding Plan`, while preserving the default no-model-call invariant. Clarify that hooks and default install/update/native paths must not call `zai`; fake Z.AI is required in tests; prompt/output are bounded and must not persist raw secrets; CLI and MCP options are equivalent.
  Must NOT do: Do not broaden this into a generic LLM advisor, worker policy, LLM Wiki clone, or default self-verify behavior.

  Parallelization: Can parallel: YES | Wave 2 | Blocks: [6] | Blocked by: [1]

  References (executor has NO interview context - be exhaustive):
  - Project:  `.issueops/ADR.md:332-350` - current ADR says the harness does not call models; add a narrow exception rather than contradicting it.
  - Project:  `.issueops/ARCHITECTURE.md:110-116` - current architecture lists existing `Z.AI Coding Plan` surfaces; add self-verify opt-in post-loop as bounded exception.
  - Project:  `.issueops/OPERATIONS.md:176-185` - self-verify CLI examples; add advisory/gate examples with fake/test caution.
  - Project:  `.issueops/OPERATIONS.md:227-231` - progress/save-state/history/compare behavior; clarify `llm_eval` immediate JSON vs compact state boundary.
  - Project:  `.issueops/CAUTIONS.md:101-108` - self-verify drift cautions; add no LLM inside the loop and no lower iteration minimum.
  - Project:  `.issueops/CAUTIONS.md:122-132` - no LLM Wiki reimplementation and hook critical path caution; preserve this boundary.
  - Project:  `.issueops/TESTING.md:125-149` - fake tests policy; add fake Z.AI requirement for self-verify LLM eval.
  - Project:  `.issueops/CONVENTIONS.md:78-86` - schema/golden drift rule; mention CLI/MCP LLM eval parity if needed.

  Acceptance criteria (agent-executable only):
  - [ ] Documentation token check passes:
    `python3 - <<'PY'
from pathlib import Path
checks = {
  '.issueops/ADR.md': ['self-verify --llm-eval', 'opt-in', 'Z.AI Coding Plan'],
  '.issueops/ARCHITECTURE.md': ['self-verify --llm-eval', 'post-loop'],
  '.issueops/OPERATIONS.md': ['--llm-eval', '--llm-eval-mode=gate', 'internal default timeout'],
  '.issueops/CAUTIONS.md': ['self-verify --llm-eval', 'hook', 'fake Z.AI'],
  '.issueops/TESTING.md': ['fake Z.AI', 'llm_eval'],
}
for path, needles in checks.items():
    text = Path(path).read_text()
    for needle in needles:
        assert needle in text, f'{path}: missing {needle}'
print('ok')
PY`
  - [ ] Docs diff contains only LLM-eval boundary updates and no unrelated cleanup:
    `git diff -- .issueops/ADR.md .issueops/ARCHITECTURE.md .issueops/OPERATIONS.md .issueops/CAUTIONS.md .issueops/TESTING.md > evidence/task-5-self-verify-llm-eval-docs.diff`
  - [ ] No docs mention default self-verify calling `zai`:
    `python3 - <<'PY'
from pathlib import Path
for path in ['.issueops/ADR.md','.issueops/ARCHITECTURE.md','.issueops/OPERATIONS.md','.issueops/CAUTIONS.md','.issueops/TESTING.md']:
    text = Path(path).read_text().lower()
    assert 'default self-verify calls Z.AI' not in text
print('ok')
PY`

  QA scenarios (MANDATORY - task incomplete without these):
  ```
  Scenario: Operations docs include exact advisory and gate commands
    Tool:     bash
    Steps:    python3 - <<'PY' | tee evidence/task-5-self-verify-llm-eval-operations-docs.txt
from pathlib import Path
text = Path('.issueops/OPERATIONS.md').read_text()
for needle in ['issueops self-verify --llm-eval', '--llm-eval-mode=gate', '--model', 'internal default timeout']:
    assert needle in text, needle
print('ok')
PY
    Expected: command exits 0 and prints ok.
    Evidence: evidence/task-5-self-verify-llm-eval-operations-docs.txt

  Scenario: Architecture docs preserve default no-model-call invariant
    Tool:     bash
    Steps:    python3 - <<'PY' | tee evidence/task-5-self-verify-llm-eval-boundary-docs.txt
from pathlib import Path
text = Path('.issueops/ARCHITECTURE.md').read_text().lower()
assert 'opt-in' in text
assert 'post-loop' in text
assert 'hook' in Path('.issueops/CAUTIONS.md').read_text().lower()
print('ok')
PY
    Expected: command exits 0 and prints ok.
    Evidence: evidence/task-5-self-verify-llm-eval-boundary-docs.txt
  ```

  Commit: YES | Message: `docs(self-verify): document opt-in llm eval boundary` | Files: [`.issueops/ADR.md`, `.issueops/ARCHITECTURE.md`, `.issueops/OPERATIONS.md`, `.issueops/CAUTIONS.md`, `.issueops/TESTING.md`, `.issueops/CONVENTIONS.md`, `evidence/task-5-self-verify-llm-eval-*`]

- [ ] 6. Run fake Z.AI tmux QA and cleanup receipts

  What to do: Build the local CLI and run end-to-end tmux QA using fake Z.AI only. Capture advisory success, default no-LLM behavior, gate failure behavior, malformed advisory behavior, and cleanup proof. Use the real compiled binary but fake Z.AI scripts. Record exact stdout/stderr and JSON artifacts under `evidence/`.
  Must NOT do: Do not call real `zai`; do not leave tmux sessions running; do not commit generated binaries.

  Parallelization: Can parallel: NO | Wave 4 | Blocks: [final] | Blocked by: [3, 4, 5]

  References (executor has NO interview context - be exhaustive):
  - Pattern:  `cmd/issueops/main.go:646-671` - CLI flag behavior to exercise.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:91-112` - fake Z.AI process execution path to exercise.
  - Pattern:  `cmd/issueops/self_verify_llm_eval.go:168-188` - gate failure path to exercise.
  - Test:     `cmd/issueops/self_augment_summary_test.go:1016-1024` - fake Z.AI helper behavior to mirror after Task 2 updates it to `-p` only.
  - Project:  `.issueops/TESTING.md:193-205` - completion evidence and QA requirements.
  - External: local `Z.AI --help` evidence says `-p, --print`; QA fake must reject any argv shape other than `-p <prompt>`.

  Acceptance criteria (agent-executable only):
  - [ ] Build succeeds:
    `go build -o bin/issueops ./cmd/issueops | tee evidence/task-6-self-verify-llm-eval-build.txt`
  - [ ] Fake-`zai` tmux script exits 0 and writes all expected artifacts:
    `bash evidence/run-task-6-self-verify-llm-eval-tmux.sh | tee evidence/task-6-self-verify-llm-eval-tmux-run.txt`
  - [ ] Artifact JSON checks pass:
    `python3 - <<'PY'
import json
from pathlib import Path
adv = json.loads(Path('evidence/task-6-self-verify-llm-eval-advisory.json').read_text())
assert adv['llm_eval']['ok'] is True
assert adv['llm_eval']['mode'] == 'advisory'
default = json.loads(Path('evidence/task-6-self-verify-llm-eval-default.json').read_text())
assert 'llm_eval' not in default
gate = json.loads(Path('evidence/task-6-self-verify-llm-eval-gate.json').read_text())
assert gate['ok'] is False
assert gate['llm_eval']['mode'] == 'gate'
malformed = json.loads(Path('evidence/task-6-self-verify-llm-eval-malformed.json').read_text())
assert malformed['ok'] is True
assert malformed['llm_eval']['ok'] is False
assert 'parse' in malformed['llm_eval']['error'].lower()
print('ok')
PY`
  - [ ] Cleanup proof has no remaining `io-sv-llm-*` sessions:
    `tmux ls 2>/dev/null | python3 -c "import sys; data=sys.stdin.read(); assert 'io-sv-llm-' not in data; print('ok')" | tee evidence/task-6-self-verify-llm-eval-tmux-cleanup.txt`

  QA scenarios (MANDATORY - task incomplete without these):
  ```
  Scenario: tmux advisory/default/malformed/gate e2e with fake Z.AI
    Tool:     tmux
    Steps:    cat > evidence/run-task-6-self-verify-llm-eval-tmux.sh <<'SH'
set -euo pipefail
mkdir -p evidence
tmp="$(mktemp -d)"
cat > "$tmp/Z.AI" <<'ZAI'
#!/bin/sh
[ "$1" = "-p" ] || { echo "expected -p" >&2; exit 9; }
case "${ZAI_MODE:-advisory}" in
  advisory) printf '{"ok":true,"score":98,"summary":"fake advisory ok","blockers":[],"risks":[],"recommended_next_actions":[]}\n' ;;
  gate) printf '{"ok":false,"score":40,"summary":"blocked","blockers":["missing QA"],"risks":[],"recommended_next_actions":["run QA"]}\n' ;;
  malformed) printf 'not-json\n' ;;
  *) echo "unknown ZAI_MODE" >&2; exit 10 ;;
esac
ZAI
chmod +x "$tmp/Z.AI"
run_session() {
  session="$1"; shift
  tmux new-session -d -s "$session" "cd '$PWD' && $*; tmux wait-for -S ${session}-done"
  tmux wait-for "${session}-done"
  tmux kill-session -t "$session" 2>/dev/null || true
}
run_session io-sv-llm-default "./bin/issueops self-verify --iterations=10 --seed=100 --target-score=95 --json > evidence/task-6-self-verify-llm-eval-default.json 2> evidence/task-6-self-verify-llm-eval-default.err"
run_session io-sv-llm-advisory "ZAI_MODE=advisory ./bin/issueops self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval --model '$tmp/Z.AI' internal default timeout=2s --json > evidence/task-6-self-verify-llm-eval-advisory.json 2> evidence/task-6-self-verify-llm-eval-advisory.err"
run_session io-sv-llm-malformed "ZAI_MODE=malformed ./bin/issueops self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval --model '$tmp/Z.AI' internal default timeout=2s --json > evidence/task-6-self-verify-llm-eval-malformed.json 2> evidence/task-6-self-verify-llm-eval-malformed.err"
set +e
run_session io-sv-llm-gate "ZAI_MODE=gate ./bin/issueops self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval --llm-eval-mode=gate --model '$tmp/Z.AI' internal default timeout=2s --json > evidence/task-6-self-verify-llm-eval-gate.json 2> evidence/task-6-self-verify-llm-eval-gate.err"
set -e
SH
bash evidence/run-task-6-self-verify-llm-eval-tmux.sh
    Expected: script exits 0; four JSON artifacts exist; default omits llm_eval, advisory succeeds, malformed advisory records structured error, gate records ok=false.
    Evidence: evidence/task-6-self-verify-llm-eval-tmux-run.txt

  Scenario: tmux cleanup proof
    Tool:     bash
    Steps:    tmux ls 2>/dev/null | python3 -c "import sys; data=sys.stdin.read(); assert 'io-sv-llm-' not in data; print('ok')" | tee evidence/task-6-self-verify-llm-eval-cleanup.txt
    Expected: command exits 0 and prints ok.
    Evidence: evidence/task-6-self-verify-llm-eval-cleanup.txt
  ```

  Commit: NO | Message: `n/a` | Files: [`evidence/run-task-6-self-verify-llm-eval-tmux.sh`, `evidence/task-6-self-verify-llm-eval-*`]

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
- Reference the plan file path in the final commit footer: `Plan: plans/self-verify-llm-eval-2026-06-01.md`.

## Success criteria
- All Must-Have shipped; all QA scenarios pass with captured evidence; F1-F4 approved; commit history clean.

## Reviewer fix notes
- Strict LLM JSON parsing now rejects unknown fields.
- Command failure is covered by a fake Z.AI non-zero exit test.
- Large evidence remains valid JSON by wrapping bounded evidence under `evidence_json`.
- `llm_eval.evidence_packet_bytes` reports the actual bounded prompt bytes sent to Z.AI.
- Fresh tmux QA keeps non-empty default/advisory/gate JSON artifacts.
