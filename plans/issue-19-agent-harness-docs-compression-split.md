# Issue 19: agent-harness project docs compression and selective split

Issue: provider issue `#19` in the current `agent-harness` repository
Branch: `chore/19-agent-harness-docs-compression-split`
Worktree: sibling IssueOps worktree for `chore-19-agent-harness-docs-compression-split`

## Domain Grill

- "압축" means removing duplication, moving historical detail out of the active reading path, and keeping current contracts concise. It does not mean deleting decision history.
- "분할" means extracting over-broad reference sections into stable subdocuments only where the current file has multiple distinct reader tasks. It does not mean fragmenting every large document.
- `.agent-harness` docs are agent-facing operating contracts, so path changes must preserve discoverability through required reading, docs index output, and existing routing hints.
- The first pass should focus on documents whose role is already overloaded: `ADR.md`, `OPERATIONS.md`, and `CAUTIONS.md`.
- `ARCHITECTURE.md`, `CONVENTIONS.md`, and `TESTING.md` are large enough to monitor, but their current roles are clearer. Split them only if verification shows routing or duplication still hurts after the first pass.

## Success Criteria

- `ADR.md` keeps current decisions and rationale in the active path while historical phase notes move to an archive or notes file.
- `OPERATIONS.md` becomes a compact quick-start and map, with detailed install, CLI, MCP, hook, verification, and worker usage moved to focused operation documents.
- `CAUTIONS.md` separates evergreen cautions from dated incident notes.
- IssueOps worktree/main checkout confusion is guarded by deterministic PreToolUse behavior, not prompt discipline alone.
- IssueOps decision-point replies can be guarded by strict-ready Stop hook behavior when numbered next actions are expected.
- Required reading, project docs catalog behavior, and human-readable references still point to the new paths.
- Duplicated LLM Wiki, API documentation gate, MCP, hook, and worker guidance is reduced to one source of truth plus short cross-references.
- No Go behavior, CLI/MCP schema, worker behavior, bootstrap target repo behavior, or architecture decision changes are introduced.

## Implementation Plan

1. Re-measure current docs and map duplicated sections.
   Verify with `find .agent-harness -maxdepth 2 -type f -name '*.md' -print | sort | xargs wc -l` and targeted `rg` for duplicated topics.

2. Compress `ADR.md`.
   Move dated phase/history notes out of the main file while preserving decision record access. Keep the current final decisions, accepted/rejected alternatives, and active roadmap references in `ADR.md`.
   Verify with heading review and `rg` checks for moved headings.

3. Split `OPERATIONS.md`.
   Keep a quick-start and reference map in `OPERATIONS.md`. Move detailed usage into `.agent-harness/operations/` subdocuments using narrow names such as `install.md`, `cli.md`, `mcp.md`, `hooks.md`, `verification.md`, and `worker.md` if the content boundaries remain clean.
   Verify with link/path checks and docs index output.

4. Split `CAUTIONS.md`.
   Keep evergreen hazards in `CAUTIONS.md`. Move dated incidents into a separate caution history file and leave a short pointer from the main file.
   Verify with heading review and `rg '^## 20[0-9]{2}-'`.

5. Enforce IssueOps worktree edit targeting.
   Add a focused PreToolUse guard that blocks mutating tool events outside `HARNESS_EXPECTED_WORKTREE` when IssueOps sets that environment variable. Keep the guard no-op outside IssueOps.
   Verify with focused hook/core tests and native install contract tests.

6. Enforce IssueOps numbered next actions.
   Add a strict-ready Stop hook guard that blocks final responses missing `1.`, `2.`, and `3.` choices only when `HARNESS_EXPECT_NUMBERED_NEXT_ACTIONS=1` and final assistant text is available.
   Verify with focused hook/core tests and native install contract tests.

7. Update routing references.
   Update required-reading references, docs catalog metadata, operations references, and any tests or golden files that intentionally enumerate project docs.
   Verify with `./bin/agent-harness docs --json`, `./bin/agent-harness bootstrap --dry-run`, and `./bin/agent-harness bootstrap --sync --dry-run`.

8. Final verification.
   Run `git diff --check`, `go test ./cmd/harness -run Golden -count=1`, and any focused tests required by changed golden/catalog output.

## Non-goals

- Do not redesign the project architecture or change accepted ADR decisions.
- Do not change unrelated Go core, CLI behavior, MCP schema, worker behavior, hooks, or install scripts beyond the IssueOps worktree guard needed to prevent source-checkout edits.
- Do not split every large document just because it is large.
- Do not delete historical decisions; preserve them outside the hot reading path.
- Do not update remote issue body with repo-local plan path.

## Verification Commands

```bash
find .agent-harness -maxdepth 2 -type f | sort
find .agent-harness -maxdepth 2 -type f -name '*.md' -print | sort | xargs wc -l
rg -n "LLM Wiki|API Documentation|OpenAPI|MCP|worker|hook|IssueOps|self-verify|자기 검증" .agent-harness
./bin/agent-harness docs --json
./bin/agent-harness bootstrap --dry-run
./bin/agent-harness bootstrap --sync --dry-run
go test ./cmd/harness -run Golden -count=1
go test ./internal/core -run 'PreToolUseWorktreeGuard|PreToolUseSearchRouting' -count=1
go test ./internal/core -run 'NumberedNextActions|PreToolUseWorktreeGuard|PreToolUseSearchRouting' -count=1
go test ./cmd/harness -run 'TestRunHookPreToolUse|TestRunHookStop' -count=1
go test ./internal/adapter -run 'Codex|Claude|Install' -count=1
git diff --check
```
