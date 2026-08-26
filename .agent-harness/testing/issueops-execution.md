# IssueOps v1 execution and optional Orca verification

[← TESTING.md](../TESTING.md) owns the test-strategy index. This module owns
the IssueOps execution vertical contract tests and the optional Orca
verification surface. CLI/MCP/Codex/Claude adapter parity that is not
IssueOps-execution-specific lives in
[cli-mcp-and-hosts.md](cli-mcp-and-hosts.md); race package sets are cross-cut
with [concurrency-and-race.md](concurrency-and-race.md).

`execution release` production vertical은 `internal/adapter/issueops` differential
test로 schema v1·missing/zero legacy schema·rich sidecar·holder index와 denial
atomicity를 비교한다. 변경 시 core focused race, outbound focused race,
architecture ratchet, CLI/MCP contract, golden, scoped vet와 build를 실행하며,
전체 suite는 PR CI가 회귀 증거로 담당한다.

`issueopspublication` vertical은 fixed operation ID와 fixed clock을 쓰는 frozen
legacy oracle/new vertical differential로 create·reconcile의 result JSON, error
text, record row, `external_intent_v1` row를 byte-for-byte 비교한다. Provider
create·inventory·live verification 중에는 cycle lock이 해제되어 동시 read와
replacement preview가 완료되어야 한다. CLI text, MCP `isError`, production
provider resolver caller-zero, non-test legacy full-flow 부재를 각각 adapter 및
AST ratchet 테스트로 고정한다.

Normal tests and self-verification must remain green without Orca. Use injected
workspace, provider, process, and Orca adapters for the default suite. The
current execution contract is `issueops_v1` with `schema_version=1`; legacy
rows and files are reset through the explicit fingerprint-CAS maintenance path,
never migrated or dual-read by normal execution.

Execution tests must cover:

- `direct`, explicit `orca`, and `auto`; `auto` may fall back only when the
  read-only Orca probe fails before the first external mutation.
- claimable → active → revoking/released generation transitions, the
  `lease_holder_v1` reverse index, exact native process identity, and stale
  holder/generation rejection across CLI, MCP, and hooks.
- every record-specific hook block exposes exactly `code`, `lifecycle_id`,
  `expected_root`, `current_generation`, and `next_command` in raw JSON. The
  Codex/Claude native reason string must decode to the same five-field object
  without adding fields rejected by either host's strict hook schema.
- replacement preview, revoke, and finalize with PID-reuse-safe process
  observation plus complete Orca owner inventory. A live terminal, task, or
  dispatch blocks finalization.
- a sealed context packet and owner prompt with exact digests, no raw claim
  token, no unresolved placeholder, only current catalog commands, and the
  exact ordered 14-field owner report golden.
- preparation and every Orca reseed persist artifact identity version 1 and one
  complete issue-body, packet, and prompt digest identity. Resume must survive an owner-prompt template
  upgrade without rerendering, while independent prompt, packet, issue-body,
  or stored-digest drift fails before an Orca mutation. Unversioned all-empty
  legacy bindings route through preview and generation-CAS reseed; versioned
  all-empty, unversioned-complete, partial, and future-version identities are
  invalid. Producer tests must prove new prepare and reseed outputs carry both
  the version marker and all three digests.
- Orca plan readiness tests must prove a non-empty staged `plan` before fresh
  owner evidence/mutation, atomic `plan_path` persistence with the worktree
  receipt, exact staged/sealed/durable digest equality on reseed and resume,
  and zero operation/worktree/terminal/Run/task/dispatch/lease mutations on
  failure. Released recovery staging is limited to a clean holderless Orca
  generation and changes only the next reseal input. Run the focused regressions
  with `go test ./internal/adapter/issueops ./cmd/harness/harnessapp -run 'PlanArtifact|Preparation.*Plan|Owner.*Plan|Replace.*Plan|Intent.*Plan|Resume.*Plan' -count=1`
  plus `go test ./internal/adapter/issueops ./internal/adapter/lifecycle -run 'Artifact.*Released|Released.*Artifact' -count=1`.
- completion only from `pr` with the durable verified PR/MR projection; the
  completion receipt, lease release, reverse-index deletion, and `done` phase
  transition are one atomic write. An identical retry is idempotent only when
  all terminal invariants still hold.
- completed replacement preview and reseed must test parent drift and no-drift
  against the same outbound observer. Drift must preserve the raw record,
  completion/history/ledger/lease, token paths, artifact prepare count, and
  repository commit count.
