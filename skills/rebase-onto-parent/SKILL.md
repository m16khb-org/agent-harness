---
name: rebase-onto-parent
description: Detect whether the current branch's parent branch has advanced, then rebase onto it safely using evidence-based parent resolution, a verified backup ref, range-diff proof that no commit was lost, and a confirmed --force-with-lease push. Use when the user asks to rebase onto the parent branch, catch up with the base branch, refresh a stacked branch, or says "부모 브랜치 rebase", "베이스 브랜치 최신화", "브랜치 최신으로 맞춰줘", "rebase onto parent", "catch up with base", "sync with base branch". Git Operations sub-skill; an issueops-owned branch routes to `issueops execution sync-base` instead of rebasing.
---

# Rebase Onto Parent

> **Git Operations sub-skill.** Git has no notion of a parent branch, so this skill
> resolves one from recorded evidence, proves the parent actually advanced, and
> rebases with a verified recovery path. Interactive rebase, bisect, and reflog
> archaeology belong to the parent **`git-operations`** skill. Ordinary commit and push
> belong to **`atomic-commit-push`**.

**User's request:** $ARGUMENTS

This skill owns exactly four effects:

1. resolve the parent branch of the current branch and record the decision;
2. determine whether that parent has advanced past the divergence point;
3. rebase the current branch onto the parent with a verified backup ref;
4. optionally push the rewritten branch with `--force-with-lease` after explicit
   confirmation.

It never resolves conflicts on its own, never uses bare `--force`, and never
rebases a branch owned by an active issueops cycle.

## Routing gate (run first)

1. **IssueOps-owned branch?** Check `ISSUEOPS_ID` in the environment and run
   `issueops list --repo "$PWD" --json`. If a cycle owns this
   branch, stop and route to `issueops execution sync-base`. That
   surface is merge-based and bound to the execution lease generation; rewriting
   history here invalidates the lease and the recorded PR head.
2. **Protected branch?** Refuse to rebase `main`, `master`, `develop`, or
   `release/*` unless the user names the branch and confirms in the same message.
3. **Orca is optional.** Orca worktree lineage is one evidence source among
   several. Never require an Orca runtime, and skip the source when
   `orca status --json` does not report a ready runtime.

## Safety rules

- The working tree must be clean and no rebase or merge may be in progress.
- Create and verify a backup ref before the rebase starts. No backup, no rebase.
- Never resolve a conflict automatically. Report both sides and hand the choice
  to the user.
- Push only with `--force-with-lease=<branch>:<pre-rebase remote SHA>`. Refuse
  bare `--force` and refuse a lease without an explicit expected value.
- Every claim in the final report must cite a command output, not an assumption.

## Step 1 — Resolve the parent branch

Collect evidence in this order and stop at the first hit. Report which source
answered so the user can judge the confidence.

| Rank | Source | Command | Confidence |
|---|---|---|---|
| 1 | This skill's own record | `git config --get branch.<branch>.parent` | exact |
| 2 | Open PR base / MR target | `gh pr view --json baseRefName` / `glab mr view` | exact |
| 3 | `gh` CLI merge base | `git config --get branch.<branch>.gh-merge-base` | exact |
| 4 | IssueOps pinned base | `git config --get branch.<branch>.base` | SHA, needs lookup |
| 5 | Orca worktree lineage | `orca worktree show --worktree path:<path> --json` | exact when ready |
| 6 | VS Code merge base | `git config --get branch.<branch>.vscode-merge-base` | usually the default branch |
| 7 | Divergence-point lookup | reflog SHA plus candidate comparison (below) | inferred |
| 8 | Repository default branch | `git symbolic-ref refs/remotes/origin/HEAD` | weak, announce it |

```bash
BRANCH=$(git branch --show-current)
git config --get "branch.$BRANCH.parent"
git config --get "branch.$BRANCH.gh-merge-base"
git config --get "branch.$BRANCH.base"
git config --get "branch.$BRANCH.vscode-merge-base"
```

Each `git config --get` exits 1 when the key is absent. Treat exit 1 as "no
evidence from this source" and continue down the cascade; do not treat it as a
failure of the skill.

### Rank 7: divergence-point lookup

`git switch -c` records only `branch: Created from HEAD`, so the reflog gives a
SHA and not a name. Recover the divergence SHA, then rank the candidate branches
by how recent their merge base is. The immediate parent is the candidate whose
merge base equals the divergence point.

```bash
BRANCH=$(git branch --show-current)
FORK=$(git reflog show "$BRANCH" --format='%H %gs' | awk '/branch: Created from/ {print $1}' | tail -1)
git for-each-ref --format='%(refname:short)' refs/remotes/origin | while IFS= read -r REF; do
  if [ "$REF" != "origin/$BRANCH" ]; then
    MERGE_BASE=$(git merge-base "$REF" HEAD)
    printf '%s merge-base=%s parent-ahead=%s\n' \
      "$REF" \
      "$(git rev-parse --short "$MERGE_BASE")" \
      "$(git rev-list --count "$MERGE_BASE..$REF")"
  fi
done
```

