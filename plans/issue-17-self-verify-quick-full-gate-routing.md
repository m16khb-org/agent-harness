# Issue 17: self-verify quick/full gate and external LLM routing

Issue: #17
Branch: `feature/17-self-verify-quick-full-gate-routing`
Worktree: `$HOME/Workspace/agent-harness.worktrees/feature-17-self-verify-quick-full-gate-routing`

## Success Criteria

- `agent-harness self-verify` defaults to quick one-iteration mode.
- `agent-harness self-verify --full` runs the full gate with at least 10 iterations.
- `--iterations` without `--full` is rejected.
- The final self-verify LLM gate remains a single foreground blocking gate after deterministic evidence collection.
- External LLM judge prompts explicitly require read-only judgment and forbid file, git, workspace, issue, label, PR, MR, and state mutation.
- IssueOps remote scoring is classified as `background_join`; the main loop may continue local work but must join before remote artifact writes.
- IssueOps review subagent prompts are bounded by explicit scope, time budget, worktree identity checks, large/generated path exclusions, and a fallback direct verification path.
- IssueOps skill guidance is split so `SKILL.md` stays a compact phase router and detailed operational rules live in phase-specific reference files.

## Implementation Plan

1. Add resolver tests and CLI/MCP mode handling for quick/default versus full ten-plus-iteration self-verification.
   Verify with focused `go test ./cmd/harness`.
2. Update self-verify loop contract, usage, MCP schema, skills, README, and project docs.
   Verify with golden tests and exact-string search.
3. Add LLM gate classification fields and read-only prompt rules for self-verify and IssueOps remote scoring.
   Verify with focused prompt and JSON result tests.
4. Refresh golden contract files and run the repository test suite.
   Verify with `go test ./... -count=1`.
5. Add the subagent review timeout lesson to IssueOps guidance.
   Verify with exact-string search and repository tests.
6. Split the oversized IssueOps skill into a compact router plus phase references.
   Verify with skill validation, line counts, golden tests, and repository tests.

## Background Join Rule

Do not use lifecycle hooks to poll or wait for background LLM jobs. Hooks may display lightweight status only. The main IssueOps loop owns completion decisions: before creating or editing issues, labels, PRs, MRs, assignments, or comments, it must join any `background_join` LLM result and require a successful read-only judgment.

## Bounded Review Rule

Do not ask subagents to review broad diffs that include golden files, generated fixtures, vendored data, or response snapshots. Split those checks or rely on golden tests. Review prompts must specify included paths, excluded paths, time budget, expected output shape, and a fallback when the reviewer does not return within the wait budget.
