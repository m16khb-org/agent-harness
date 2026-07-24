# Workpool One-Shot Removal Implementation Plan

> **For the execution agent:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans`, `superpowers:test-driven-development`, and `superpowers:verification-before-completion`. Execute in `/Users/habin/workspace/agent-harness.worktrees/remove-workpool` only.

**Goal:** Remove the `workpool` feature in one source change, including its Go core, CLI, MCP, IssueOps gate, hook reminder, public contracts, tests, and active documentation.

**Architecture:** Actual subagent concurrency belongs to each host runtime. Codex uses its native spawned-thread limit; durable delegated work uses IssueOps child cycles and the generation-fenced execution contract. The old workpool SQLite database is left inert on disk and is not migrated or deleted.

**Tech Stack:** Go 1.26.x, standard `flag` CLI, MCP Go SDK, SQLite-backed harness state, golden contract tests, Markdown project guidance.

## Global Constraints

- Work only in `/Users/habin/workspace/agent-harness.worktrees/remove-workpool` on branch `m16khb/remove-workpool`.
- Baseline is exact commit `9650fc908a085697b2caedc0d922b5267bda25a5`; `go test ./... -count=1` passed before implementation.
- Do not touch `/Users/habin/workspace/agent-harness` or copy its uncommitted changes into this worktree.
- Remove the feature in one branch change. Do not add deprecation aliases, compatibility shims, hidden MCP aliases, or recovery commands.
- Do not delete or mutate `~/.local/state/agent-harness/workpool/harness.db`; do not call mutating legacy `workpool status/reap/close` commands.
- Do not commit, push, create a PR, merge, or clean either worktree.
- Preserve unrelated behavior, but clean product-specific workpool material from tracked historical ADR/plan/spec/issue documents too. Rewrite mixed documents surgically around IssueOps child cycles, native host concurrency, or generic SQLite maintenance; do not delete unrelated sections or whole mixed-purpose documents.
- Use `apply_patch` for source edits and file deletion. Use formatting/golden-generation commands only for mechanical output.
- The public post-removal contract is:
  - `agent-harness workpool ...` is an unknown top-level command.
  - every `workpool_*` MCP call is an unknown tool and no `workpool_*` tool appears in `tools/list`.
  - strict IssueOps PR readiness no longer reads a workpool namespace or emits `pool_incomplete:*`.
  - hook reminders contain child-cycle guidance only.
  - state maintenance and legacy-process classification no longer enumerate workpool.
  - native host fan-out and IssueOps child cycles remain intact.

## Karpathy Input/Output Contract

- **Input:** clean branch at the baseline SHA plus this plan; no source-worktree changes and no runtime-state mutation.
- **Output:** a reviewable uncommitted diff removing every live workpool surface, regenerated goldens, and verification evidence.
- **Sanity cases:** unknown CLI command, absent MCP tools, child-only reminder, strict readiness unaffected by inert workpool state.
- **Adversarial cases:** a stale `~/.local/state/agent-harness/workpool/harness.db` must neither be mutated nor restore a gate; historical Markdown mentions must not be mistaken for active functionality.
- **Tool truth:** verify with current CodeGraph/`rg`, Go tests, the built binary, and Git status; do not infer from this plan when repository evidence differs.

---

### Task 1: Turn removal requirements into failing public-contract tests

**Files:**
- Modify: `internal/adapter/cli/usage_test.go` or the existing CLI catalog test that owns `Commands()`/`Usage()`
- Modify: `internal/adapter/mcp/catalog_test.go`
- Modify: `internal/core/hookprompt/orchestration_reminder_test.go`
- Modify: `cmd/harness/statecli/state_cli_maintain_test.go`
- Modify: `cmd/harness/hookcli/hook_stop_contract_test.go`

**Interfaces:**
- Consumes: `cli.Commands`, `cli.Usage`, `mcp.AdvertisedTools`, `mcp.DispatchMap`, `orchestrationReminderValue`, state-maintenance result roots.
- Produces: negative tests proving workpool is absent from live surfaces before implementation code is removed.

- [ ] **Step 1: Add the CLI and MCP absence assertions**

  Add assertions equivalent to:

  ```go
  for _, command := range Commands() {
      if command.Name == "workpool" {
          t.Fatal("workpool command must be removed")
      }
  }
  if strings.Contains(Usage("test"), "agent-harness workpool") {
      t.Fatal("usage must not advertise workpool")
  }
  for _, tool := range AdvertisedTools() {
      if strings.HasPrefix(tool.Name, "workpool_") {
          t.Fatalf("workpool MCP tool must be removed: %s", tool.Name)
      }
  }
  for name := range DispatchMap() {
      if strings.HasPrefix(name, "workpool_") {
          t.Fatalf("workpool dispatch route must be removed: %s", name)
      }
  }
  ```

- [ ] **Step 2: Change reminder and state-maintenance expectations**

  In the reminder tests, keep the child-cycle fixture and assert that output contains child status but contains neither `workpool` nor `pool fanout`. Remove the obsolete JSON pool fixture helper. In `state_cli_maintain_test.go`, expect only `worker` and `loop` as skipped fixed roots after materializing state and IssueOps.

- [ ] **Step 3: Remove workpool from stop-relay fixture text**

  Keep the stop-hook’s three-choice contract, but make choice 3 a child/execution action rather than `workpool 상태만 확인합니다`. Remove the workpool import and legacy pool JSON fixture from `seedStopRelayOrchestrationFixture`.

- [ ] **Step 4: Verify RED**

  Run:

  ```bash
  go test ./internal/adapter/cli ./internal/adapter/mcp ./internal/core/hookprompt ./cmd/harness/statecli ./cmd/harness/hookcli -count=1
  ```

  Expected: failures naming the still-present `workpool` command/tools/reminder/fixed root. If every test passes, strengthen the assertions until at least one failure proves the old feature is still observable.

---

### Task 2: Remove the CLI, MCP, and response-contract surfaces

**Files:**
- Delete: `cmd/harness/workpoolcli/workpool.go`
- Delete: `cmd/harness/workpoolcli/workpool_cli_test.go`
- Delete: `cmd/harness/mcpcli/mcp_tool_workpool.go`
- Delete: `cmd/harness/mcpcli/mcp_workpool_test.go`
- Delete: `internal/adapter/mcp/workpool_catalog.go`
- Modify: `cmd/harness/harnessapp/cli_facade.go`
- Modify: `cmd/harness/harnessapp/root_command_facade.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `internal/adapter/mcp/catalog.go`
- Modify: `internal/adapter/mcp/catalog_test.go`
- Modify: `cmd/harness/mcpcli/mcp_tools.go`
- Modify: `cmd/harness/mcpcli/mcp_sdk_server.go`
- Modify: `cmd/harness/contractcli/contract.go`
- Modify: `cmd/harness/harnessapp/response_contract_cli_snapshot_test.go`
- Modify: `cmd/harness/harnessapp/response_contract_mcp_snapshot_test.go`

