# IDD Implementation Needs

## Scope

Issue-Driven Development, or IDD, treats issues as the source of truth above SDD, TDD, plans, branches, worktrees, review, and PR/MR drafting. The goal is not just to create tickets. The goal is to preserve the decision structure of collaborative work so a teammate can inspect an issue and its related issues to understand the rationale behind a branch, worktree, plan, implementation, feedback loop, and PR/MR.

This document records what the current `agent-harness` codebase already supports and what still needs to be added to implement IDD as a first-class methodology.

> **Status (updated after the IDD implementation commit `f5bdd28`).** Most of the
> capabilities originally listed as missing are now shipped: typed issue-graph
> links (`link-related`), first-class decision records (`decision`), worktree
> orchestration (`worktree prepare/verify/cleanup-readiness`), the
> `force-release`/`force-done` escape hatches, the cycle-scoped PreToolUse guard,
> binary-drift detection in `doctor`, and opt-in GitHub/GitLab provider writes
> gated by explicit confirmation. The "Current Support" table and the per-item
> status lines below mark what is Delivered versus what genuinely Remains.

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
| Durable cycle state | Present and expanded. State stores `repo`, `branch`, `phase`, `issue_url`, `plan_path`, `worktree_path`, child issue links, provider branch preparation, feedback items, timestamps, **typed issue-graph links (`IssueLinks`), and decision records (`Decisions`)**. | `internal/core/issueops/model/types.go`, `internal/core/issueops/issueops_state.go` |
| CLI state commands | Present in current source. Commands include `start`, `status`, `link-issue`, `link-plan`, `link-worktree`, `link-child`, `link-related`, `decision`, `branch prepare`, `worktree prepare/verify/cleanup-readiness`, `feedback add`, `pr-readiness`, `force-release`, and benchmark commands. | `cmd/harness/issueopscli/issueops.go`, `internal/adapter/cli/usage.go` |
| MCP tools | Mostly present. Shipped tools include start/status, child/issue/plan/worktree linking, `issueops_link_related`, `issueops_force_release`, and provider branch preparation. Still pending: an MCP `issueops_decision` tool (decision records currently CLI-only). | `cmd/harness/mcpcli/mcp_tool_issueops.go`, `internal/adapter/mcp/issueops_lifecycle_catalog.go` |
| Readiness check | Present. Basic readiness (`IssueOpsPRReadiness`) checks 12 items: intent, design review, branch, branch_prepare, branch_link_verified, issue_url, worktree_path, plan_path, plan_exists, plan_in_worktree, ai_slop_clean, contract_feedback_issue_update — and now surfaces IDD warnings for missing decision records (`no_decision_records`) and a missing issue graph (`no_issue_graph_links`). Strict readiness (`IssueOpsStrictPRReadiness`) adds 14 more: repo, repo_git, branch_match, worktree_clean, upstream, upstream_fetch, upstream_synced, worktree_exists, plus fingerprint drift checks. Still pending: cleanup status as a first-class readiness field (cleanup is tracked separately via `cleanup`/`worktree cleanup-readiness`). | `internal/core/issueops/issueops_pr_readiness.go`, `internal/core/issueops/issueops_pr_readiness_strict.go` |
| Worktree orchestration | Present as executable commands. `issueops worktree prepare/prepare-tools/verify/cleanup-readiness` derive, verify, and report cleanup readiness for the worktree contract. Branch preparation records the required provider-linked branch order before local worktree creation. The PreToolUse guard now enforces isolation at **cycle granularity** (see item 7). | `cmd/harness/issueopscli/worktreecmd/worktree.go`, `internal/core/lifecycle/lifecycle_worktree_guard.go` |
| Quality benchmark | Present. Deterministic scoring (17 dimensions) covers branch/worktree gate quality, isolation, cleanup, TDD, subagent orchestration, and PR/MR quality. | `internal/core/issueops/benchmark/issueops_benchmark.go`, `internal/core/issueops/benchmark/issueops_benchmark_score.go` |
| Provider writes | Present as opt-in adapters. GitHub (`gh`) and GitLab (`glab`) adapters can create issues and PRs/MRs, every mutating call gated by `Confirm` (dry-run preview otherwise); core stays provider-neutral. Still pending: provider-side attachment of remote hierarchy/linked items. | `internal/adapter/provider/github/provider.go`, `internal/adapter/provider/gitlab/provider.go`, `internal/port/provider.go` |

## Capabilities: Delivered and Remaining

The items below were the original gap list. Each now carries a **Status** line:
✅ Delivered, ◐ Partially delivered, ☐ Remaining.

### 1. Durable issue graph

**Status: ◐ Partially delivered.** Typed local links and graph summaries shipped; provider-side remote hierarchy attachment is intentionally deferred (see item 5).

Current state stores the main `issue_url` and first-class child links, plus typed related links (`link-related`). IDD still needs provider adapters to mirror that local graph as remote hierarchy/linked items.

Delivered: typed `link-related` (`depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements`) in `internal/core/issueops/linking/link.go`; decision-aware graph warnings (`no_issue_graph_links`) in `pr-readiness`.

Remaining:

- Provider adapters that can create or attach remote hierarchy items after the local graph contract is stable.

### 2. First-class decision records

**Status: ◐ Partially delivered.** Decision records, impact categories, and readiness warnings shipped via the CLI; an MCP tool and content guardrails remain.

