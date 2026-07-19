# Issue #47 — Supervised Multi-Cycle Control-Plane Deadlock Repair

## Outcome

Repair the shared lifecycle guard so multiple supervised IssueOps records for one source checkout do not block a deliberately closed set of host control-plane, read-only, exact-ID lifecycle, and Codex hook-trust observation requests. All repository mutation, ambiguous lifecycle mutation, malformed payloads, and claimed-worker ownership violations remain fail-closed.

This plan covers the bootstrap repair, verification, publication, merge, and native reinstall. Issue #46 recovery starts only after the merged repair is installed and its live hook matrix passes from the original source checkout.

## Verified Starting State

- Bootstrap source commit and `origin/main`: `c5e9304665228732db16b224073f46124122ec21`.
- Issue: `https://github.com/m16khb/agent-harness/issues/47`; branch: `47-fix-supervised-multicycle-control-plane-deadlock`.
- Canonical worker: `/private/tmp/agent-harness-bootstrap.X4ZhDJ.worktrees/47-fix-supervised-multicycle-control-plane-deadlock`.
- Baseline `go test ./... -count=1`: exit 0, including `internal/core/lifecycle` in approximately 151 seconds.
- Original source live records: `io-65b25b19728b` is `coordinator_preparing`; `io-d492e1a529e3` is `closed`; both name `/Users/m16khb/Workspace/agent-harness` as the source repository.
- The recovery worktree's sole uncommitted change remains untouched in `internal/core/issueops/issueops_handoff_publication.go`; its recorded diff SHA-256 is `122f2b750108606eaecce8734b75dc7c003929f04815c36e7a828b1847a3dfaf`.
- Installed Codex is `0.144.6`. Its generated app-server schema defines `hooks/list` with optional `cwds: []string`; the installed native binary contains `write_stdin`, `hooks/list`, and `config/batchWrite` protocol strings.
- A live SSH-only `codex -C <issue-47-worker> app-server --stdio` probe accepted `initialize`, `initialized`, and `hooks/list` as separate JSONL messages. The exact worker cwd returned 13 enabled hooks, all trusted, with empty warnings and errors. No config write or trust bypass was used.
- Official `openai/codex` source at tag `rust-v0.144.6`, `codex-rs/core/src/tools/handlers/unified_exec/write_stdin.rs:110-115`, returns `None` from `pre_tool_use_payload`: non-empty continuation writes never invoke PreToolUse. Therefore a guard predicate for `write_stdin` would not execute in the real host and cannot enforce the requested method denylist.
- Installed CodeGraph exposes `codegraph explore [options] <query...>`. The repository has no `.codegraph/`, so CodeGraph was not used for repository exploration and no index will be created.

## Root Cause

`handoffOwnershipBlockReason` recognizes only the existing exact shell/read-tool observations before calling `selectSupervisedHandoffRecord`. The selector can identify a worker root or the narrow lifecycle ID sources known to `lifecycleRecordID`, but it cannot identify the requested host control-plane tools, IssueOps `status`/`resume` MCP calls, or Codex trust-review transport. When more than one record then matches `record.Repo`, selection returns the multi-cycle ambiguity error before those non-repository operations can be classified.

This is an ordering plus classification defect, not a reason to remove durable records or weaken the ownership fence.

## Allowed-Set Decision

The repair adds four independent, exact predicates before or at record selection. No generic “safe tool” or generic JSON-RPC mechanism is introduced.

### 1. Host control plane

Allow only exact tool names:

- `get_goal`
- `update_goal`
- `update_plan`
- `request_user_input`

These tools cannot edit the repository or mutate an IssueOps lifecycle. Prefix, suffix, case-folded, or look-alike names remain outside the carve-out. Their host-owned input schemas are not duplicated in the harness; exact tool identity is the boundary.

### 2. CodeGraph observation

Extend `commandparse.ExactReadOnlyShellCommand` with exactly three parsed argv tokens:

```text
codegraph explore '<one argv-safe query>'
```

The query must be non-empty and must not begin with `-`. Options, an explicit path, additional query argv, other CodeGraph subcommands, shell control, substitutions, redirects, expansions, and executable-path aliases remain denied. This matches the repository's prescribed quoted invocation and cannot build, refresh, or remove an index.

### 3. Exact-ID IssueOps MCP observation/control

