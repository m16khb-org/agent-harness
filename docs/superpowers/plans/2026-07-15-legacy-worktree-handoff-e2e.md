# Legacy Worktree Handoff E2E Implementation Plan

> **For agentic workers:** Execute the bounded verification task only; do not modify source files, commit, push, or create a PR.

**Goal:** Prove that IssueOps can adopt an already-created Git legacy worktree into an Orca supervised handoff and create a visible Codex worker session.

**Architecture:** The coordinator records the exact Git/Orca checkout identity, applies the current issue/attempt marker only to that worktree, then creates one worker terminal and dispatches one bounded verification task. No task is permitted to change the repository.

**Tech Stack:** Go IssueOps CLI, Orca orchestration, Git worktree, Codex worker.

## Global Constraints

- The current worktree is an existing Git checkout; do not delete, recreate, or retarget it.
- Abort on any path, branch, HEAD, repo, instance, issue, or marker mismatch.
- The worker may inspect state and submit evidence only.
- No source mutation, commit, push, PR, or cleanup removal is authorized.

### Task 1: Adopt and dispatch the existing worktree

**Files:**
- Verify only: durable IssueOps state, Orca worktree/terminal/task/dispatch inventory.

- [ ] Record the provider-linked branch head and design review.
- [ ] Run `issueops worktree prepare --orchestrator orca --confirm` and require `worktree_adopted=true` with the existing canonical path.
- [ ] Run `issueops handoff start --confirm` using the exact current coordinator recipient identity.
- [ ] Require one worker terminal, one task, and one dispatch linked to the adopted worktree before reporting success.

### Task 2: Worker-side bounded verification

**Files:**
- Verify only: `git status --short` and `issueops status --json`.

- [ ] Claim the assigned task through IssueOps.
- [ ] Verify the worktree is unchanged and report the terminal/task/dispatch IDs through the durable result path.
- [ ] Stop without source mutation if task metadata or worktree identity differs from the task packet.

## Verification

```bash
go test -race ./internal/core/issueops ./internal/adapter/orca -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```
