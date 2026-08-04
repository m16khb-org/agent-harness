# Issue #334 Turing report

## Outcome

Provenance-bound delegated children can now bootstrap their branch preparation
and execution preparation before the child has an execution record of its own.
The authority is borrowed only from the active, durable parent that delegated
and references the child, and only for those two bootstrap commands.

The direct execution adapter also accepts the exact sealed parent worktree as
the preparation cwd. This lets the parent generate and preview the child handoff
before releasing its lease; the child then confirms preparation as the sole
writer.

## TDD evidence

- RED: delegated `branch prepare` was rejected because the child had no execution
  from which to validate generated provenance.
- RED: delegated `execution prepare` was rejected for the same circular bootstrap
  dependency.
- RED: direct preparation rejected the sealed parent worktree cwd even though it
  is the only authorized coordinator before the child worktree exists.
- GREEN: the current referenced child borrows the exact active parent authority.
- GREEN: a foreign session, unrelated child, arbitrary cwd, or any non-bootstrap
  command remains rejected.

## Verification

- focused delegated bootstrap tests in lifecycle and IssueOps CLI
- full lifecycle and IssueOps CLI package tests
- focused race tests for delegated bootstrap
- outbound IssueOps preparation adapter tests
- repository-wide tests, race tests, vet, build, and native install
- `git diff --check`

## Live dogfood

The installed candidate created GitHub issue #333, prepared its linked branch,
and previewed its execution from the active #228 parent. After the parent lease
was released, the exact child execution preparation completed and generation 1
became the sole active writer in the canonical #333 worktree.

That child then committed the install activation fixture repair, pushed exact
HEAD `ae719a077fce6d8f1953c2302a5de98931b444bd`, verified draft PR #335, passed
both GitHub CI checks, and completed its IssueOps execution with a released
lease. This proves the bootstrap path through a real child lifecycle rather
than only unit tests.

## Safety boundary

There is no generic fallback for children without execution state. The child
must have sealed delegation to an active parent, the parent must durably list
that exact child, native holder identity and generated binary provenance must
match, and the branch, base branch, parent worktree, command cwd, and command
kind must match the durable topology. After bootstrap, normal child authority
is required for every mutation.
