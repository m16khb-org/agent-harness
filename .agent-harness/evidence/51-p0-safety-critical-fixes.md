# #51 P0 safety fixes — Turing evidence

## Scope

This worker changed only the #51 P0 skill material: Engelbart synthetic fixtures,
Shannon shell-measurement safety, and Torvalds bisect/clean safety. No history
rewrite, force-push, remote preflight, pull request, or golden update was performed.

## Success criteria

| Criterion | Result | Evidence |
| --- | --- | --- |
| P0-ENGELBART-FIXTURES | PASS | The six tracked fixture files carry the synthetic-fixture marker and contract tests reject personal, internal-service, and policy-shaped identifiers. |
| P0-SHANNON-SHELL | PASS | The P0 safety test covers empty diffs, no-match output, space-containing paths, and `ugrep` aliases without a false zero or shell crash contract. |
| P0-TORVALDS-SAFETY | PASS | The P0 safety test requires a quoted bisect script boundary plus clean dry-run, list, stash, and explicit-confirmation guidance. |

## Evidence artifacts

- `scripts/engelbart_skill_contract_test.py` verifies the six synthetic fixtures and
  the Engelbart contract/rubric structure.
- `scripts/engelbart_quality_rubric.py` scores the current synthetic fixture at 100,
  above its 92-point pass line; intentionally bad fixtures remain failing controls.
- `scripts/p0_skill_safety_test.py` is the regression contract for the Shannon and
  Torvalds P0 safety requirements.
- `skills/shannon/SKILL.md` uses a Bash-safe, locale-stable measurement contract
  instead of alias-sensitive pipelines or unchecked zero-output fallbacks.
- `skills/torvalds/references/bisect-protocol.md` runs `git bisect` through a quoted
  executable script path, and `skills/torvalds/SKILL.md` documents the guarded clean
  workflow.

## Verification

Passing checks:

- `python3 scripts/p0_skill_safety_test.py`
- `python3 scripts/engelbart_skill_contract_test.py`
- `python3 scripts/engelbart_quality_rubric.py`
- `python3 scripts/validate-skill.py skills/engelbart`
- `python3 scripts/validate-skill.py skills/shannon`
- `python3 scripts/validate-skill.py skills/torvalds`
- `python3 -m py_compile scripts/engelbart_quality_rubric.py scripts/engelbart_skill_contract_test.py scripts/p0_skill_safety_test.py`
- `go test ./cmd/harness/contractgolden -run Golden -count=1`
- `git diff --check`

Exact unrelated baseline failure, retained without changing a golden:

- `go test ./... -count=1` fails only at
  `cmd/harness/harnessapp.TestResponseContractsGolden`
  (`cmd/harness/harnessapp/response_contract_golden_test.go:48`) because the
  `doctor` operational snapshot depends on the current external Orca/worktree
  inventory and no longer matches `cmd/harness/testdata/response_contracts.golden.json`.
  This worker did not modify Go response contracts or the golden, so the
  non-hermetic baseline drift is recorded separately rather than normalized.

## Hopper diagnosis

The failure reproduces with the focused response-contract test and the full Go test
suite. Its diff is operational `doctor` data rather than any fixture, skill, or P0
script path changed here; updating the unrelated snapshot would conceal the
environment-dependent baseline instead of verifying this issue.

## Cleanup receipt

No subagents, remote mutations, generated binaries, stashes, history rewrites, or
force-pushes were created by this worker. The local commit contains only the files
listed by this issue's scoped status review.

## Verification mode

Focused executable contracts were run after implementation, followed by skill
validators, Python syntax compilation, the unaffected golden package, and whitespace
validation. The repository-wide Go suite was also run and its single non-hermetic
baseline failure is preserved above.

## Skipped checks

`go build -o bin/agent-harness ./cmd/harness` was not run because the supervised
handoff command policy blocks the coordinator-owned binary output path. Remote
preflight, push, pull request creation, merge, history rewrite, and force-push are
outside the #51 worker authorization and were not attempted.
