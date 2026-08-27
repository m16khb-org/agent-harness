# IssueOps Multi-Cycle Hook Deadlock Repair

## TL;DR

Repair the supervised PreToolUse selection path so exact lifecycle observations/recovery and a narrow set of proven read-only observations remain reachable when multiple handoffs share one source checkout, while ambiguous writes and terminal steering remain fail-closed. Then rebuild, reinstall, and prove the real Codex hook contract with fresh-session and direct payload evidence.

## Context

### Original Request

The user disabled `~/.codex/hooks.json`, resumed Codex, and asked for every observed blocker to be resolved and recorded so `agent-harness` works after hooks are re-enabled. The user explicitly requested the Von Neumann planning skill and the Turing evidence loop.

### Interview Summary

- Preserve all current cycles/worktrees and repair reachability rather than deleting state to hide ambiguity.
- Keep ownership policy in shared Go core.
- Pin existing fail-closed write behavior, add regression tests before production changes, document each verified blocker, and reinstall the native hooks only after full verification.

### Gap Analysis

- **Contradiction resolved:** source checkout must remain observation-only during active supervised handoffs, but the current selector blocks observations before classifying them. Read-only bypass must occur before ambiguous record selection, without allowing any write-like command.
- **False lead resolved:** the installed binary is not stale; `go version -m` and Git both identify commit `18a8083e3f2a`.
- **Command-shape gap:** the exact parser accepts `agent-harness` and `./bin/agent-harness`, while operators also invoke the equivalent `bin/agent-harness`; this can turn a valid exact-ID observation into an id-less ambiguity.
- **Invalid recovery input separated:** the historical recovery attempt added coordinator flags that the real `handoff recover` CLI does not accept. The repaired hook must expose the canonical working escape, not bless invalid flags.
- **Scope-creep guard:** do not introduce a generic JavaScript evaluator or a broad “read-only command” catalog. Retain the hardened exact command grammar and add only the proven executable spelling and early observation routing.
- **Runtime risk:** installed file readback is insufficient. Codex 0.144.5 can retain live hook state; verification must include `hooks/list`, direct event payloads, and a fresh process/session probe.
- **Artifact risk:** editing `.agent-harness/*.md` changes docs-index response golden; regenerate and inspect only the expected projection.
- **Wrapper risk:** long-running JSON gates must use DTO-backed fields. `self-verify` exposes `ok` and `termination_eligible`, not a top-level `passed` field.
- **Audit drift risk:** stability commands must match current public flags, and timeouts must exceed measured full test/race/self-verify durations rather than historical repository timings.

## Work Objectives

### Core Objective

Eliminate the multi-cycle PreToolUse deadlock while preserving supervised single-writer enforcement.

### Deliverables

- Named baseline and regression tests for exact-ID, read-only, and ambiguous-write behavior.
- Minimal shared-core parser/selection repair.
- Detailed blocker/cause/escape record under `.agent-harness/`.
- Turing goals, ledger, evidence, cleanup receipts, and final quality-gate result.
- Rebuilt binary and restored native Codex hook configuration.
- Fresh hooks-enabled E2E evidence.

### Definition of Done

- Targeted test names run and pass; no `[no tests to run]` result is accepted.
- Exact `status --id` and `resume --id` work for either matching cycle under multi-cycle source ambiguity using all supported local executable spellings.
- `pwd`, safe `rg`, read-only Git, and explicit read tools pass under multi-cycle source ambiguity.
- Source `apply_patch`, `go test`, raw terminal write/control, malformed exact commands, and shell-control payloads remain blocked.
- The blocker document names every verified incident cause and a working bounded escape.
- Full Go tests, race, vet, build, contract goldens, self-verify, and stability audit pass in one final sequence.
- Installed Codex hooks are present, trusted/enabled for the exact cwd, and direct/fresh-session probes reproduce the expected allows and blocks.

### Must Have

- TDD order: baseline PASS → named RED → minimal GREEN.
- Shared Go core owns semantics; adapters only serialize/format.
- Existing IssueOps/Orca state is preserved.
- Evidence and cleanup receipts exist for every Turing criterion.

### Must NOT Have

- No cycle force-release, worktree deletion, global Orca reset, force push, hook bypass, arbitrary JS evaluation, or broad command allowlist.
- No approval of invalid CLI flags merely because a previous agent emitted them.
- No completion claim based only on unit tests or installed-file readback.

## Verification Strategy

- **Unit/contract:** command parser and lifecycle selection/guard tests with exact named `-run` regexes.
- **CLI scenario:** synthetic multi-cycle state plus direct `hook pre-tool-use --json` payloads.
- **Native integration:** install dry-run, real install, configured-event smoke, Codex `hooks/list`, and fresh Codex process probe.
- **Project health:** focused package set, `go test ./...`, `go test -race ./...`, `go vet ./...`, build, response goldens, self-verify, and full stability audit.