Recognize only `issueops_status` and `issueops_resume`, either bare or with the exact `mcp__agent_harness__` prefix. Extract a non-empty `id` before source fallback so the matching supervised record is selected.

- `status`: input keys are exactly `id`.
- `resume`: input keys are `id` plus optional `repo` and optional `bind`; `repo`, when present, must resolve to the selected record's source repo, and `bind` must be absent or exactly `false`. The public CLI/MCP handlers already reject `bind=true` for supervised handoffs, so the guard must not imply otherwise.
- Nested `flags`, unknown keys, missing/empty IDs, IDs absent from the source checkout's supervised-record set, wrong repos, non-boolean or true `bind`, and look-alike namespaces are denied.

For these two recognized MCP tools, an ID absent from the supervised `byID` set must return an explicit block before the selector's general “explicit different lifecycle” escape. This closes the only new binding-capable route without changing the existing foreign-ID behavior of other exact lifecycle commands. These tools are kept separate from handoff mutation MCP classification.

### 4. Bounded Codex hook-trust observation during `coordinator_preparing`

Direct `codex -C <worker> app-server --stdio` remains blocked by the supervised guard. Codex 0.144.6 checks only that initial shell launch; every later `write_stdin`, including `config/batchWrite`, bypasses PreToolUse. Pretending to validate synthetic `write_stdin` requests in core would be a non-enforcing security control.

Add one bounded command instead:

```text
agent-harness issueops handoff codex-hooks-list --id <exact-cycle-id> --json
```

The lifecycle guard allows this exact command only from the exact source checkout, on host `codex`, for a valid uniquely selected record in pristine `coordinator_preparing`. Missing IDs and IDs not present in the current source checkout's supervised `byID` set—including a valid cycle owned by another source checkout—return a dedicated block before the selector's general explicit-different-lifecycle escape. No worker, executable, method, stdin, timeout, or config argument is caller-controlled.

The CLI reads `record.ExecutionHandoff.WorkerRoot` and delegates to `internal/adapter/codex`. The adapter owns all subprocess transport:

- the adapter resolves the installed `codex` target once to an absolute path, then executes exactly `<resolved-codex> -C <record.WorkerRoot> app-server --stdio` with `cmd.Dir` fixed to that worker root;
- the child receives only the named `codex_hooks_list_v1` environment allowlist (`CODEX_HOME`, home/locale/user/temp/XDG display-neutral settings; never ambient credentials or `PATH`), and the resolved executable, fixed argv, cwd, timeout, environment-key names, outcome, and bounded redacted diagnostic are appended under a unique process-execution audit id;
- it writes exactly these three hard-coded JSONL messages, one write at a time:
  - `{"method":"initialize","id":1,"params":{"clientInfo":{"name":"agent_harness","title":"agent-harness","version":"1"}}}`
  - `{"method":"initialized","params":{}}`
  - `{"method":"hooks/list","id":2,"params":{"cwds":["<exact-record-worker-root>"]}}`
- after writing initialize, it requires exactly one successful JSON-RPC response with id 1 before writing initialized and hooks/list; response id 2 is accepted only afterward;
- it exposes no generic JSON-RPC or stdin API, closes stdin after response id 2, and kills and reaps the child process group after a fixed 15-second total timeout;
- stdout is capped at 1 MiB, stderr diagnostics at 32 KiB, one JSONL line at 512 KiB, and the combined response/notification count at 256;
- a JSON-RPC error, duplicate id 1/2, response ID other than 1/2, id 2 before successful id 1, malformed/trailing JSON, or output after the bounded object count fails closed; bounded notifications without a response ID may be ignored while awaiting the two expected responses;
- response id 2 must name exactly the requested cwd. The result preserves the bounded hook inventory, warnings, errors, enabled state, trust status, current hashes, source paths, and redacted commands needed for agent attestation;
- the helper accepts only the canonical bounded `io-<12-lowercase-hex>` cycle id and returns a generic non-reflective denial before lookup for every other value;
- the id-2 result must contain exactly one cwd entry, at most 1,024 hooks, at most 256 warnings, at most 256 errors, and no string or map key anywhere in the raw result longer than 4,096 bytes; secret-bearing normalized keys and their opaque values are rejected rather than reflected;
- raw-result traversal is capped at depth 32 and 32,768 nodes, and the exact indented JSON result (including reserved audit-id space) is capped at 1 MiB;
- it never sends or exposes `config/batchWrite` and never writes Codex config or hook trust.

