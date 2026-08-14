---
name: issue-branch-worktree
description: Create an issue-based branch and isolated worktree, then register the branch on the issue (GitHub linked branch / GitLab related branch). Use when the user asks to start work from an issue, create a worktree for an issue, make an issue branch, link or register a branch to an issue, or says "이슈 브랜치", "이슈 워크트리", "브랜치를 이슈에 등록". Standalone slice of the full issueops cycle; when an active issueops cycle owns the work, this skill wraps `issueops branch prepare` / `link-worktree` instead of freelancing.
---

# Issue Branch Worktree

> Focused flow: **issue → pinned base SHA → branch → isolated worktree → provider-side branch registration → verify**. The full lifecycle (grilling, plan, execution lease, PR) belongs to the **`issueops`** skill; advanced git surgery belongs to **`torvalds`**.

## Routing gate (run first)

1. **Active issueops cycle?** Check `ISSUEOPS_ID` in the environment and `agent-harness issueops list --repo "$PWD" --json`. If a cycle owns this issue:
   - Branch/worktree prep is OWNED by issueops. Use `issueops branch prepare` and `issueops execution prepare` / `link-worktree`; this skill only supplies the git mechanics and verification below.
   - GitHub + Orca mode follows the #176 ordering documented in `skills/issueops/SKILL.md` (`branch prepare` base-SHA-only → stage plan → Orca prepare → GraphQL `createLinkedBranch` at the sealed SHA → `branch prepare --link-verified`). Do not reorder it from here.
2. **Orca ad-hoc?** If no issueops cycle and `orca status --json` reports `runtime.state == "ready"`, note that `orca worktree create` mints its own `<git-user>/<name>` branch. That conflicts with the issue-number-first branch rule below, so for issue-registered branches prefer the manual flow in this skill; use orca only to attach sessions to the finished worktree.
3. **Provider?** Determine GitHub vs GitLab from the issue URL or `git remote get-url origin`. Verify auth early: `gh auth status` / `glab auth status`.

## Safety rules

- Never create the branch from an unpinned moving ref. Resolve and record the exact base SHA first.
- Branch and worktree creation are local and safe. **Provider registration (remote branch creation, linked-branch mutation, issue notes) is an external write — state what will be written and get user confirmation before the first remote mutation, then proceed without re-asking for the rest of the registration.**
- Never reuse an existing local or remote branch name; collision means stop and report.
- Never run this flow from inside another worktree of the same repo without reporting it; the source checkout must remain clean throughout.
- No force operations, no default-branch mutation, no `gh issue develop` when the base SHA must be sealed (it links at the base branch's *current* HEAD, not your pinned SHA).

## Workflow

### 1. Identify the issue and derive names

```bash
# GitHub
gh issue view <number-or-url> --json number,title,url,id
# GitLab
glab issue view <iid> --output json   # or glab api "projects/:id/issues/<iid>"
```

- Branch name: `<issue-number>-<kebab-title-slug>` (lowercase, ASCII, ≤ ~50 chars). The number-first prefix is mandatory: GitLab auto-relates `<iid>-*` branches to the issue, and the issueops contract requires it on GitHub too.
- Worktree path: `../<repo>.worktrees/<branch-with-slashes-replaced-by-dashes>` (sibling of the checkout, never inside it).
- Collision check: `git branch --list <branch>`, `git ls-remote --heads origin <branch>`, `git worktree list`.

### 2. Pin the base SHA

```bash
git fetch origin <base-branch>
BASE_SHA=$(git rev-parse "origin/<base-branch>")
```

- Base branch is the repo default unless the user or the parent issue names another (child work targets the parent issue branch — see **`gitlab-usecase`**).
- Record `BASE_SHA` in your report; every later step uses it, never the branch name.

### 3. Create branch + worktree atomically

```bash
git worktree add "../<repo>.worktrees/<branch>" -b "<branch>" "$BASE_SHA"
```

- One command creates both; do not pre-create the branch separately.
- Immediately verify: `git -C "../<repo>.worktrees/<branch>" rev-parse HEAD` equals `$BASE_SHA`, and `git status --short` in the source checkout is empty.

### 4. Register the branch on the issue (remote write — confirm first)

**GitHub — GraphQL `createLinkedBranch` at the pinned SHA:**

```bash
ISSUE_ID=$(gh issue view <number> --json id -q .id)
gh api graphql -f query='
  mutation($issueId: ID!, $oid: GitObjectID!, $name: String!) {
    createLinkedBranch(input: {issueId: $issueId, oid: $oid, name: $name}) {
      linkedBranch { ref { name } }
    }
  }' -f issueId="$ISSUE_ID" -f oid="$BASE_SHA" -f name="<branch>"
```

- This both creates the remote branch at `$BASE_SHA` and links it to the issue in one step.
- Do NOT substitute `gh issue develop` — it links at the base branch's current HEAD, which can drift past your pinned SHA.

**GitLab — remote branch at the pinned SHA; the `<iid>-` prefix does the linking:**

```bash
glab api --method POST "projects/:id/repository/branches" \
  -f branch="<branch>" -f ref="$BASE_SHA"
git -C "../<repo>.worktrees/<branch>" branch --set-upstream-to "origin/<branch>"
```

- GitLab shows any `<iid>-*` branch under the issue's related branches automatically; no extra mutation needed. Optionally add an issue note naming the branch and worktree if the user wants an audit trail.

### 5. Record in issueops (when a cycle is active)

```bash
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" \
  --provider github|gitlab --issue-url <URL> \
  --branch "<branch>" --base-branch <base> --base-sha "$BASE_SHA" \
  --link-verified --json
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" \
  --worktree-path "../<repo>.worktrees/<branch>" --json
```

- Pass `--link-verified` only after step 6 confirms the provider link exists. GitHub Orca mode: omit it on the first `branch prepare` per the #176 ordering.

### 6. Verify (all must pass before reporting done)

- `git worktree list` shows the new worktree at the expected path.
- Worktree `HEAD` == `$BASE_SHA`; worktree `git status --short` is clean.
- Provider link visible:
  - GitHub: `gh api graphql` query on the issue's `linkedBranches` includes the branch.
  - GitLab: `glab api "projects/:id/issues/<iid>/related_merge_requests"` is not the check — open the issue's related branches (`glab api "projects/:id/repository/branches/<branch>"` proves the remote branch; the `<iid>-` prefix guarantees the relation).
- Source checkout branch and status unchanged.
- Report: issue URL, branch, base SHA, worktree path, and the registration evidence.

## Failure handling

- Branch/worktree name collision: stop, list the colliding refs/paths, ask whether to reuse or rename. Never delete to make room.
- `createLinkedBranch` rejected (e.g. name taken remotely, insufficient scope): report the exact API error; do not fall back to `gh issue develop` silently.
- Partial completion (worktree exists, registration failed): keep the worktree, report exactly which step remains, and make the retry idempotent — re-check existence before every create.