**Interfaces:**
- Consumes: top-level CLI command map, adapter command catalog, MCP catalog/dispatch map, contract-schema registry.
- Produces: no routable or advertised workpool CLI/MCP/response type.

- [ ] **Step 1: Delete dedicated adapters and their tests**

  Delete the five dedicated files listed above. Do not replace them with stubs.

- [ ] **Step 2: Remove top-level CLI wiring**

  Remove the `workpoolcli` import and `runWorkpool` facade, remove `"workpool": runWorkpool` from the root command map, and remove the command plus all ten workpool usage lines from `internal/adapter/cli/usage.go`.

- [ ] **Step 3: Remove MCP wiring**

  Remove `DispatchWorkPool`, the `WorkPoolTools` catalog section, the handler lookup entry, and `handleWorkpoolMCPToolCall` from the generic dispatch loop. Keep all other dispatch groups and their order unchanged.

- [ ] **Step 4: Remove workpool response schemas and snapshots**

  Remove `workpool_pool`, `workpool_task`, `workpool_claim`, and `workpool_status` from `contract.go`. Remove CLI and MCP snapshot setup for create/add/status/close and their ID replacements. Do not change unrelated snapshot entries.

- [ ] **Step 5: Verify focused adapter GREEN**

  Run:

  ```bash
  go test ./internal/adapter/cli ./internal/adapter/mcp ./cmd/harness/mcpcli ./cmd/harness/harnessapp ./cmd/harness/contractcli -count=1
  ```

  Expected: compilation succeeds; golden tests may still fail only because checked-in snapshots have not yet been regenerated.

