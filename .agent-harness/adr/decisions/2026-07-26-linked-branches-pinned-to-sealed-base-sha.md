# 2026-07-26 — Linked branches are pinned to the sealed base SHA

← [ADR index](../../ADR.md)

Decision: `branch prepare` guides GitHub linked-branch creation through the `createLinkedBranch` GraphQL mutation with the sealed base SHA as `oid`, instead of `gh issue develop --base <branch>`. GitLab's `ref` takes the same SHA (#180).

Rationale:

- `gh issue develop --base` accepts only a branch name; GitHub resolves that branch's HEAD at call time and fills `oid` itself. Orca creates its local branch from the base SHA sealed at `execution prepare`, so any base advance between the two makes the published branch diverge from the worktree, and push fails as `non-fast-forward`.
- Every resolution path is closed by design: the seal guard rejects `merge`, the safety hook rejects force push, `sync-base` requires completion, and a new worktree is blocked. Measured on #147; that cycle had to be published by cherry-pick and closed with `cleanup abandon`, leaving durable state and the published commit divergent.
- `oid` is a required field of `CreateLinkedBranchInput`, so passing the sealed SHA removes the divergence by construction rather than narrowing the window. Verified live by creating a linked branch at an arbitrary non-HEAD SHA.

Rejected alternatives: re-seal the base at Orca prepare time (the same divergence recurs whenever the base advances again before linking — the window shrinks but the failure mode stays), and allow `sync-base` before completion (that reverses the `completion_present` gate its own issue established).

This supersedes the `gh issue develop` step in #163's Orca ordering. The ordering itself — Orca prepare before remote linking — is unchanged.