- current completion generation 0/missing is invalid in preview and reseed even
  when the request supplies a generation. No selected-generation compatibility
  fallback or legacy wording is permitted.
- released-completion sync-base tests must cover matching/missing/wrong/history
  generation, claimable/history-only state, canonical cwd, live/mismatched
  process receipt, pending intent, stale fingerprint, immutable completion, and
  exact apply/finalize/abort/retry commands. Hook matrices must run the exact
  forms for Codex and Claude and block duplicates, wrappers, shell expansion,
  wrong cwd/lifecycle, stale history generation, multiple modes, and unknown
  flags.
- architecture tests must scan every production Go file in
  `internal/port/issueopsbasesync` and allow only Request, Receipt, Inspector,
  and the `context` import.
- typed-error
  `next_command` and conflict `abort_command` cannot escape generated-command
  provenance binding. GREEN requires canonical executable, hash, and generation
  provenance on both fields, or a tested conversion to non-executable guidance.
- released sync-base production reachability must start from a claimable fixture
  and use the public execution dispatcher with the production claim and complete
  handlers before preview/apply/finalize. Direct completion/active record writes
  are allowed only for isolated gate tests and cannot serve as vertical evidence.
- generated `next_command` tests must cover every production-reachable adapter
  path. CLI covers prepare/status/replace/resume/sync-base/switch-mode preview;
  MCP covers its advertised prepare/status/replace/resume/reconcile/complete
  surface plus typed base-sync-required errors from resume/replace. Because MCP
  has no sync-base action, a success-result sync-base binder/test is dead code.
  Both adapters also cover
  execution reseed preview, cleanup finish preview/apply, exact current-binary path/hash binding,
  observation failure with no command fallback, and stale installed-binary versus
  newer worktree-binary rejection before a mutation handler is entered. The same
  envelope must decode to equivalent Codex and Claude hook decisions. A transition
  such as switch-mode apply that removes execution authority must return non-command
  guidance instead of an executable `next_command`.
- Codex exec hook tests must include its stable command-only payload shape: top-level
  turn cwd points at the source checkout and `tool_input` has no workdir. A current-
  generation absolute generated IssueOps mutation may proceed only when its exact
  `--cwd` selects the canonical worktree and the CLI independently proves that
  `os.Getwd()` matches it before mutation. Bare commands, mismatched process cwd,
  stale provenance, and delegation commands whose `--parent` identity is missing
  must remain fail-closed.

Orca external-intent tests treat worktree, terminal, Run create, Run bind, task,
and dispatch as six separate durable stages. For every stage, exercise authoritative 0, exact 1,
multiple candidates, transport failure, post-mutation crash, and CAS identity
change. Zero may invoke only with durable `not_invoked_proven` evidence and at
most one proven-not-invoked retry. The idempotent Run-bind stage may converge an
unknown outcome within the same two-attempt bound. Exact one adopts; every
ambiguous create outcome retains the intent without fallback or duplicate mutation.

The prepared runtime ID is mandatory on every terminal/task/dispatch receipt
and inventory row. Task title/display name and dispatch assignee/injection must
match the sealed intent. A terminal handle is runtime-scoped and is never
durable authority: later stages re-resolve the current handle from exact
worktree ID plus PTY. Owner quiescence uses the complete task inventory and
checks the bound dispatch independently, so a dispatched/running task cannot be
hidden by a ready-only listing.

Use this focused package set before the full repository gates:

```bash
go test ./internal/adapter/issueops/... ./internal/adapter/lifecycle ./internal/adapter/orca ./internal/adapter/codex ./internal/adapter/claude ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/mcpcli -count=1
go test -race ./internal/adapter/issueops/... ./internal/adapter/lifecycle ./internal/adapter/orca ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/hookcli/hookinput ./cmd/harness/mcpcli -count=1
```

Native activation tests use isolated temporary homes. They require same-
directory staged build, smoke, atomic rename, strict Codex/Claude semantic and
raw-file readback, and a sealed activation receipt written last. Every injected
crash before that final receipt must leave destructive legacy reset blocked.

When Orca is installed, a disposable live E2E may be added as release evidence;
it is never a default unit-test dependency. Resolve exact runtime/repo/worktree/
PTY/task/dispatch identities, never use a global reset, and remove only the
uniquely named disposable resources after an explicit cleanup decision. When
Orca is absent, prove explicit Orca mode fails before mutation and `auto`
returns the deterministic direct fallback projection.

After native installation, exercise Codex and Claude hook fixtures through the
common hook input boundary and require exact host, session, process, cwd, and
allow/block projections.