This helper only makes the documented read-only review reachable. It does not set trust, attest a result, invoke `--dangerously-bypass-hook-trust`, or persist repository authority. It keeps host process mechanics in the Codex adapter and lifecycle selection in shared core.

## Deny Matrix That Must Stay Pinned

| Category | Examples | Expected |
|---|---|---|
| Host look-alikes | `functions__update_goal`, `UpdateGoal`, `update_goal_extra` | block under source ambiguity |
| CodeGraph mutation/ambiguity | `codegraph sync`, `codegraph explore --path /tmp q`, `codegraph explore one two` | block |
| MCP without exact authority | status/resume with missing/empty/unmatched ID, nested `flags`, unknown field, foreign namespace, resume wrong repo, non-boolean/true `bind` | block |
| Codex direct/unbounded transport | direct app-server launch, any synthetic/unbound `write_stdin` including `config/batchWrite`, bypass flag | block |
| Bounded helper mismatch | missing/unmatched (including real foreign-source) ID, wrong cwd/host/state, positional argument, extra flag, worker/method/stdin/config argument, or non-pristine pending/cleanup/session/result field | block |
| Repository mutation | `apply_patch`, `go test`, `git add`, arbitrary shell mutation | block |
| Ambiguous lifecycle mutation | handoff/mutation call without a matching exact ID or worker target | block |
| Ownership regression | source tries claimed-worker mutation; claimed worker targets another worker/root | block |

## TDD Execution

### Step 1 — Add RED tests only

Edit tests before production code:

- `internal/core/commandparse/issueops_test.go`
  - `TestExactReadOnlyShellCommandAllowsOnlyExactCodeGraphExplore`
- `internal/core/lifecycle/lifecycle_handoff_multi_cycle_recovery_test.go`
  - `TestHandoffMultiCycleAllowsExactHostControlPlaneTools`
  - `TestHandoffMultiCycleAllowsExactCodeGraphExplore`
  - `TestHandoffMultiCycleSelectsExactIssueOpsStatusResumeMCP`
  - `TestHandoffMultiCycleRejectsInexactIssueOpsStatusResumeMCP`
  - `TestHandoffMultiCycleAllowsExactCoordinatorPreparingCodexTrustReview`
  - `TestHandoffMultiCycleRejectsInexactControlPlaneAndTrustReview`
  - `TestHandoffCodexHooksListRequiresValidSourceCoordinatorPreparingRecord`
- `cmd/harness/hookcli/hook_pre_tool_handoff_test.go`
  - `TestRunHookPreToolUseSupervisedMultiCycleControlPlaneMatrix`
- `cmd/harness/issueopscli/issueops_codex_hooks_cli_test.go`
  - `TestIssueOpsHandoffCodexHooksListOwnsExactCodexArgvAndJSONL`
  - `TestIssueOpsHandoffCodexHooksListFailsClosedOnMalformedOrUnboundedResponse`

Positive tables must fail against unchanged production code because the current guard returns the multi-cycle source ambiguity block or the existing CLI reports an unknown bounded-helper subcommand. Negative companions must already pass. The lifecycle state table explicitly covers valid single-cycle `coordinator_preparing` allow plus invalid record, claimed state, claimed-worker cwd, cross-record target, wrong host/cwd, direct app-server, and synthetic `write_stdin` denial. A compile error, panic, or `[no tests to run]` is not valid RED evidence.

Run:

```bash
go test ./internal/core/commandparse -run '^TestExactReadOnlyShellCommandAllowsOnlyExactCodeGraphExplore$' -count=1 -v
go test ./internal/core/lifecycle -run '^(TestHandoffMultiCycle(AllowsExactHostControlPlaneTools|AllowsExactCodeGraphExplore|SelectsExactIssueOpsStatusResumeMCP|RejectsInexactIssueOpsStatusResumeMCP|AllowsExactCoordinatorPreparingCodexTrustReview|RejectsInexactControlPlaneAndTrustReview)|TestHandoffCodexHooksListRequiresValidSourceCoordinatorPreparingRecord)$' -count=1 -v
go test ./cmd/harness/hookcli -run '^TestRunHookPreToolUseSupervisedMultiCycleControlPlaneMatrix$' -count=1 -v
go test ./cmd/harness/issueopscli -run '^TestIssueOpsHandoffCodexHooksList(OwnsExactCodexArgvAndJSONL|FailsClosedOnMalformedOrUnboundedResponse)$' -count=1 -v
```

