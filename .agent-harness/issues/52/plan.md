# #52 P1 Pioneer Contract and CLI Correctness Plan

**Issue:** https://github.com/m16khb/agent-harness/issues/52
**Cycle:** `io-087d18993969`
**Base:** `main` at `8acdf260c906d7cf0b2bd93cfd50590904b9d920`
**Scope:** Correct only the finite factual source/CLI contract defects listed in #52. PR #64 commit `f4f16562292bac491fe764b7bba039f6f605a414` is patch evidence, not a cherry-pick target.

## Success contract

- CLI usage exposes the executable Brooks IssueOps devil's-advocate command and its golden stays synchronized.
- Berners-Lee, Brooks, Dijkstra, Hopper, Karpathy, Torvalds, Turing, and Von Neumann source claims match current repository/runtime contracts.
- Turing has one ownership/evidence boundary, Karpathy checks tool availability and names real harness surfaces, and tracked prompts are distinguished from ignored measurement evidence.
- The quality dashboard states the historical measurement and current loader counts without conflating them.
- Routing topology, new runtime delegation, scorer/benchmark work, and unrelated refactors remain untouched.

## Implementation

1. Add a named RED contract in `internal/core/skillcontract/skill_contract_test.go` covering every accepted #52 factual correction, including artifact-policy wording. Cross-check dashboard fixture wording against the loader-derived count contract instead of duplicating an independent hard-coded count.
   - Verify RED: `go test ./internal/core/skillcontract -run TestP1PioneerCorrectnessContracts -count=1` must fail for the expected missing or stale claims before production edits.
2. Add/update the CLI expectation in `internal/adapter/cli/usage_test.go`, then minimally update `internal/adapter/cli/usage.go` and `cmd/harness/testdata/usage.golden.txt` to expose the real devil's-advocate command.
   - Verify: `go test ./internal/adapter/cli -count=1` and `go test ./cmd/harness/contractgolden -run Golden -count=1`.
3. Compare each hunk from `f4f16562292bac491fe764b7bba039f6f605a414` with current main; apply only still-valid factual corrections to:
   - `skills/berners-lee/SKILL.md`
   - `skills/brooks/SKILL.md`
   - `skills/dijkstra/SKILL.md`
   - `skills/hopper/SKILL.md`
   - `skills/karpathy/SKILL.md`
   - `skills/torvalds/references/rebase-protocol.md`
   - `skills/turing/SKILL.md`
   - `skills/von-neumann/SKILL.md`
4. Make the additional current-issue corrections in `skills/karpathy/SKILL.md`, `skills/turing/SKILL.md`, and `.agent-harness/operations/quality-dashboard.md`; do not touch #55 routing ownership.
5. Run the named GREEN test, changed-skill validators, focused contracts, and full suite:
   - `python3 scripts/validate-skill.py skills/berners-lee`
   - `python3 scripts/validate-skill.py skills/brooks`
   - `python3 scripts/validate-skill.py skills/dijkstra`
   - `python3 scripts/validate-skill.py skills/hopper`
   - `python3 scripts/validate-skill.py skills/karpathy`
   - `python3 scripts/validate-skill.py skills/torvalds`
   - `python3 scripts/validate-skill.py skills/turing`
   - `python3 scripts/validate-skill.py skills/von-neumann`
   - `go test ./internal/core/skillcontract -count=1`
   - `go test ./... -count=1`

## Review and publication

- Measure Shannon signal before and after the cleanup pass; remove only duplication introduced by this change.
- Commit with Conventional Commit subject plus Lore body, push the exact issue branch, and open a draft PR to `main` with #52 labels and assignee.
- Do not merge, force-push, delete the branch/worktree, or close the issue.