An older ancestor such as `origin/main` shows a merge base further back and
`parent-ahead=0`; the real parent shows a merge base equal to `$FORK`. When two
candidates tie, ask the user instead of guessing.

### Rank 8: repository default branch

```bash
git symbolic-ref --quiet refs/remotes/origin/HEAD
git remote set-head origin --auto
git symbolic-ref --quiet refs/remotes/origin/HEAD
```

The first call exits 1 in a repository where `origin/HEAD` was never written.
`git remote set-head origin --auto` creates it. Reaching rank 8 means the parent
was inferred rather than recorded, so say so before asking for confirmation.

### Confirm and record

Present the resolved parent with its evidence and the divergence figures, then
ask for confirmation. After the user confirms, record the decision so later runs
skip the cascade:

```bash
git config "branch.$BRANCH.parent" "<parent-ref>"
```

## Step 2 — Has the parent advanced?

```bash
git fetch --prune origin
MERGE_BASE=$(git merge-base "<parent>" HEAD)
git rev-list --left-right --count "<parent>...HEAD"
```

The left number counts commits the parent has and this branch does not; the
right number counts this branch's own commits. A left value of `0` means the
branch is already current: report that and stop without touching anything.

Prefer the remote-tracking ref `origin/<parent>` over the local branch. The
local copy of the parent may be stale or checked out in another worktree.

## Step 3 — Preflight

```bash
git status --porcelain
test -d "$(git rev-parse --git-path rebase-merge)" && echo "rebase in progress"
test -d "$(git rev-parse --git-path rebase-apply)" && echo "rebase in progress"
test -f "$(git rev-parse --git-path MERGE_HEAD)" && echo "merge in progress"
git rev-parse --verify --quiet "<parent>^{commit}"
```

Non-empty `git status --porcelain` output means the tree is dirty: stop and ask
whether to stash. Any in-progress rebase or merge means stop. A missing parent
commit means the resolution in step 1 was wrong; go back rather than continue.

Then create and verify the backup, and record the remote tip for the later lease:

```bash
BRANCH=$(git branch --show-current)
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
BACKUP="refs/backup/rebase-onto-parent/$BRANCH/$STAMP"
git update-ref "$BACKUP" HEAD
git show-ref --verify --quiet "$BACKUP" && echo "backup verified: $BACKUP"
git rev-parse "origin/$BRANCH"
```

Report the backup ref name and the pre-rebase remote SHA to the user before
proceeding. Both are needed for recovery and for the push lease.

## Step 4 — Choose the rebase form

The plain form is correct only while the recorded divergence point is still an
ancestor of the parent. When the parent was itself rebased or amended, replaying
the old range makes Git re-apply a commit the parent already carries in a
modified form, which produces a conflict in a file this branch never touched.

```bash
git merge-base --is-ancestor "<old-merge-base>" "<parent>"
```

Exit 0 selects the plain form. Exit 1 selects `--onto`. Recover
`<old-merge-base>` from `branch.<branch>.base`, or from
`git merge-base --fork-point "<parent>" "<branch>"`, or from the reflog
divergence SHA of step 1. `git merge-base "<parent>" "<branch>"` is the wrong
input here: after the parent moves it returns a much older commit, and using it
reintroduces the same duplicate replay.

`--fork-point` reads the parent's local reflog, so it is unavailable in a fresh
clone. When no source yields the old divergence point and the plain form is not
provably safe, say so and ask the user rather than guessing.

## Step 5 — Rebase

<!-- skill-shell: destructive recovery="restore from the verified refs/backup/rebase-onto-parent ref recorded in step 3, or run git rebase --abort while the rebase is in progress" -->
```bash
git rebase --no-fork-point "<parent>"
```

<!-- skill-shell: destructive recovery="restore from the verified refs/backup/rebase-onto-parent ref recorded in step 3, or run git rebase --abort while the rebase is in progress" -->
```bash
git rebase --onto "<parent>" "<old-merge-base>"
```

`--no-fork-point` is deliberate. Without it Git may silently drop commits it
believes the upstream already discarded, which is the opposite of the guarantee
this skill makes. The `--onto` form takes the parent as the new base and the old
divergence point as the start of the range to replay.

## Step 6 — Conflicts

Stop at the first conflict. Do not choose a side and do not continue.

```bash
git diff --name-only --diff-filter=U
git show :2:<path>
git show :3:<path>
```

**During a rebase the sides are inverted from the usual reading.** `:2:` (ours)
is the parent's content, because the parent is checked out as the base being
replayed onto. `:3:` (theirs) is this branch's commit. Label them by meaning in
the report, never as "ours" and "theirs".

