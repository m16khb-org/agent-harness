# IssueOps v1 Worktree And Context Rules

## Branch And Canonical Worktree

Create and verify the provider-linked issue branch before execution. GitLab
branch names start with the issue/task number and a hyphen. GitHub uses the
`createLinkedBranch` GraphQL mutation with the sealed base SHA as `oid`, not
`gh issue develop` — that CLI takes only a branch name and lets GitHub resolve
its current HEAD, which diverges from the sealed base (#176). Run
`issueops branch prepare` and follow the commands it renders. Record the exact
base branch and base SHA; never infer a base from the source checkout's current
HEAD.

Execution v1 is the only owner of local workspace provisioning:

```bash
agent-harness issueops execution prepare --id "$ISSUEOPS_ID" --mode auto \
  --owner-host "$OWNER_HOST" --owner-model "$OWNER_MODEL" \
  $ACTOR_FLAGS --json
```

Preview first, then repeat the identical request with `--confirm`. The command
creates or reuses exactly one sibling path:

```text
../<repo>.worktrees/<branch-name-with-slashes-replaced>
```

Do not run `git worktree add`, link a different path, or provision dependencies
from a second workflow. The persisted canonical path, branch, HEAD, base SHA,
and worktree identity are the contract for both direct and Orca modes.

Load `execution.md` for lease, owner claim, replacement, reconciliation,
remote publication, and completion commands.

## Edit Target Guard

Before every edit batch, verify the exact target rather than trusting the shell
prompt:

```bash
pwd
git branch --show-current
git rev-parse HEAD
test "$PWD" = "$EXPECTED_WORKTREE"
git -C "$SOURCE_CHECKOUT" status --short
git status --short
```

- Root patch/edit/generation tools at `$EXPECTED_WORKTREE`.
- Keep TDD, formatting, verification, commits, and PR/MR preparation inside the
  canonical worktree.
- Do not edit the selected cycle from the source checkout.
- If a tool writes to the wrong checkout, stop, move only your own changes to
  the canonical worktree, and verify both statuses before continuing.
- Worker prompts must include the exact lifecycle ID, generation, branch,
  worktree, allowed paths, acceptance criteria, and stop rule.

Installed Codex and Claude hooks use `HARNESS_SOURCE_CHECKOUT` and
`HARNESS_EXPECTED_WORKTREE` as deterministic guard inputs:

```bash
export HARNESS_SOURCE_CHECKOUT="/path/to/source-checkout"
export HARNESS_EXPECTED_WORKTREE="/path/to/repo.worktrees/69-example"
```

Hooks are guards, not workflow owners. They may reject a mutation outside the
persisted path but never create the branch/worktree, grant a lease, launch an
owner, run tests, or publish a remote artifact.

## Dependencies And Local Configuration

Set up dependencies inside the canonical worktree with the repository's own
documented install command. Reuse a large generated dependency directory only
after verifying the package manager, lockfile, runtime, platform, and native
module state match. Never commit generated dependency directories or symlinks.

Local-only configuration such as `.env`, `.env.local`, `.mcp.json`, or
`dbhub.toml` may be linked only when the task needs it, the source file is
ignored, and no secret will enter prompts, logs, tests, issue bodies, or PR/MR
bodies. Stop before linking a tracked, unignored, or unexplained credential
file.

## Context Routing

Use `rg`, CodeGraph when the repository is indexed, and native file tools
rooted at the canonical worktree. Any separately installed code-intelligence
tool must use that same root. After edits, combine exact-string search with the
relevant tests so stale references and wrong-root changes are observable.

## Parallel Independence

The exact lifecycle ID and canonical worktree fence only the selected cycle.
There is one active execution per record; unrelated cycles do not share its
generation or native holder. The source main worktree remains available before,
during, and after direct or Orca execution for unrelated cycles and read-only
inspection of the selected cycle.