Record `=== RUN`, the expected assertion mismatch, and non-zero exit status. The CLI helper tests call the existing `runIssueOpsHandoff` dispatcher, so unchanged code fails with the observable unknown-subcommand mismatch rather than a compile error. Implement the happy path, then keep each malformed/oversized fixture RED until its corresponding bound is added.

### Step 2 — Add the minimum production behavior

Production scope:

- `internal/core/commandparse/issueops.go`
- `internal/core/lifecycle/lifecycle_handoff_guard.go`
- `internal/core/lifecycle/lifecycle_handoff_authority.go`
- `internal/adapter/codex/hooks_list.go`
- `cmd/harness/issueopscli/issueops_codex_hooks_cli.go`
- the existing IssueOps/harnessapp usage and routing files needed to expose the bounded subcommand
- `skills/issueops/references/orca-handoff.md`, `skills/turing/SKILL.md`, and concise operating/caution text that replaces the unsafe direct interactive recipe

Implementation shape:

1. Add the exact CodeGraph branch beside existing `pwd`/`rg`/`git` read-only grammar.
2. Add small exact-name helpers for host control-plane and IssueOps status/resume MCP; do not broaden `explicitHandoffReadOnlyTool` or `handoffMCPToolKind`.
3. Teach `lifecycleRecordID` the two exact observation MCP tools, and block a recognized observation tool whose exact ID is absent from the supervised `byID` set before the pre-existing general explicit-ID escape.
4. After selection, allow only a schema-valid status/resume payload for the selected record; supervised resume accepts absent/false `bind` only.
5. Add the exact-ID `handoff codex-hooks-list` parser/authority row. Its missing/foreign-ID cases share the dedicated pre-escape block with observation MCP, including IDs that exist only under another source checkout. Direct app-server and `write_stdin` receive no carve-out.
6. Add the bounded adapter plus thin CLI described above. The adapter owns fixed child argv/protocol and the CLI owns record lookup; no durable session registry or generalized JSON-RPC proxy is added.

No lifecycle refactor, generic tool registry, generalized JSON-RPC allowlist, process-session registry, automatic trust mutation, or adjacent cleanup is in scope.

### Step 3 — Turn RED to GREEN and pin legacy fences

Rerun the two named commands and require every named test to appear and pass. Then run:

```bash
go test ./internal/core/commandparse -count=1
go test ./internal/core/lifecycle -run '^(TestHandoffGuardBlocksBeforeClaim|TestHandoffGuardAllowsMatchingClaimedWorkerInTree|TestHandoffGuardBlocksCoordinatorAbsolutePathIntoWorkerTree|TestClaimedWorkerRoleBlocksCoordinatorOwnedCommandsAndChecksBranch|TestClaimedWorkerMutationOperandsAndSymlinksStayWithinRoot|TestCoordinatorPreparingAllowsOnlyExactPreDispatchCancel|TestHandoffMultiCycle.*)$' -count=1 -v
go test ./cmd/harness/hookcli -run '^TestRunHookPreToolUseSupervisedMultiCycleControlPlaneMatrix$' -count=1 -v
go test ./cmd/harness/issueopscli -run '^TestIssueOpsHandoffCodexHooksList(OwnsExactCodexArgvAndJSONL|FailsClosedOnMalformedOrUnboundedResponse)$' -count=1 -v
```

If a regex selects no tests, replace it with the exact existing names reported by `go test -list` before treating it as evidence.

## Actual Hook Matrix

The named hookcli RED test creates synthetic state containing two source-sharing supervised records and feeds complete PreToolUse JSON through `runHookPreToolUse`. It evaluates representative allow and deny rows through the adapter, not only the core function, and checks exact decision/reason. After GREEN, repeat the same matrix against the built CLI with a `mktemp -d` `HARNESS_STATE_DIR`; remove only that explicitly resolved temp directory afterward.

For native runtime proof after merge/install:

1. Query the two original cycle IDs exactly.
2. From the original source checkout, evaluate the four control-plane names, exact status/resume, exact CodeGraph form, and exact `handoff codex-hooks-list --id` path.
3. Require the bounded helper's response id 2 to report the exact worker cwd, enabled required hooks, empty warnings/errors, and trusted generated commands; its fake-child contract test proves the exact child argv and three-message transcript.
4. Evaluate direct app-server, synthetic `write_stdin`, representative generic mutation, and malformed helper payloads and require a block before execution.
5. Never invoke `config/batchWrite`, disable hooks, or use trust bypass merely to prove the predicate.

