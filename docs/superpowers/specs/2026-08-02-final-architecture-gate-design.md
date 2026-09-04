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
| `internal/core` | 41 |
| `internal/core/issueops` | 15 |
| `internal/port` | 28 |
| `cmd/issueops/issueopscli` | 1 |

The normalized inventory digest is
`bf3e95dded1a56286d43cb2b39aaf0607f0fd959053a3fb3c380e53cbe9fa2a1`
for both captures. The complete reverse-import paths are part of the deletion
decision, not an optional summary:

<details><summary><code>internal/core</code> importers (41)</summary>

- `cmd/issueops/apidoc/reviewfiles`
- `cmd/issueops/apidoc/reviewprompt`
- `cmd/issueops/basiccli`
- `cmd/issueops/draftwikicli`
- `cmd/issueops/issueopsapp`
- `cmd/issueops/hookcli`
- `cmd/issueops/hookcli/hookcatalog`
- `cmd/issueops/hookcli/hookfailure`
- `cmd/issueops/installcli`
- `cmd/issueops/issueopscli`
- `cmd/issueops/issueopscli/benchmarkartifact`
- `cmd/issueops/issueopscli/benchmarkcmd`
- `cmd/issueops/issueopscli/feedbackcleanup`
- `cmd/issueops/issueopscli/remotecmd`
- `cmd/issueops/issueopscli/remoteverify`
- `cmd/issueops/loopcli`
- `cmd/issueops/mcpcli`
- `cmd/issueops/mcpcli/resources`
- `cmd/issueops/pathutil`
- `cmd/issueops/policycli`
- `cmd/issueops/projectcli`
- `cmd/issueops/qualitycli`
- `cmd/issueops/selfworkflow/augmentcatalog`
- `cmd/issueops/selfworkflow/augmentlesson`
- `cmd/issueops/selfworkflow/augmentplan`
- `cmd/issueops/selfworkflow/candidateexport`
- `cmd/issueops/selfworkflow/historycompare`
- `cmd/issueops/selfworkflow/llmeval`
- `cmd/issueops/selfworkflow/stateio`
- `cmd/issueops/statecli`
- `cmd/issueops/statuscli`
- `cmd/issueops/validationcli/candidateexport`
- `cmd/issueops/validationcli/commandpolicy`
- `cmd/issueops/validationcli/contractauditworker`
- `cmd/issueops/validationcli/nativeintegration`
- `cmd/issueops/validationcli/parallelisolation`
- `cmd/issueops/validationcli/preflightfuzz`
- `cmd/issueops/validationcli/qagate`
- `cmd/issueops/validationcli/smoke`
- `cmd/issueops/validationcli/stateroundtrip`
- `cmd/issueops/workercli`

</details>

<details><summary><code>internal/core/issueops</code> importers (15)</summary>

- `cmd/issueops/issueopsapp`
- `cmd/issueops/hookcli`
- `cmd/issueops/issueopscli`
- `cmd/issueops/issueopscli/executioncmd`
- `cmd/issueops/issueopscli/feedbackcleanup`
- `cmd/issueops/issueopscli/remotecmd`
- `cmd/issueops/mcpcli`
- `internal/adapter/inbound/issueopscompletion`
- `internal/adapter/inbound/issueopslease`
- `internal/adapter/inbound/issueopspreparation`
- `internal/adapter/inbound/issueopspublication`
- `internal/adapter/operationalhealth`
- `internal/core`
- `internal/core/hookprompt`
- `internal/core/lifecycle`

</details>

<details><summary><code>internal/port</code> importers (28)</summary>

- `cmd/issueops/issueopsapp`
- `cmd/issueops/installcli`
- `cmd/issueops/issueopscli`
- `cmd/issueops/issueopscli/executioncmd`
- `cmd/issueops/mcpcli`
- `internal/adapter/claude`
- `internal/adapter/codex`
- `internal/adapter/gitworktree`
- `internal/adapter/hostprobe`
- `internal/adapter/inbound/issueopspublication`
- `internal/adapter/installutil`
- `internal/adapter/operationalhealth`
- `internal/adapter/orca`
- `internal/adapter/outbound/issueopscompletion`
- `internal/adapter/outbound/issueopslease`
- `internal/adapter/outbound/issueopspreparation`
- `internal/adapter/outbound/issueopspublication`
- `internal/adapter/provider`
- `internal/adapter/provider/github`
- `internal/adapter/provider/gitlab`
- `internal/adapter/provider/issuebody`
- `internal/application/issueopslease`
- `internal/core`
- `internal/core/install`
- `internal/core/issueops`
- `internal/core/issueops/cleanupchildren`
- `internal/core/sqlstore`
- `internal/core/toolconformance`

</details>

<details><summary><code>cmd/issueops/issueopscli</code> importers (1)</summary>

- `cmd/issueops/issueopsapp`

</details>

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

Hook acceptance has two independent proof layers against the built child
binary:

1. A direct CLI matrix uses one isolated fixture per host and complete native
   Codex and Claude payloads, including transcript metadata. Each fixture
   projects the same generation holder under the payload's native host. The
   exact holder in the canonical child worktree must be admitted; changing the
   session or process receipt one field at a time must return the native Codex
   block or Claude deny with the same underlying IssueOps classification. An
   unrelated cwd without the exact holder identity is also denied when it
   targets the canonical worktree. The existing source-cwd compatibility stays
   intact: the exact holder may name an explicit canonical target while the
   host session cwd is the source checkout.
2. Fresh host processes use the default IssueOps state so their hook child
   processes can observe the real active lease. Codex loads the exact ignored
   project `.codex/hooks.json` copied from `configs/codex/hooks.json`, reports
   the discovered hook through `hooks/list`, and runs with `--enable hooks`
   plus explicit non-interactive trust. Claude loads
   `configs/claude/hooks.settings.json` through `--settings` and emits hook
   lifecycle events with `--include-hook-events`. Each fresh foreign host is
   instructed to attempt one mutation-shaped sentinel command; the hook must
   be observed and the sentinel must remain absent.

No host-runtime probe uses an isolated `ISSUEOPS_STATE_DIR`: native hook child
processes do not inherit such state unless explicitly propagated. No probe uses
`--disable hooks`, Claude `--safe-mode`, or Claude `--bare`.

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
direct native-payload smoke, fresh configured-host Codex/Claude smoke, and
final-head GitHub Actions. Any
failure blocks the child merge.

## Completion boundary

The child PR targets `117-hexagonal-architecture-migration`. After merge, only
#200 and its branch/worktree/lifecycle are closed and cleaned. Parent #117 stays
open until the umbrella PR to `main` passes the same full and hook-enabled gates.
