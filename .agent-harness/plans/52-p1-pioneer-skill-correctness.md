# GitHub #52 — P1 Pioneer Skill Correctness

## Objective

Implement only the ten P1 correctness repairs from the parent remediation plan for Berners-Lee, Brooks, Karpathy, Turing, Torvalds, von Neumann, Hopper, Dijkstra, and the CLI usage contract. Preserve P0/P2/P3/P4 work for their owning issues.

## Safety and ownership boundaries

- Work only in branch `52-p1-pioneer-skill-correctness` and this Orca-owned worktree.
- Do not rewrite history, force-push, open/merge a PR, or accept the handoff.
- Torvalds ownership is limited to `skills/torvalds/references/rebase-protocol.md`; #51 owns `SKILL.md`, `bisect-protocol.md`, and clean safety.
- Keep host-neutral language unless a verified host-specific tool is required. Do not invent APIs or candidate identifiers.
- Touch only files needed for P1-1 through P1-10 and directly affected contract/golden tests.

## TDD execution plan

1. Add or tighten focused contract tests that reproduce the ten stale/incorrect claims before editing prose or fences.
2. P1-1 Berners-Lee: replace numeric sub-agent pattern references with the canonical slugs `high-volume-exploration`, `devils-advocate-review`, `parallel-independent-research`, and `cross-verification-consensus`.
3. P1-2 Brooks and CLI usage: add an IssueOps Integration section with the exact `issueops devils-advocate review` CLI command and MCP equivalent; update the shared CLI usage text/golden without changing command behavior.
4. P1-3 Karpathy: remove the false claim that Shannon measures prompt quality; state that Shannon applies only to generated code artifacts, and narrow the IssueOps claim to its verified contract.
5. P1-4/P1-5/P1-7 Turing: replace duplicated supervised-handoff prose with a short link to `skills/issueops/references/orca-handoff.md`; make the evidence contract referenced and proportionate-mode consistent or remove it if truly redundant; replace nonexistent browser tooling and mark `xdotool` Linux-only while naming the verified macOS path.
6. P1-6 Torvalds: correct rebase backup retention—backup refs persist until explicitly deleted—and document bounded cleanup with `git branch -D "$BACKUP"` only after the existing confirmation ladder.
7. P1-8 von Neumann: replace the nonexistent `task(subagent_type="explore")` pseudo-API with the skill's existing host-neutral delegation wording.
8. P1-9 Hopper: correct the strategy count to four, use a registered self-augment candidate identifier, and add Strategy D to the routing golden/snapshot contract.
9. P1-10 Dijkstra: reconstruct the broken fenced block around the scaling-test interpretation so every example and explanatory line renders in the intended block.
10. Run focused tests first, then validate every changed skill and the shared contract/usage suites. Perform an AI-slop cleanup pass limited to this diff and report exact evidence.

## Acceptance criteria

- Each P1 item has current-source evidence, one minimal repair, and a focused regression check.
- All changed skills pass `scripts/validate-skill.py`.
- `go test ./internal/core/skillcontract -count=1` passes.
- CLI usage/contract goldens pass after the Brooks command documentation update.
- No P0, P2, P3, P4, history-rewrite, force-push, PR creation, or handoff acceptance work is included.

## Verification commands

```bash
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
go test ./... -count=1
```

## Handoff report

Return the final head, changed-file list, P1 item-to-evidence mapping, Turing report, exact verification results, known unrelated failures, and cleanup receipts. Stop at any model/usage prompt or scope-expanding decision and escalate to the source coordinator.