---

### Task 3: Remove the core package and all live integrations

**Files:**
- Delete: `internal/core/workpool/`
- Delete: `internal/core/workpool_facade.go`
- Delete: `internal/core/issueops_pool_gate_test.go`
- Delete: `cmd/harness/issueopscli/issueops_pool_gate_cli_test.go`
- Modify: `internal/core/issueops_facade.go`
- Modify: `internal/core/hookprompt/orchestration_reminder.go`
- Modify: `internal/core/hookprompt/orchestration_reminder_test.go`
- Modify: `cmd/harness/hookcli/hook_stop_contract_test.go`
- Modify: `internal/core/state/state_maintain.go`
- Modify: `internal/core/state/state_doctor_entry.go`
- Modify: `internal/core/issueops/reset_legacy_process.go`
- Modify: `cmd/harness/statecli/state_cli_maintain_test.go`

**Interfaces:**
- Consumes: strict IssueOps readiness, child reminder, state root catalog, legacy process classifier.
- Produces: strict readiness composed only from IssueOps and loop gates; no workpool imports or state roots.

- [ ] **Step 1: Delete the workpool core and pool-gate tests**

  Delete every tracked file under `internal/core/workpool/`, the facade, and the two IssueOps pool-gate test files. Do not preserve exported aliases.

- [ ] **Step 2: Simplify strict IssueOps readiness**

  Remove the workpool import and `issueOpsStrictPRReadinessWithPoolGate`. Make the two facade functions compose only the IssueOps result and loop gate:

  ```go
  func IssueOpsStrictPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
      return issueOpsStrictPRReadinessWithLoopGate(
          issueops.IssueOpsStrictPRReadiness(record),
          record.Repo,
      )
  }

  func IssueOpsStrictPRReadinessWithState(stateRoot string, record IssueOpsRecord) IssueOpsReadiness {
      return issueOpsStrictPRReadinessWithLoopGate(
          issueops.IssueOpsStrictPRReadinessWithState(stateRoot, record),
          record.Repo,
      )
  }
  ```

- [ ] **Step 3: Make orchestration reminders child-only**

  Remove `orchestrationPoolReminders`, `orchestrationPoolSummary`, `linkedActivePoolSummaries`, `readPoolManifestOnly`, and `countPoolTaskFiles`. Remove imports made unused by that deletion. Preserve child counting, validation, deterministic ordering, and reminder limits.

- [ ] **Step 4: Remove maintenance and process-classifier awareness**

  Remove `filepath.Join(base, "workpool")` from fixed maintenance roots, remove `"workpool"` from `isHarnessOwnedStateDirectory`, and remove workpool cases from `isHarnessRuntimeProcess`/`classifyHarnessRuntimeProcess`. Do not delete any state directory.

- [ ] **Step 5: Verify core GREEN and no Go references**

  Run:

  ```bash
  gofmt -w \
    cmd/harness/contractcli/contract.go \
    cmd/harness/harnessapp/cli_facade.go \
    cmd/harness/harnessapp/root_command_facade.go \
    cmd/harness/harnessapp/response_contract_cli_snapshot_test.go \
    cmd/harness/harnessapp/response_contract_mcp_snapshot_test.go \
    cmd/harness/hookcli/hook_stop_contract_test.go \
    cmd/harness/mcpcli/mcp_sdk_server.go \
    cmd/harness/mcpcli/mcp_tools.go \
    cmd/harness/statecli/state_cli_maintain_test.go \
    internal/adapter/cli/usage.go \
    internal/adapter/cli/usage_test.go \
    internal/adapter/mcp/catalog.go \
    internal/adapter/mcp/catalog_test.go \
    internal/core/hookprompt/orchestration_reminder.go \
    internal/core/hookprompt/orchestration_reminder_test.go \
    internal/core/issueops/reset_legacy_process.go \
    internal/core/issueops_facade.go \
    internal/core/state/state_doctor_entry.go \
    internal/core/state/state_maintain.go
  go test ./internal/core/... ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/statecli -count=1
  rg -n -i 'workpool|work pool|DispatchWorkPool|WorkPool' --glob '*.go' cmd internal
  ```

  Expected: tests pass and `rg` exits 1 with no matches. Generic non-product words such as a database connection pool are not targets.

