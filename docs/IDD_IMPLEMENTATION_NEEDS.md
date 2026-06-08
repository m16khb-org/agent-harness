# IDD Implementation Needs

## Scope

Issue-Driven Development, or IDD, treats issues as the source of truth above SDD, TDD, plans, branches, worktrees, review, and PR/MR drafting. The goal is not just to create tickets. The goal is to preserve the decision structure of collaborative work so a teammate can inspect an issue and its related issues to understand the rationale behind a branch, worktree, plan, implementation, feedback loop, and PR/MR.

This document records what the current `agent-harness` codebase already supports and what still needs to be added to implement IDD as a first-class methodology.

## Method

The investigation used current repository evidence:

- `skills/issueops/SKILL.md` defines the current agent-facing workflow.
- `internal/core/issueops/` (state, readiness, phase, feedback, linking, active, benchmark) defines durable IssueOps state. Types live in `internal/core/issueops/model/types.go`.
- `cmd/harness/issueopscli/` defines the CLI surface; `cmd/harness/mcpcli/` defines MCP tool handlers.
- `cmd/harness/harnessapp/root_command_facade.go` routes `issueops` via `runIssueOps`.
- Tests are scattered: `cmd/harness/issueopscli/issueops_test.go`, `cmd/harness/mcpcli/issueops/issueops_test.go`, `internal/core/issueops/issueops_test.go`, and lifecycle/readiness/phase test files.
- `internal/core/issueops/benchmark/` and `testdata/issueops/fixtures` cover benchmarked issue, worktree, TDD, subagent, cleanup, and PR/MR quality dimensions.
- `internal/core/lifecycle/lifecycle_worktree_guard.go` is the PreToolUse hook guard that enforces worktree isolation for mutating tool calls.
- A fresh `/tmp` build of the current source successfully ran `issueops start` and `issueops benchmark run --judge none`.
- The checked `bin/agent-harness` binary is older than the current source for this command surface: its usage output omitted `issueops`, while the freshly built binary accepted it.

## IDD Contract

IDD should make these records durable and inspectable:

1. Issue contract: problem, current evidence, acceptance criteria, non-goals, verification, open decisions, and related issues.
2. Decision record: each meaningful product, architecture, implementation, test, review, or scope decision that changes the issue contract.
3. Issue graph: typed links between issues, such as depends-on, blocks, supersedes, follows-up, duplicates, splits-from, and implements.
4. Branch/worktree contract: issue-derived branch, expected worktree path, `HEAD`, base branch, workspace root, and cleanup status.
5. Plan contract: plan path, issue link, decision links used by the plan, verification commands, and ownership boundaries.
6. Feedback contract: source, body, classification, decision impact, and whether it updated the issue, plan, test, implementation, or PR/MR.
7. PR/MR readiness contract: issue link, plan link, linked decision graph summary, worktree verification, test evidence, risk, cleanup status, and reviewer notes.

## Current Support

| Area | Current state | Evidence |
| --- | --- | --- |
| Advisory workflow | Present. The `issueops` skill describes problem intake, issue contract, planning, TDD/subagent implementation, feedback, and PR/MR drafting. | `skills/issueops/SKILL.md` |
| Durable cycle state | Present but still limited. State stores `repo`, `branch`, `phase`, `issue_url`, `plan_path`, `worktree_path`, child issue links, provider branch preparation, feedback items, and timestamps. No decision records or typed issue graph links yet. | `internal/core/issueops/model/types.go`, `internal/core/issueops/issueops_state.go` |
| CLI state commands | Present in current source. Commands include `start`, `status`, `link-issue`, `link-plan`, `link-worktree`, `link-child`, `branch prepare`, `feedback add`, `pr-readiness`, and benchmark commands. | `cmd/harness/issueopscli/issueops.go`, `internal/adapter/cli/usage.go` |
| MCP tools | Partial. Tests cover MCP start/status, child linking, and provider branch preparation; architecture docs describe matching IssueOps MCP tools. | `cmd/harness/mcpcli/issueops/issueops_test.go`, `.agent-harness/ARCHITECTURE.md` |
| Readiness check | Present. Basic readiness (`IssueOpsPRReadiness`) checks 12 items: intent, design review, branch, branch_prepare, branch_link_verified, issue_url, worktree_path, plan_path, plan_exists, plan_in_worktree, ai_slop_clean, contract_feedback_issue_update. Strict readiness (`IssueOpsStrictPRReadiness`) adds 14 more: repo, repo_git, branch_match, worktree_clean, upstream, upstream_fetch, upstream_synced, worktree_exists, plus fingerprint drift checks. Missing: issue graph summary, decision records, cleanup status. | `internal/core/issueops/issueops_pr_readiness.go`, `internal/core/issueops/issueops_pr_readiness_strict.go` |
| Worktree expectations | Present as advisory/benchmark evidence, not as executable git orchestration. Branch preparation now records the required provider-linked branch order before local worktree creation. The PreToolUse guard enforces worktree isolation but at repo granularity (see item 7). | `skills/issueops/SKILL.md`, `internal/core/issueops/benchmark/`, `internal/core/lifecycle/lifecycle_worktree_guard.go` |
| Quality benchmark | Present. Deterministic scoring (17 dimensions) covers branch/worktree gate quality, isolation, cleanup, TDD, subagent orchestration, and PR/MR quality. | `internal/core/issueops/benchmark/issueops_benchmark.go`, `internal/core/issueops/benchmark/issueops_benchmark_score.go` |
| Provider writes | Intentionally absent. Hooks and core do not create remote issues or PR/MRs. | `skills/issueops/SKILL.md`, docs architecture |