## Execution Strategy

### Parallel Execution Waves

Implementation is cross-cutting and remains in the main agent. Only the mandatory final adversarial review is delegated after all evidence is captured.

### Dependency Matrix

| Task | Depends on | Enables |
|---|---|---|
| T1 evidence bootstrap | none | T2–T6 |
| T2 baseline + RED | T1 | T3 |
| T3 minimal repair | T2 | T4–T6 |
| T4 documentation/contracts | T3 | T5–T6 |
| T5 full verification/install | T3–T4 | T6 |
| T6 native E2E/review | T5 | completion |

## TODOs

### T1 — Bootstrap durable Turing criteria and capture the pre-fix baseline

**Recommended Agent:** Main agent (cross-cutting state and safety context)

**References:** `skills/turing/SKILL.md`; `.agent-harness/TESTING.md`; `internal/core/lifecycle/lifecycle_handoff_guard_test.go`; `internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go`

**Work:** Create `.agent-harness/turing/goals.json`, `ledger.jsonl`, and named evidence targets. Capture current Git/binary/hook/state inventory. Add or identify a characterization test proving ambiguous source mutation and raw terminal steering are blocked before any production change.

**Acceptance Criteria:**

- Goals enumerate binary pass/fail criteria for exact-ID reachability, read-only reachability, fail-closed writes, blocker documentation, native installation, and enabled-hook E2E.
- The baseline test prints its intended `=== RUN` name and passes on unchanged production code.
- Evidence contains no secret values and records cleanup as `none spawned`.

**QA Scenarios:**

- Happy: two active handoff records + source `apply_patch` returns `block` with the multi-cycle reason.
- Failure: distinct persisted worker handle still does not authorize arbitrary terminal steering.

### T2 — Add named RED regressions for the deadlock inputs

**Recommended Agent:** Main agent (TDD sequence must remain in one context)

**References:** `internal/core/commandparse/issueops.go`; `internal/core/commandparse/issueops_test.go`; `internal/core/lifecycle/lifecycle_handoff_authority.go`; `internal/core/lifecycle/lifecycle_handoff_fence_scope_test.go`

**Work:** Add tests for `bin/agent-harness` exact parsing and for multi-cycle source observations (`status`, `resume`, `pwd`, safe `rg`, read-only Git, explicit read tool). Run only the new names and capture the correct assertion failures before editing production code.

**Acceptance Criteria:**

- At least one parser test and one lifecycle test fail on unchanged code for the expected behavior mismatch.
- Malformed exact commands, active shell control, `go test`, source patch, and terminal writes remain negative companions in the same tables.
- RED evidence records terminal exit and exact failed assertion, not a compile/import error.

**QA Scenarios:**

- Happy target: `bin/agent-harness issueops status --id io-first --json` is recognized as exact.
- Failure companion: `bin/agent-harness issueops status --id io-first; rm -rf /` remains unrecognized and blocked.

### T3 — Implement the smallest shared-core selection repair

**Recommended Agent:** Main agent (security-sensitive authority decision)

**References:** `internal/core/commandparse/issueops.go:19`; `internal/core/lifecycle/lifecycle_handoff_guard.go:166`; `internal/core/lifecycle/lifecycle_handoff_authority.go:821`; `.agent-harness/ARCHITECTURE.md` “IssueOps handoff” invariants

**Work:** Accept the equivalent repo-local `bin/agent-harness` spelling in the exact parser. Add a narrowly named observation predicate that runs before supervised record selection and recognizes only the existing hardened read-only grammars and explicit read tools. Do not alter lifecycle write authority tables.

**Acceptance Criteria:**

- New RED tests turn GREEN with a small, reviewable diff.
- Unknown tools/commands and all negative companions remain blocked.
- Existing focused lifecycle/commandparse packages remain green.

**QA Scenarios:**

- Happy: multi-cycle source `git status --short` returns allow without selecting an arbitrary record.
- Failure: multi-cycle source `git add .`, `go test ./...`, `apply_patch`, and `orca terminal send` still return block.

### T4 — Record blocker causes and refresh affected documentation contracts

**Recommended Agent:** Main agent (must distinguish verified defects from operator/agent misuse)

**References:** `.agent-harness/CAUTIONS.md`; `.agent-harness/TESTING.md` docs-index golden rule; `.agent-harness/AGENT_WORKFLOW.md`; historical Codex logs and current state inventory