---

### Task 4: Regenerate public goldens and prove removed commands are rejected

**Files:**
- Modify mechanically: `cmd/harness/testdata/usage.golden.txt`
- Modify mechanically: `cmd/harness/testdata/mcp_tools.golden.json`
- Modify mechanically: `cmd/harness/testdata/response_contracts.golden.json`

**Interfaces:**
- Consumes: live CLI usage, MCP advertised catalog, response-contract snapshot builders.
- Produces: checked-in golden contracts with no workpool surface.

- [ ] **Step 1: Run golden tests before update**

  ```bash
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  ```

  Expected: focused golden mismatch failures showing removed workpool entries.

- [ ] **Step 2: Regenerate goldens through the owning tests**

  ```bash
  go test ./cmd/harness/contractgolden -run Golden -update -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update -count=1
  ```

- [ ] **Step 3: Verify regenerated goldens**

  ```bash
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  rg -n -i 'workpool|work pool|workpool_' cmd/harness/testdata
  ```

  Expected: both tests pass and the final `rg` exits 1 with no matches.

- [ ] **Step 4: Verify runtime rejection with a temporary binary**

  ```bash
  removal_bin="$(mktemp)"
  go build -o "$removal_bin" ./cmd/harness
  "$removal_bin" workpool status --pool wp-does-not-exist
  ```

  Expected: build succeeds; invocation exits non-zero as an unknown top-level command. Remove only the explicit `mktemp` file after recording the result.

---

### Task 5: Clean all tracked workpool documentation and replace active guidance