Delivered: `issueops decision` with title, body, kind, rationale, alternatives, affected issue links, and affected artifacts (`AddIssueOpsDecision` in `internal/core/issueops/issueops_decision.go`); validated impact categories (issue, plan, test, implementation, review, pr_mr, follow-up); a missing-decision warning (`no_decision_records`) in `pr-readiness`.

Remaining:

- An MCP `issueops_decision` tool (decision records are currently CLI-only).
- Guardrails against storing secrets or large private issue bodies in state.

### 3. Branch and worktree orchestration

**Status: ✅ Delivered.** All four worktree subcommands ship in `cmd/harness/issueopscli/worktreecmd/worktree.go`.

Delivered:

- `issueops worktree prepare` derives a branch name from an issue, plans a sibling worktree path, and records the base `HEAD`.
- `issueops worktree prepare-tools` provisions the worktree tooling contract.
- `issueops worktree verify` checks `pwd`, branch, `HEAD`, expected path, cleanliness, and base relation.
- `issueops worktree cleanup-readiness` reports clean/dirty and merged/unmerged without deleting automatically.
- Git execution flows through the existing command-policy boundaries rather than ad hoc shell strings.

### 4. Stronger PR/MR readiness

Basic `pr-readiness` already checks 12 items (intent, design review, branch evidence, worktree path, plan existence, ai-slop-clean, contract feedback); strict mode adds 14 more (repo git, branch match, worktree cleanliness, upstream sync, fingerprint drift). The structural gap is not the number of checks — it's what they cover. IDD readiness should also validate that the collaboration evidence backing a PR/MR is present in the cycle record.

**Status: ◐ Partially delivered.** Issue-graph and decision warnings ship; cleanup status is not yet a readiness field, and the decision marker is advisory (a warning) rather than a hard gate.

Delivered:

- issue graph summary surfaced as the `no_issue_graph_links` warning,
- decision presence surfaced as the `no_decision_records` warning,
- the stuck-cycle guard now allows release via `force-release`/`force-done` (see item 7).

Remaining:

- cleanup status recorded as a first-class `pr-readiness` field (cleanup is currently tracked via `cleanup`/`worktree cleanup-readiness`),
- an explicit "no decision changes" marker to distinguish "intentionally none" from "forgotten".

### 5. Provider boundary

IDD should integrate with GitHub/GitLab without turning agent-harness into a full provider client too early.

**Status: ✅ Delivered (initial adapters).** Optional GitHub/GitLab write adapters now exist, gated by explicit confirmation; core stays provider-neutral.

Delivered:

- Core stays provider-neutral; remote URLs and provider metadata are accepted as data.
- Optional `gh`/`glab` adapters (`internal/adapter/provider/{github,gitlab}`) implement the `port.IssueProvider` interface for issue and PR/MR creation.
- Every mutating call requires `Confirm=true`; without it the adapter returns a dry-run preview, satisfying the explicit-approval requirement.

Remaining:

- Provider-side attachment of remote hierarchy/linked items (mirrors item 1).

### 6. Binary drift check

The current checked `bin/agent-harness` can drift from source. During this investigation, the checked binary usage omitted `issueops`, while a fresh source build exposed it.

**Status: ✅ Delivered (doctor check).** `doctor` now flags stale binaries; README guidance and self-verify coverage remain optional follow-ups.

Delivered:

- `doctor.checkBinaryDrift` (`internal/core/doctor/checks.go`) compares `bin/agent-harness` mtime against the latest source change and raises a `binary_drift` warning with a `go build -o bin/agent-harness ./cmd/harness` fix.

Remaining (optional):

- README guidance that command-surface verification should use a freshly built binary before claiming a feature is unavailable.
- Self-verify coverage for stale binary command-surface drift.

### 7. Repo-global mutating lock → cycle-scoped guard + force-release escape hatch

**Status: ✅ Delivered (A, B, and C).** The escape hatch, the cycle-scoped guard, and the optional `done`-phase skip all ship. The "Root cause" and "Deadlock scenario" below describe the **pre-fix** behavior and are retained for historical context.

Delivered:

- **A.** `issueops force-release --id <id> --reason "..."` (`ForceReleaseIssueOps`) transitions a stuck cycle to `done`, requires a reason, and records it; also exposed as the MCP tool `issueops_force_release`.
- **B.** The guard is now cycle-scoped: when the current branch has no active cycle, `worktreeGuardBlockReason` returns no block (`lifecycle_worktree_guard.go` lines 37-43), so a cycle stuck on another branch no longer deadlocks repo-wide edits.
- **C.** `ForceDoneIssueOps` advances a PR-phase cycle to `done` while recording the skipped remote-artifact verification reason, removing the systemic deadlock path.

Historical context (pre-fix):

The PreToolUse hook guard (`internal/core/lifecycle/lifecycle_worktree_guard.go`) originally enforced worktree isolation by blocking mutating tool calls outside a linked worktree when **any** active IssueOps cycle existed for the repo — at repo granularity, not cycle granularity — so it blocked edits to files unrelated to the blocking cycle.

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

> Steps 1–6 are largely complete as of `f5bdd28` (force-release, cycle-scoped guard,
> state schema for graph + decisions, CLI/MCP link-related, IDD readiness warnings,
> worktree commands). The remaining work is steps 7–9 plus the per-item "Remaining"
> bullets above: MCP `issueops_decision`, decision content guardrails, cleanup status
> as a readiness field, stale-binary self-verify coverage, and provider remote hierarchy.

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