## Missing Capabilities

### 1. Durable issue graph

Current state stores the main `issue_url` and first-class child links, but not a complete typed graph. IDD still needs richer typed links between related issues and decisions so collaborators can traverse the decision structure.

Needed:

- `issueops link-related --type depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements`.
- Decision-aware issue graph summaries in `status` and `pr-readiness`.
- Provider adapters that can create or attach remote hierarchy items after the local graph contract is stable.

### 2. First-class decision records

Feedback is currently stored as source/body/timestamp. IDD needs decisions to be explicit records with classification and impact.

Needed:

- `issueops decision add` with fields for title, body, kind, rationale, alternatives, affected issue links, and affected artifacts.
- Decision impact categories: issue, plan, test, implementation, review, PR/MR, follow-up.
- A short decision summary in `issueops status` and `pr-readiness`.
- Guardrails against storing secrets or large private issue bodies in state.

### 3. Branch and worktree orchestration

The benchmark can score worktree evidence, but the harness does not yet create or verify the worktree contract as a durable state transition.

Needed:

- `issueops worktree prepare` to derive a branch name from an issue, create a sibling `<repo>.worktrees/<branch-slug>` path, and record the base `HEAD`.
- `issueops worktree verify` to check `pwd`, branch, `HEAD`, expected path, cleanliness, and base relation.
- `issueops worktree cleanup-readiness` to report clean/dirty, merged/unmerged, and removal choices without deleting automatically.
- Policy-gated git execution through existing command policy boundaries, not ad hoc shell strings.

### 4. Stronger PR/MR readiness

Basic `pr-readiness` already checks 12 items (intent, design review, branch evidence, worktree path, plan existence, ai-slop-clean, contract feedback); strict mode adds 14 more (repo git, branch match, worktree cleanliness, upstream sync, fingerprint drift). The structural gap is not the number of checks — it's what they cover. IDD readiness should also validate that the collaboration evidence backing a PR/MR is present in the cycle record.

Needed readiness fields currently missing from both basic and strict:

- issue graph summary present,
- at least one decision record or explicit "no decision changes" marker,
- cleanup status recorded,
- the stuck-cycle guard allows the cycle to be released (see item 7).

### 5. Provider boundary

IDD should integrate with GitHub/GitLab without turning agent-harness into a full provider client too early.

Needed:

- Keep core provider-neutral.
- Accept remote URLs and provider metadata as data.
- Add optional adapters only after the local issue graph and worktree contract are stable.
- Require explicit user approval before creating/updating remote issues, PRs, or MRs.

### 6. Binary drift check

The current checked `bin/agent-harness` can drift from source. During this investigation, the checked binary usage omitted `issueops`, while a fresh source build exposed it.

Needed:

- A doctor or verify-work finding when `bin/agent-harness` usage differs from `go run ./cmd/harness` or a fresh temp build.
- README guidance that command-surface verification should use a freshly built binary before claiming a feature is unavailable.
- Optional self-verify coverage for stale binary command-surface drift.