For each conflicted file, report the path, both sides, and which commit of this
branch is being replayed (`git rev-parse --short REBASE_HEAD`). Then offer three
options and wait:

1. the user resolves, then `git add <path>` and `git rebase --continue`;
2. hand the file to **`git-operations`** for its conflict-resolution protocol;
3. abort and return to the pre-rebase state.

<!-- skill-shell: destructive recovery="abort returns the branch to the pre-rebase tip; confirm afterwards against the backup ref recorded in step 3" -->
```bash
git rebase --abort
```

If conflicts recur across more than three commits, abort, report the pattern,
and ask whether a merge from the parent would suit the branch better than a
rebase.

## Step 7 — Verify

Two invariants must both hold before the result is reported as successful.

```bash
git range-diff "<old-merge-base>..<backup-ref>" "<parent>..HEAD"
git rev-list --count "<old-merge-base>..<backup-ref>"
git rev-list --count "<parent>..HEAD"
git diff --stat "<backup-ref>" HEAD
```

1. **Every commit survived unchanged.** Each `range-diff` row must read `=`.
   A `!` row means the patch changed, which is expected only where a conflict was
   resolved; name that file and that resolution in the report. A `<` or `>` row
   means a commit was dropped or added, which is a failure.
2. **The only new content is the parent's.** `git diff --stat <backup-ref> HEAD`
   must show exactly the files the parent brought in.

Use `<old-merge-base>..<backup-ref>` on the left and `<parent>..HEAD` on the
right. Passing the parent's tip on both sides mixes the parent's own commits into
the comparison and produces rows that look like losses but are not.

## Step 8 — Push

Ask before pushing. Show the branch, the backup ref, the pre-rebase remote SHA,
the exact command, and any open PR or MR with its base branch.

<!-- skill-shell: destructive recovery="the pre-rebase remote SHA recorded in step 3 restores the remote ref; the lease rejects the push when the remote moved" -->
```bash
git push --force-with-lease="<branch>:<pre-rebase remote SHA>" origin "<branch>"
```

The explicit expected value is what makes the lease meaningful. A value that no
longer matches the remote is rejected as `! [rejected] ... (stale info)`, and an
expected value Git cannot resolve to a known object degrades to an unforced push
and is rejected as `! [rejected] ... (non-fast-forward)`. Both mean the same
thing operationally: do not retry, re-read the remote and find out who moved it.

Refuse `git push --force`. Refuse `--force-with-lease` without `=<branch>:<sha>`,
because the bare form trusts the local remote-tracking ref, and a background
fetch can have already updated it.

## Recovery

The backup ref is the recovery path for every failure after step 3.

```bash
git for-each-ref "refs/backup/rebase-onto-parent/<branch>"
git log --oneline -1 "<backup-ref>"
git diff --stat "<backup-ref>" HEAD
```

Show the user what would be discarded, then ask for confirmation naming the exact
command before running it.

<!-- skill-shell: destructive recovery="the backup ref is verified to exist and its diff against HEAD is shown to the user, who confirms the exact command before it runs" -->
```bash
git reset --hard "<backup-ref>"
```

Backup refs under `refs/backup/` are not pruned by ordinary garbage collection
while they exist. Delete one only after the user confirms the rebase result.

## Never

- Rebase a branch an active issueops cycle owns; route to
  `issueops execution sync-base`.
- Rebase without a verified backup ref.
- Resolve a conflict by picking a side without the user's decision.
- Push with bare `--force`, or with a lease that carries no expected SHA.
- Report success without both step 7 invariants shown.
- Delete a backup ref in the same run that created it.

## Verified facts

Each row was reproduced in a scratch repository before being written here.

| Claim | Evidence |
|---|---|
| `git switch -c` records `branch: Created from HEAD`, never a branch name | reflog of a branch created with `switch -c` |
| The divergence-point lookup separates the immediate parent from an older ancestor | parent showed `merge-base` equal to the fork point with `parent-ahead=2`; `origin/main` showed an earlier merge base with `parent-ahead=0` |
| Plain `git rebase <parent>` conflicts in an untouched file after the parent is amended | exit 1 with the parent's own file listed by `--diff-filter=U` |
| `git rebase --onto <parent> <old-merge-base>` handles the same case cleanly | exit 0, the branch commit preserved, the parent's content intact |
| `git merge-base --fork-point` and the reflog divergence SHA both recover the old merge base | both returned the same SHA that `git merge-base` alone did not |
| `--force-with-lease` with a wrong expected SHA is rejected | `! [rejected] ... (stale info)` in one run and `! [rejected] ... (non-fast-forward)` in another, depending on whether Git could resolve the expected value |
| `range-diff` over the old and new ranges proves commit preservation | every row read `=` after a successful rebase |
| `origin/HEAD` may be absent and is restored by `git remote set-head origin --auto` | `symbolic-ref` exit 1 before, exit 0 after |