**Files:**
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `.agent-harness/ARCHITECTURE.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/AGENT_WORKFLOW.md`
- Modify: `.agent-harness/SUB_AGENT_PATTERNS.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/ADR.md`
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/orchestration.md`
- Modify: `docs/superpowers/issues/2026-07-17-issue-21-owned-skills-review.md`
- Modify: `docs/superpowers/plans/2026-07-06-issueops-subagent-orchestration.md`
- Modify: `docs/superpowers/plans/2026-07-08-sqlite-store-maintenance.md`
- Modify: `docs/superpowers/plans/2026-07-13-sqlite-store-hardening.md`
- Modify: `docs/superpowers/plans/2026-07-13-sqlite-store-maintenance-discovery.md`
- Modify: `docs/superpowers/specs/2026-07-06-issueops-subagent-orchestration-design.md`
- Modify: `docs/superpowers/specs/2026-07-13-sqlite-store-hardening-design.md`
- Modify: `docs/superpowers/specs/2026-07-13-sqlite-store-maintenance-discovery-design.md`

**Interfaces:**
- Consumes: current project architecture and operational guidance.
- Produces: one active model—native host concurrency for ephemeral fan-out and IssueOps child cycles/execution v1 for durable work.

- [ ] **Step 1: Remove workpool from active product maps**

  Remove it from the AGENTS planned directory map, CLI lists, README architecture/feature tables, operations command lists, testing obligations, and IssueOps workflow examples.

- [ ] **Step 2: Rewrite orchestration guidance**

  Replace D3/pilot/heartbeat/claim instructions with:

  ```text
  Ephemeral independent fan-out uses the host's native subagent concurrency controls.
  Durable delegated work uses IssueOps child cycles, isolated canonical worktrees,
  generation-fenced execution ownership, and parent accept/reject validation.
  ```

  Do not claim a Claude-specific numeric cap unless verified. Codex’s cap is host-native and does not justify a shared harness scheduler.

- [ ] **Step 3: Rewrite historical product-specific material**

  Remove old workpool decision blocks, pilot/lease instructions, and workpool store-root examples from ADR, caution, issue, plan, and spec documents. Preserve the surrounding child-cycle, IssueOps, SQLite, and subagent analysis. Where a sentence combines several state roots or orchestration modes, delete only the workpool item and repair the grammar.

- [ ] **Step 4: Record one compact removal decision**

  Keep exactly one dated ADR removal record explaining that the bounded task-pool feature was removed because it did not enforce host spawning, native Codex concurrency owns thread bounds, IssueOps child/execution v1 owns durable delegation, and existing user-state bytes are deliberately not deleted. This is the only non-plan tracked documentation location allowed to name the removed feature.

- [ ] **Step 5: Do not rewrite unrelated generic terminology**

  Do not alter generic algorithm discussion of a worker pool, database connection pool, Go worker goroutines, or host-native subagent pools unless it explicitly references the removed agent-harness product/command/state namespace.

- [ ] **Step 6: Audit all tracked documentation**

  ```bash
  rg -n -i 'workpool|work pool|workpool_' \
    --glob '*.md' --glob '*.txt' .
  ```

  Expected: matches only this implementation plan and the single compact ADR removal record. Inspect each match; no command, tool, state root, gate, pilot, lease, or worker-loop instruction may survive.

---

### Task 6: Full verification and handoff report

**Files:**
- Verify only: entire worktree

**Interfaces:**
- Consumes: completed uncommitted diff.
- Produces: evidence-backed completion report without publication or cleanup.

- [ ] **Step 1: Format and inspect the complete diff**

  ```bash
  gofmt -w \
    cmd/harness/contractcli/contract.go \
    cmd/harness/harnessapp/cli_facade.go \
    cmd/harness/harnessapp/root_command_facade.go \
    cmd/harness/harnessapp/response_contract_cli_snapshot_test.go \
    cmd/harness/harnessapp/response_contract_mcp_snapshot_test.go \
    cmd/harness/hookcli/hook_stop_contract_test.go \
    cmd/harness/mcpcli/mcp_sdk_server.go \
    cmd/harness/mcpcli/mcp_tools.go \
    cmd/harness/statecli/state_cli_maintain_test.go \
    internal/adapter/cli/usage.go \
    internal/adapter/cli/usage_test.go \
    internal/adapter/mcp/catalog.go \
    internal/adapter/mcp/catalog_test.go \
    internal/core/hookprompt/orchestration_reminder.go \
    internal/core/hookprompt/orchestration_reminder_test.go \
    internal/core/issueops/reset_legacy_process.go \
    internal/core/issueops_facade.go \
    internal/core/state/state_doctor_entry.go \
    internal/core/state/state_maintain.go
  git diff --check
  git status --short
  git diff --stat
  ```

  Confirm every changed line traces to workpool removal or the plan itself.

- [ ] **Step 2: Run focused contract gates**

  ```bash
  go test ./internal/adapter/cli ./internal/adapter/mcp ./internal/core/hookprompt ./cmd/harness/statecli ./cmd/harness/hookcli -count=1
  go test ./cmd/harness/contractgolden -run Golden -count=1
  go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
  ```

- [ ] **Step 3: Run full correctness and race gates**

  ```bash
  go test ./... -count=1
  go test -race ./... -count=1
  ```

  Do not infer the race result from the normal test result.

- [ ] **Step 4: Build outside tracked output**

  ```bash
  removal_bin="$(mktemp)"
  go build -o "$removal_bin" ./cmd/harness
  "$removal_bin" version
  unlink "$removal_bin"
  ```

- [ ] **Step 5: Run residual-reference audit**

  ```bash
  rg -n -i 'workpool|work pool|DispatchWorkPool|WorkPool|workpool_' \
    --glob '*.go' --glob '*.json' --glob '*.txt' cmd internal
  git ls-files 'internal/core/workpool/**' 'cmd/harness/workpoolcli/**' '*workpool*' '*pool_gate*'
  ```

  Expected: no live Go, golden, dedicated package, or pool-gate file remains. Repository-wide Markdown matches are limited to this plan and one compact ADR removal record.

- [ ] **Step 6: Prove boundaries were respected**

  ```bash
  git -C /Users/habin/workspace/agent-harness status --short
  git status --short --branch
  ```

  Confirm the source-worktree status is unchanged from the handoff baseline and the isolated branch has no commit or push. Do not hash or rewrite the live workpool DB after source work; simply report that no command in this plan mutates it.

- [ ] **Step 7: Report**

  Report:

  - worktree path and branch;
  - deleted public/core/integration surfaces;
  - replacement paths (native concurrency and IssueOps child cycles);
  - focused/full/race/build command receipts;
  - residual historical mentions;
  - source-worktree preservation;
  - explicit `not committed`, `not pushed`, and `runtime state untouched`.