### 7. Repo-global mutating lock → cycle-scoped guard + force-release escape hatch

The PreToolUse hook guard (`internal/core/lifecycle/lifecycle_worktree_guard.go`) enforces worktree isolation by blocking mutating tool calls outside a linked worktree when any active IssueOps cycle exists for the repo. This guard operates at **repo granularity, not cycle granularity**: it blocks edits to files that have no relationship to the blocking cycle.

#### Root cause

`worktreeGuardBlockReason` (line 37-48): when the current branch has no active cycle (`!ok`) but `ActiveIssueOpsLinkedWorktreeCyclesForRepo` returns any non-done cycle with a linked worktree, **every file inside the repo** (`sourceCheckoutTargetNeedsLinkedWorktree` returns `true` for any path under the repo root) is blocked unless it falls inside one of those worktrees.

`sourceCheckoutTargetNeedsLinkedWorktree` (line 95-104) is the key: it categorically considers any target inside the repo root as "needs linked worktree." It does not inspect whether the target file is actually part of any cycle's scope.

#### Deadlock scenario

```
#2399 cycle → implement → PR phase → strict readiness: missing remote_artifact verification
    → done phase rejected (remote_artifact required)
    → cleanup also blocked (merged=false)
    → cycle stays active with linked worktree
    → ALL mutating edits in the main repo are blocked — even dbhub.toml, README, or any file unrelated to #2399
    → agent can't even bootstrap a new cycle to fix the stuck one
```

The deadlock's structural cause: `done` phase requires `remote_artifact` verification, but the harness intentionally has no provider adapters to perform that verification (item 5). The cycle is stuck permanently.

#### Needed

**A. `issueops force-release` command (immediate escape hatch):**

- `issueops force-release --id <id> [--reason "..."]` that transitions a stuck cycle to `done` regardless of phase gate requirements.
- Must be explicit and deliberate: requires `--reason`, logs the release in the cycle record.
- Also available as MCP tool: `issueops_force_release`.
- This is the minimum viable fix — without it, a single stuck cycle deadlocks the repo.

**B. Cycle-scoped guard (structural fix):**

- Replace the binary "any active cycle → block all repo edits" logic with scope-aware checking.
- A mutating target is only blocked if it belongs to an active cycle's scope AND is outside that cycle's linked worktree.
- Scope determination: plan file references, explicit scope declaration in cycle state, or fallback to the current "target inside worktree" check for cycles that have worktrees linked.
- Files with no relationship to any active cycle are never blocked.

**C. Make `remote_artifact` verification optional for `done` phase:**

- Allow `--force` on `issueops phase --to done` or `issueops verify-artifact` with a `skip=true` flag.
- Record the skip reason in the cycle state so the decision is auditable.
- This removes the systemic deadlock path that creates the problem in the first place.

## Suggested Implementation Order

1. **Add `issueops force-release`** to unblock stuck cycles (item 7). This is the most operationally urgent gap: a single stuck cycle deadlocks the entire repo.
2. **Make the PreToolUse guard cycle-scoped instead of repo-global** (item 7). After force-release exists as an escape hatch, refine the guard so unrelated files aren't blocked.
3. Extend IssueOps state schema with related issues and decision records, preserving read compatibility with current records (items 1, 2).
4. Add CLI and MCP commands for related issue links and decision records (items 1, 2).
5. Strengthen `pr-readiness` to report missing IDD evidence: issue graph summary, decision records, cleanup status (item 4).
6. Add worktree prepare/verify/cleanup-readiness commands using policy-aware argv execution (item 3).
7. Add contract/golden tests and benchmark fixtures for linked decision graphs and worktree verification.
8. Add stale binary drift detection to `doctor` or `verify-work` (item 6).
9. Only then consider provider adapters for creating/updating remote issues and PR/MRs (item 5).

## Verification Commands Used During Investigation

```bash
go build -o /tmp/agent-harness-idd-check ./cmd/harness
/tmp/agent-harness-idd-check issueops start --repo "$PWD" --branch "idd-readme-smoke" --json
/tmp/agent-harness-idd-check issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
./bin/agent-harness issueops start --repo "$PWD" --branch "idd-readme-smoke" --json
```

Observed result: the fresh build succeeded for `issueops start` and deterministic benchmark scoring, while the checked binary printed usage and exited for `issueops`, proving binary/source drift for this command surface.
