# Issue #52 P1 evidence report

## Scope and final contract

This report covers only the ten P1 correctness repairs on branch
`52-p1-pioneer-skill-correctness`. The implementation checkpoint is
`f4f16562292bac491fe764b7bba039f6f605a414`; this evidence-only follow-up does
not change the P1 implementation, tests, CLI behavior, or draft PR body.

## P1 evidence mapping

| P1 item | Evidence file or contract |
| --- | --- |
| P1-1 | `skills/berners-lee/SKILL.md`: canonical sub-agent pattern slugs |
| P1-2 | `skills/brooks/SKILL.md`, `internal/adapter/cli/usage.go`, and `cmd/harness/testdata/usage.golden.txt`: IssueOps review command and usage contract |
| P1-3 | `skills/karpathy/SKILL.md`: Shannon scope limited to generated code artifacts |
| P1-4, P1-5, P1-7 | `skills/turing/SKILL.md`: concise handoff reference, proportionate evidence contract, and verified platform tooling |
| P1-6 | `skills/torvalds/references/rebase-protocol.md`: backup refs persist until explicit bounded cleanup |
| P1-8 | `skills/von-neumann/SKILL.md`: host-neutral delegation wording |
| P1-9 | `skills/hopper/SKILL.md` and `internal/core/skillcontract/skill_contract_test.go`: four strategies and registered candidate/routing contract |
| P1-10 | `skills/dijkstra/SKILL.md`: repaired scaling-test fenced block |

## Verification receipts

The following commands passed in this worker worktree:

```text
python3 scripts/validate-skill.py skills/berners-lee
python3 scripts/validate-skill.py skills/brooks
python3 scripts/validate-skill.py skills/karpathy
python3 scripts/validate-skill.py skills/turing
python3 scripts/validate-skill.py skills/torvalds
python3 scripts/validate-skill.py skills/von-neumann
python3 scripts/validate-skill.py skills/hopper
python3 scripts/validate-skill.py skills/dijkstra
go test ./internal/core/skillcontract -count=1
go test ./cmd/harness/contractgolden ./cmd/harness/issueopscli -count=1
git diff --check 3cc627a328da098c16c5991f7a92063492d6850e f4f16562292bac491fe764b7bba039f6f605a414
```

`go test ./... -count=1` was also run. It fails at
`cmd/harness/harnessapp:TestResponseContractsGolden` with a
`response_contracts.golden.json` mismatch; the focused P1 contract and usage
test suites above pass.

## Cleanup receipt

No temporary files, processes, history rewrites, force pushes, merges, or
scope-expanding implementation changes were performed by this P1 work item.
