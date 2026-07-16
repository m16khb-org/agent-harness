# No-change worker finish evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a claimed supervised worker that made no repository changes submit truthful, sealed completion evidence without opening a second mutation command.

**Architecture:** Add one CLI-only `--no-change` completion preset. It reads the durable handoff record, derives the current worktree head, task/dispatch identity, and the already-sealed plan path, then supplies the existing completed-result contract. Core acceptance remains the authority: it rechecks a clean worktree, exact base diff, plan existence, and worker-root containment.

**Tech Stack:** Go standard library, existing IssueOps core/handoff model, Cobra-free flag CLI tests.

## Global Constraints

- Preserve coordinator-only lifecycle commands and worker worktree fencing.
- Do not write an evidence file, create a commit, or relax core acceptance checks.
- Reject `--no-change` when callers provide changed files or a non-completed outcome.

---

### Task 1: Define the no-change CLI contract with a failing test

**Files:**
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli_test.go`
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli.go`

- [ ] **Step 1: Write the failing test**

Add a focused test that seeds a claimed Git-backed handoff and invokes the finish-request preparation helper with `NoChange: true` and one verification. Assert it derives the current `HEAD`, relative sealed `PlanPath`, task/dispatch IDs, and `no worker-created temporary resources` cleanup receipt.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./cmd/harness/issueopscli -run '^TestNoChangeHandoffFinishDefaultsSealedEvidence$' -count=1`

Expected: FAIL because the no-change request option/helper does not exist.

- [ ] **Step 3: Implement the smallest CLI-only preset**

Add `--no-change` to `issueops handoff finish`. When it is selected, read the durable record and only derive values that are already sealed or locally observable: `HEAD`, plan path relative to worker root, task ID, dispatch ID, and the exact no-temp receipt. Require a completed outcome, no changed-file flags, and at least one verification.

- [ ] **Step 4: Run the focused test to verify it passes**

Run: `go test ./cmd/harness/issueopscli -run '^TestNoChangeHandoffFinishDefaultsSealedEvidence$' -count=1`

Expected: PASS.

### Task 2: Preserve negative fences and execute E2E retry

**Files:**
- Modify: `cmd/harness/issueopscli/issueops_handoff_cli_test.go`

- [ ] **Step 1: Write failing negative tests**

Assert `--no-change` rejects changed files, a failed outcome, missing verification, a missing plan, and a plan outside the worker root.

- [ ] **Step 2: Run the focused tests to verify failures**

Run: `go test ./cmd/harness/issueopscli -run '^TestNoChangeHandoffFinish' -count=1`

Expected: FAIL before the validation branches exist.

- [ ] **Step 3: Add only the validation required by those cases**

Keep the existing core `validateHandoffAcceptEvidence` revalidation unchanged. The CLI preset must fail closed before calling finish when its derived evidence is unavailable or unsafe.

- [ ] **Step 4: Verify and commit**

Run: `go test ./cmd/harness/issueopscli -run '^TestNoChangeHandoffFinish' -count=1 && go test ./internal/core/issueops ./cmd/harness/issueopscli -count=1 && go test -race ./internal/core/issueops ./cmd/harness/issueopscli -count=1 && go vet ./... && go build -o /tmp/agent-harness-37 ./cmd/harness`

Expected: all commands pass. Commit with `fix(handoff): self-heal no-change worker finish`.

### Task 3: Close the live regression loop

**Files:**
- No source-file changes.

- [ ] **Step 1: Merge the reviewed #37 fix into the #22 branch and refresh daemon/MCP binary**

- [ ] **Step 2: Have #24 and #25 workers invoke the exact `handoff finish --no-change` command with their sealed identity and focused verification**

- [ ] **Step 3: Have each sealed coordinator accept its submitted worker result and verify both IssueOps records are `closed/accepted`**