**Work:** Add `.agent-harness/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md` with symptom, verified root cause, impact, correct escape, status, and evidence for every blocker. Add concise evergreen cautions for uncovered defect classes. If response-contract golden changes, inspect it before accepting: docs-index drift is expected, while project-local host presence is machine state and must be normalized rather than copied as raw booleans.

**Acceptance Criteria:**

- Record covers exact-ID spelling, pre-selection read-only deadlock, invalid invented recovery flags, multi-cycle durable state, missing hook backup name, live-session hook reload, and previously resolved operational blockers.
- Every claim names a file/line, command output, state row, or installed-binary/source observation.
- Golden diff contains only intended contract metadata or normalized machine-state placeholders; raw project-local presence booleans are forbidden.

**QA Scenarios:**

- Happy: a future operator can copy a canonical exact status/resume/recover command from the record.
- Failure: no instruction recommends disabling hooks, deleting cycles, or retrying ambiguous Orca mutations as routine recovery.

### T5 — Run full verification, rebuild, and restore native hooks

**Recommended Agent:** Main agent (user configuration and all-or-nothing gate)

**References:** `.agent-harness/TESTING.md`; `.agent-harness/operations/hosts.md`; `skills/stability-audit/SKILL.md`; `scripts/install-native.sh`

**Work:** Run formatting and focused packages, then the ordered full test/race/vet/build/contract/self-verify gate. Run install dry-run, inspect the plan, execute native install, and verify all configured hook events with representative payloads. Preserve unrelated co-resident hook groups from the backup.

**Acceptance Criteria:**

- All verification commands exit 0 in the final uninterrupted wave.
- The self-verify wrapper checks `.ok`, `.termination_eligible`, and `.summary.termination_eligible` against the source DTO contract.
- Stability audit invokes no obsolete top-level sync flag and gives full regression/self-verify the measured 300s/5400s ceilings.
- `~/.codex/hooks.json` exists with mode 0600 and contains generated agent-harness groups plus preserved unrelated Orca groups.
- Built binary reports the final source revision/modified status expected for the verification stage.
- No temp process/socket/directory remains from the audit.

**QA Scenarios:**

- Happy: every installed event returns schema-valid JSON for a representative payload.
- Failure: modified/untrusted hook entries, missing required events, or stale binary/version skew fail the gate.

### T6 — Prove hooks-enabled behavior and pass the binding adversarial review

**Recommended Agent:** Main agent for QA; fresh reviewer sub-agent only for the Turing final quality gate

**References:** Codex 0.144.4 installed binary; `codex app-server --stdio` hooks/list contract; `.agent-harness/turing/evidence/`; `skills/issueops/references/ai-slop-clean.md`; `skills/shannon/SKILL.md`

**Work:** Run direct PreToolUse payloads against real multi-cycle state, then a fresh Codex hooks/list/process probe. Capture allowed exact/read-only cases and blocked write/control cases. Run Shannon and ai-slop-clean checks. Give the final diff, criteria, commands, evidence, and cleanup receipts to one fresh adversarial reviewer; fix and rerun the complete scenario until it returns unconditional approval.

**Acceptance Criteria:**

- Exact-ID and read-only scenarios allow with hooks enabled; source write/control scenarios block for the intended ownership reason.
- Fresh exact-cwd hooks/list reports required events enabled with empty warnings/errors or a precisely documented, repaired trust state.
- Reviewer verdict is unconditional approval and references the captured evidence.
- Turing metrics report 100% evidence and cleanup coverage.

**QA Scenarios:**

- Happy: fresh process executes an allowed observation and receives valid hook output.
- Failure: fresh process attempts an ambiguous source mutation and is blocked before execution.

## Final Verification Wave

Run the entire ordered gate from its first command after the final edit. Any failure restarts the wave from command one. Capture terminal exits and output artifacts under `.agent-harness/turing/evidence/`.

## Commit Strategy

One atomic fix commit on `main` after the quality gate, using a Conventional Commit subject and required `Lore:` body. Push only after local `main` and `origin/main` ancestry are rechecked and no unrelated changes are staged.

## Success Criteria

Repo grounding: CodeGraph and current source locate the failure at `selectSupervisedHandoffRecord`, with supporting parser/runtime evidence and current state inventory.

Decision-complete plan: six ordered TODOs define exact files, binary acceptance, QA scenarios, verification commands, cleanup, and publication boundary.

Assumptions/defaults: valid existing supervised cycles remain authoritative; only provably read-only requests bypass selection ambiguity; all other unknowns fail closed.

Unresolved questions: none blocking. Fresh-runtime payload/probe results may refine test fixtures but cannot widen the allowlist without a new named RED case.

Acceptance criteria: all Definition of Done checks above are machine-verifiable and the final reviewer issues unconditional approval.
