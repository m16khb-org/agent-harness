# Issue #21 owned pioneer skill review

## Scope

- Cycle: `io-ff473d80b45b`, attempt 2.
- Branch: `21-review-engineering-workflow-pioneers`.
- Attempt base: `21b1a748e0625cc566cd8c6b566604b307fef1cf`.
- Owned skills: `algorithm-optimization`, `database-design`, `git-operations`, and `issueops`.
- Criterion: `issue-21-owned-skills-reviewed` passes only when all four skills have recorded evidence, every required validator exits 0, the worker result is committed, and the worktree is clean.

## Review findings

### algorithm-optimization — no change

- Gate 0 refuses optimization without a measured CPU hot path or meaningful input bound.
- Steps 1 and 5 require baseline, scaling, noise control, and before/after evidence; the IssueOps integration records those results.
- No stale command or missing cross-reference was found, so changing this file would be prose churn rather than a contract correction.

### database-design — corrected

- The first-turn contract required `EXPLAIN ANALYZE` evidence but did not say that PostgreSQL executes the statement.
- PostgreSQL's current `EXPLAIN` reference says `ANALYZE` actually executes the statement and that non-`SELECT` side effects occur normally. The correction limits live/unknown use to read-only statements and requires explicit disposable-environment or rollback planning for data-changing statements.
- Evidence: <https://www.postgresql.org/docs/current/sql-explain.html>.

### git-operations — corrected

- The identity claimed every Git object uses SHA-1, but current Git supports repository object formats `sha1` and `sha256`.
- Reflog text promised at least 90 days of recovery. Git's documented defaults are 90 days for reachable entries and 30 days for entries unreachable from the current tip, with configurable expiration.
- Evidence: <https://git-scm.com/docs/git-init> and <https://git-scm.com/docs/git-config> (`gc.reflogExpire`, `gc.reflogExpireUnreachable`).

### issueops — corrected

- The in-worktree CLI registry in `cmd/issueops/issueopscli/issueops.go` confirms the documented phase, review, feedback, remote, benchmark, cleanup, heartbeat, and handoff command families.
- Every reference named by the skill exists, including the nine IssueOps phase references, both Torvalds protocol references, the Berners-Lee report template, `PROMPT.md`, and the sub-agent policy/tradeoff documents.
- The lifecycle authority in `internal/core/lifecycle/lifecycle_handoff_authority.go` permits a claimed worker to commit locally but owns direct `orca` controllers on the coordinator side. A direct worker Orca heartbeat was blocked during this review; the sealed `issueops heartbeat` succeeded. The skill now states this boundary and automatic finish projection explicitly.
- The phase-assist section enumerates 11 skills while its introduction said 9; the count now matches the table.

## Karpathy contract

- Input/output contract: inspect only the four owned skill bodies and actual repository/tool evidence; output minimal contract corrections plus this bounded report.
- Test suite: no-defect skill remains unchanged; unsafe database command gains an execution warning; configurable Git hash/retention replaces absolute claims; supervised worker authority matches the lifecycle guard; all skill validators pass.
- Adversarial cases: reject style-only rewrites, nonexistent tool names, direct worker Orca control, scope expansion, push/PR/merge/cleanup, and hidden-reasoning requests.
- One-variable iteration: each corrected clause changes one verified contract claim; no unrelated structure or examples changed.
- Privacy/tool truth: no secret or hidden reasoning is recorded; tool names come from source registry, lifecycle guard output, or official command documentation.

## Turing evidence block

- Success criteria: `issue-21-owned-skills-reviewed` = all four reviews evidenced, required validators exit 0, atomic commit exists, and final status is clean.
- Evidence artifact: this committed report, owned skill diff, CLI registry/guard source reads, reference existence checks, and final verification output.
- Cleanup receipt: no runtime, temporary directory, server, browser, container, or external mutation was spawned.
- Verification mode: proportionate lightweight mode because this is a reversible Markdown-only contract correction; auxiliary CLI validation and diff evidence are sufficient.
- Skipped checks: full Go tests, builds, live database probes, and destructive Git probes are outside the sealed verification packet.

## Verification receipt

The pre-receipt ordered run completed on 2026-07-17 with these direct-command results:

| Command | Exit | Output |
| --- | ---: | --- |
| `git diff --check` | 0 | no output |
| `python3 scripts/validate-skill.py skills/database-design` | 0 | `Skill is valid!` |
| `python3 scripts/validate-skill.py skills/algorithm-optimization` | 0 | `Skill is valid!` |
| `python3 scripts/validate-skill.py skills/issueops` | 0 | `Skill is valid!` |
| `python3 scripts/validate-skill.py skills/git-operations` | 0 | `Skill is valid!` |

Because this receipt edit follows that run, the worker must rerun the entire ordered sequence before committing. The post-edit run and the post-commit clean-status observation are submitted as the authoritative handoff verification receipts; no partial run is reused.