## Full Verification Gate

After the final edit, run from the worker root in this order; any failure is repaired and the affected gate rerun with fresh terminal exits:

```bash
gofmt -w <only changed Go files>
go test ./internal/core/commandparse ./internal/core/lifecycle ./internal/adapter/codex ./cmd/harness/hookcli ./cmd/harness/issueopscli -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

Inspect the diff and generated artifacts. The new bounded CLI subcommand intentionally changes IssueOps usage and may add a response-contract fixture; accept only those reviewed command/DTO projections. No unrelated MCP tool, lifecycle DTO, or host-specific response drift is expected.

## Reviews and Evidence Gate

Before production edits, give this plan, issue body, target source, and deny matrix to a fresh Brooks devil's-advocate reviewer. It must challenge conceptual integrity, over-broad authority, missing negative cases, and optimistic verification. A second fresh read-only reviewer must independently check that each user acceptance criterion maps to a named test and observable runtime assertion. Resolve every Critical/Important finding in the plan before RED.

The original issue text requests direct app-server plus guarded continuation JSONL. Before RED, append a Korean design correction to issue #47 that cites the official 0.144.6 `write_stdin` source, preserves the original request as incident history, and replaces only that unenforceable acceptance row with the bounded helper contract above. Record the same decision in the IssueOps decision ledger. This is a safety-preserving refinement of the requested trust-review outcome, not an unreported relaxation.

After GREEN and full verification, run a fresh adversarial diff review plus Shannon/AI-slop checks. The main agent retains cross-cutting implementation and all Git/GitHub/installation authority; reviewers do not edit, push, merge, or clean state.

## Publication and Installation

1. Confirm the diff contains only issue #47 changes and the worker is based on current `origin/main`.
2. Create one atomic Conventional Commit with a Lore body, push the existing issue-linked branch, and open a PR whose body contains `Closes #47`.
3. Inspect CI and review output; repair failures by TDD in the same cycle. Merge only after required checks pass and a fresh review has no unresolved Critical/Important finding. Verify the PR is `MERGED`, record its merge commit, and verify issue #47 is `CLOSED` rather than assuming the keyword worked.
4. Update the original source to the verified merge commit using the repository's non-destructive update path.
5. Run official install/update dry-run, inspect paths and preservation behavior, back up generated files before replacement, then install. Preserve unrelated/manual host configuration.
6. At the merge commit, build a clean reference binary and record `git rev-parse HEAD` plus its SHA-256. Verify the installed `command -v agent-harness` bytes/hash equal that reference build, then capture Codex, Claude, GJC, MCP, and hook smoke outputs required by project docs.
7. Re-run the real original-source multi-cycle allow/block matrix. Only then begin #46 recovery.

## Issue #46 Handoff Boundary

The repair does not modify either #46 worktree. After installation, compare exact canonical/recovery records again, re-hash the preserved recovery diff, and run a focused test that demonstrates whether its post-config-lock publication hook is necessary. Preserve the clean canonical worktree and ensure only one cycle can write. Integrate valid recovery work through a reviewed patch/commit; if invalid, keep it untouched until the evidence and user-authorized final disposition are recorded.

## Success Criteria

- Every new positive test is observed RED before production edits and GREEN afterward.
- Every deny companion remains blocked for the intended reason.
- Existing single-cycle and claimed-worker fence tests remain green.
- Focused, full, race, vet, build, goldens, self-verify, adapter matrix, and installed native matrix all pass.
- The issue #47 PR is merged, issue #47 is closed by verified work, and the installed harness comes from the merge commit.
- Original #46 state and its recovery diff remain byte-for-byte preserved until the repaired guard is deployed and the dedicated recovery phase begins.

## Assumptions and Open Questions

- Assumption: host control-plane tools cannot write repository or lifecycle state; exact tool identity is therefore sufficient and avoids copying volatile host schemas into core.
- Assumption: trust review remains agent-attested according to the existing skill contract; this change provides reachability and does not infer trust from request shape.
- Verified host constraint: Codex 0.144.6 emits no PreToolUse for `write_stdin`; the bounded helper is required for an enforceable method allowlist and direct app-server remains denied.
- Open questions: none blocking. Any reviewer evidence that disproves an assumption returns the plan to design review before RED.
