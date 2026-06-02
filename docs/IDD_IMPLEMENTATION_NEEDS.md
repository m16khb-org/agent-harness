# IDD Implementation Needs

## Scope

Issue-Driven Development, or IDD, treats issues as the source of truth above SDD, TDD, plans, branches, worktrees, review, and PR/MR drafting. The goal is not just to create tickets. The goal is to preserve the decision structure of collaborative work so a teammate can inspect an issue and its related issues to understand the rationale behind a branch, worktree, plan, implementation, feedback loop, and PR/MR.

This document records what the current `agent-harness` codebase already supports and what still needs to be added to implement IDD as a first-class methodology.

## Method

The investigation used current repository evidence:

- `skills/issueops/SKILL.md` defines the current agent-facing workflow.
- `internal/core/issueops.go` defines durable IssueOps state.
- `cmd/harness/issueops.go` defines the CLI surface.
- `cmd/harness/main.go` routes `issueops` in the current source tree.
- `cmd/harness/issueops_test.go`, `cmd/harness/issueops_mcp_test.go`, and `internal/core/issueops_test.go` cover the basic lifecycle.
- `internal/core/issueops_benchmark.go` and `testdata/issueops/fixtures` cover benchmarked issue, worktree, TDD, subagent, cleanup, and PR/MR quality dimensions.
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
| Durable cycle state | Present but narrow. State stores `repo`, `branch`, `phase`, `issue_url`, `plan_path`, feedback items, and timestamps. | `internal/core/issueops.go` |
| CLI state commands | Present in current source. Commands include `start`, `status`, `link-issue`, `link-plan`, `feedback add`, `pr-readiness`, and benchmark commands. | `cmd/harness/issueops.go`, `internal/adapter/cli/usage.go` |
| MCP tools | Partial. Tests cover MCP start/status, and architecture docs describe matching IssueOps MCP tools. | `cmd/harness/issueops_mcp_test.go`, `.agent-harness/ARCHITECTURE.md` |
| Readiness check | Present but too shallow for IDD. It only requires `issue_url` and `plan_path`. | `internal/core/issueops.go` |
| Worktree expectations | Present as advisory/benchmark evidence, not as executable git orchestration. | `skills/issueops/SKILL.md`, `internal/core/issueops_benchmark.go` |
| Quality benchmark | Present. Deterministic scoring covers branch/worktree gate quality, isolation, cleanup, TDD, subagent orchestration, and PR/MR quality. | `internal/core/issueops_benchmark.go` |
| Provider writes | Intentionally absent. Hooks and core do not create remote issues or PR/MRs. | `skills/issueops/SKILL.md`, docs architecture |

## Missing Capabilities

### 1. Durable issue graph

Current state stores one `issue_url`, not a graph. IDD needs typed links between related issues and decisions so collaborators can traverse the decision structure.

Needed:

- `issueops link-related --type depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements`.
- A state schema for related issue nodes and typed edges.
- MCP tools with the same meaning as the CLI.
- Tests for edge validation, duplicate links, invalid URLs, and JSON contract stability.

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

Current `pr-readiness` only checks issue and plan links. IDD readiness should require the collaboration evidence that makes a PR/MR reviewable.

Needed readiness fields:

- issue graph summary present,
- at least one decision record or explicit "no decision changes" marker,
- branch/worktree verified,
- verification evidence recorded,
- feedback resolved or carried forward,
- cleanup status recorded,
- risk/reviewer notes ready.

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

## Suggested Implementation Order

1. Extend IssueOps state schema with related issues and decision records, preserving read compatibility with current records.
2. Add CLI and MCP commands for related issue links and decision records.
3. Strengthen `pr-readiness` to report missing IDD evidence while keeping the old issue/plan fields.
4. Add worktree prepare/verify/cleanup-readiness commands using policy-aware argv execution.
5. Add contract/golden tests and benchmark fixtures for linked decision graphs and worktree verification.
6. Add stale binary drift detection to `doctor` or `verify-work`.
7. Only then consider provider adapters for creating/updating remote issues and PR/MRs.

## Verification Commands Used During Investigation

```bash
go build -o /tmp/agent-harness-idd-check ./cmd/harness
/tmp/agent-harness-idd-check issueops start --repo "$PWD" --branch "idd-readme-smoke" --json
/tmp/agent-harness-idd-check issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
./bin/agent-harness issueops start --repo "$PWD" --branch "idd-readme-smoke" --json
```

Observed result: the fresh build succeeded for `issueops start` and deterministic benchmark scoring, while the checked binary printed usage and exited for `issueops`, proving binary/source drift for this command surface.
