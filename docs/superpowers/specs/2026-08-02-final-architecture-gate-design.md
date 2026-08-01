# Issue #200 Final Architecture Gate Design

## Context

Issue #200 is the final acceptance child of #117. Its exact base is
`9883f534f50fb628ac2fa754a3bf8f80d4734989`, the parent integration HEAD after
#199 / PR #218. The current Codex process was launched with hooks disabled, so
its behavior is explicitly excluded from hook-parity evidence.

## Problem

The provisional issue title implied deleting legacy core and port packages.
The production-only Go import graph contradicts that assumption:

| Compatibility boundary | Production importers |
| --- | ---: |
| `internal/core` | 39 |
| `internal/core/issueops` | 14 |
| `internal/port` | 28 |
| `cmd/harness/issueopscli` | 1 |

Deleting any of these packages would remove active production dependencies and
could break public Go surfaces, persisted-state compatibility, or rollback
paths. A symbol-level unused search is not equivalent to package-level
caller-zero proof and is outside this issue.

## Design

This child is an evidence-first final acceptance gate. It inventories package
imports from `go list -json ./...`, excluding test-only imports, and removes a
compatibility package only when both conditions hold:

1. the package has exactly zero production importers; and
2. it owns no public, persisted, or runtime contract required by rollback.

An eligible removal also removes only legacy baseline entries caused directly
by that package. If no package qualifies, production Go files and
`internal/architecture/testdata/legacy_imports.txt` remain byte-identical.
Deletion quantity is not a success metric.

The final architecture gate reuses the existing production graph evaluator.
Passing means forbidden dependency violations, new legacy edges, and stale
legacy edges are each zero. It does not mean the legacy baseline file itself is
empty. The release, claim, reseed, resume, reconcile, publication, completion,
and preparation capability ratchets must all remain green.

## Hook-enabled acceptance

Hook validation runs in isolated state with repository-provided Codex and
Claude hook configurations enabled. It exercises native host payloads against
the built binary:

- the exact generation holder in the canonical child worktree is admitted;
- a foreign session, process receipt, or cwd is rejected;
- Codex returns its native blocking result and Claude returns its native deny
  result with the same underlying IssueOps error classification.

The current `--disable hooks` process and absence of an observed stop hook are
never cited as evidence.

## Data and contract safety

This design adds no schema, DTO, CLI, MCP, or runtime behavior. It writes only
the design, implementation plan, and Turing evidence report unless a package
becomes caller-zero before implementation. Any ambiguity preserves the package
and baseline. Rollback is the child PR revert; there is no data migration.

## Verification

The gate consists of focused architecture and capability tests, focused race,
contract and response golden tests, full unit and race suites, `go vet`, a
binary build, deterministic ten-iteration self-verification at target score 95,
isolated hook-enabled Codex/Claude smoke, and final-head GitHub Actions. Any
failure blocks the child merge.

## Completion boundary

The child PR targets `117-hexagonal-architecture-migration`. After merge, only
#200 and its branch/worktree/lifecycle are closed and cleaned. Parent #117 stays
open until the umbrella PR to `main` passes the same full and hook-enabled gates.
