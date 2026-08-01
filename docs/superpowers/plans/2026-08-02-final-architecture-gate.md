# Issue #200 Final Architecture Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the #117 migration's final caller-zero, architecture, contract, and hook-enabled acceptance gates without deleting active compatibility packages.

**Architecture:** Treat the production Go import graph as the deletion authority and the existing architecture fitness test as the dependency authority. Preserve every package with a production importer or contract ownership, then record final evidence in tracked design/plan/Turing documents. Validate hooks through isolated enabled host configurations rather than the current disabled Codex process.

**Tech Stack:** Go 1.26.3, `go list`, Go tests/race/vet/build, agent-harness IssueOps/self-verify, Codex and Claude native hook JSON.

## Global Constraints

- Exact base is `9883f534f50fb628ac2fa754a3bf8f80d4734989`.
- Child PR target is `117-hexagonal-architecture-migration`.
- Do not remove packages with production importers or uncertain contract ownership.
- Do not perform symbol-level dead-code cleanup.
- Do not change schema v1, legacy persisted bytes, CLI/MCP JSON, error codes, or hook deny semantics.
- Do not use the current `--disable hooks` session as hook evidence.
- Do not modify `openwiki/**`.

---

### Task 1: Seal caller-zero inventory and deletion decision

**Files:**
- Inspect: `internal/architecture/dependency_test.go`
- Inspect: `internal/architecture/testdata/legacy_imports.txt`
- Create later: `.agent-harness/turing/issue200-report.md`

**Interfaces:**
- Consumes: production package `ImportPath` and `Imports` from `go list -json ./...`.
- Produces: exact importer lists for `internal/core`, `internal/core/issueops`, `internal/port`, and `cmd/harness/issueopscli`, plus an explicit removal set.

- [ ] **Step 1: Capture the production graph twice**

  Run `go list -json ./...` twice from the canonical worktree and store only temporary JSON outside the repository.

- [ ] **Step 2: Compare normalized inventories**

  Normalize repository-local `ImportPath` and `Imports`, sort them, and require identical SHA-256 digests. Expected: identical digests.

- [ ] **Step 3: Enumerate candidate importers**

  Use `jq -s` over the `go list` stream to list production importers of the four compatibility boundaries. Expected at the sealed base: counts `39`, `14`, `28`, and `1` respectively.

- [ ] **Step 4: Decide the removal set**

  Require both zero production importers and no public/persisted/runtime contract ownership. Expected: empty removal set; therefore do not edit production Go or `legacy_imports.txt`.

- [ ] **Step 5: Run the architecture gate**

  Run `go test ./internal/architecture -count=1 -v`. Expected: PASS, including `TestProductionGraphMatchesBaseline` and every production routing/caller ratchet.

### Task 2: Commit the approved design and plan

**Files:**
- Create: `docs/superpowers/specs/2026-08-02-final-architecture-gate-design.md`
- Create: `docs/superpowers/plans/2026-08-02-final-architecture-gate.md`

**Interfaces:**
- Consumes: approved #200 remote contract and Task 1 inventory.
- Produces: reviewable, rollback-safe design and executable verification plan.

- [ ] **Step 1: Materialize staged artifacts into tracked paths**

  Copy the exact approved spec and plan content using a reviewed patch in the canonical worktree.

- [ ] **Step 2: Self-review the spec and plan**

  Search for placeholder markers, incomplete instructions, contradictions, scope expansion, and incorrect paths. Expected: no placeholders or ambiguity.

- [ ] **Step 3: Verify the documentation diff**

  Run `git diff --check` and inspect `git diff --name-only`. Expected: only the two approved document paths.

- [ ] **Step 4: Commit atomically**

  Stage only the two documents and commit with the repository's Conventional Commit plus Lore body policy.

### Task 3: Prove active-hook Codex and Claude parity

**Files:**
- Inspect: `configs/codex/hooks.json`
- Inspect: `configs/claude/hooks.settings.json`
- Inspect: `cmd/harness/hookcli/**`
- Update later: `.agent-harness/turing/issue200-report.md`

**Interfaces:**
- Consumes: canonical worktree, active generation-1 holder receipt, built child binary, and repository hook configs.
- Produces: isolated allow/deny receipts for both host payload schemas.

- [ ] **Step 1: Create isolated hook state**

  Create an exact temporary directory with `mktemp -d`, install or copy only the child binary and repository hook configs, and point `HARNESS_STATE_DIR` to an isolated state copy representing the active #200 lifecycle.

- [ ] **Step 2: Exercise exact-holder payloads**

  Send native Codex and Claude pre-tool payloads for a harmless mutation-shaped command in the canonical worktree using the exact session/process/generation holder. Expected: both allow.

- [ ] **Step 3: Exercise foreign-holder payloads**

  Change one identity field at a time for session, process receipt, and cwd. Expected: Codex native block and Claude native deny with underlying `holder_identity_mismatch` or exact canonical-worktree denial.

- [ ] **Step 4: Verify disabled-host exclusion**

  Record that the parent Codex process was launched with hooks disabled and that none of its missing hook events are evidence.

- [ ] **Step 5: Remove isolated resources**

  Move the exact temporary directory to Trash and record the recoverable cleanup path.

### Task 4: Run final verification and publish the child

**Files:**
- Create: `.agent-harness/turing/issue200-report.md`
- Verify: all files changed from exact parent base.

**Interfaces:**
- Consumes: Tasks 1–3 evidence and the final committed diff.
- Produces: AC-200-01..09 report, final child HEAD, draft PR, CI readback, merge, and IssueOps cleanup receipt.

- [ ] **Step 1: Run focused gates**

  Run the architecture, capability unit/race, contract golden, and response golden commands from issue #200. Expected: all PASS.

- [ ] **Step 2: Run full local gates**

  Run `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and `go build -o bin/agent-harness ./cmd/harness`. Expected: all PASS.

- [ ] **Step 3: Run deterministic self-verification**

  Run `./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json`. Expected: ten successful iterations, minimum goal score at least 95, no gaps.

- [ ] **Step 4: Record final evidence report**

  Map AC-200-01..09 to exact commands, outputs, commit SHA, hook receipts, and cleanup inventory in `.agent-harness/turing/issue200-report.md`; commit it atomically.

- [ ] **Step 5: Run independent implementation review**

  Dispatch one fresh adversarial reviewer under the repository's verified pattern only after the final diff is committed. Require unconditional PASS and record the diff fingerprint and evidence.

- [ ] **Step 6: Publish and verify**

  Push the child branch, create a draft PR targeting `117-hexagonal-architecture-migration`, wait for final-head GitHub Actions, and require every check green.

- [ ] **Step 7: Complete and merge the child**

  Record IssueOps completion with the exact generation and final HEAD, mark the PR ready, merge it, reflect completion into #200, close #200, and clean only its branch/worktree/lifecycle after preview/fingerprint checks.

- [ ] **Step 8: Continue the umbrella**

  Update #117 with #199 and #200 completion evidence, then create and verify the umbrella PR to `main` without closing #117 until that PR is merged.
