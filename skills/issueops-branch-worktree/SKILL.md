---
name: issueops-branch-worktree
description: Create an issue-based branch and isolated worktree, then register the branch on the issue (GitHub linked branch / GitLab related branch). Use when the user asks to start work from an issue, create a worktree for an issue, make an issue branch, link or register a branch to an issue, or says "이슈 브랜치", "이슈 워크트리", "브랜치를 이슈에 등록". Standalone slice of the full issueops cycle; when an active issueops cycle owns the work, this skill wraps `issueops branch prepare` / `link-worktree` instead of freelancing.
---

# IssueOps Branch Worktree

> Focused flow: **issue → pinned base SHA → branch → sibling isolated worktree → provider-side branch registration → optional Orca registration → verify**. The full lifecycle (grilling, plan, execution lease, PR) belongs to the **`issueops`** skill; advanced git surgery belongs to **`torvalds`**.

## Routing gate (run first)

1. **Active issueops cycle?** Check `ISSUEOPS_ID` in the environment and `agent-harness issueops list --repo "$PWD" --json`. If a cycle owns this issue:
   - Branch/worktree prep is OWNED by issueops. Use `issueops branch prepare` and `issueops execution prepare` / `link-worktree`; this skill only supplies the git mechanics and verification below.
   - GitHub + Orca mode follows the #176 ordering documented in `skills/issueops/SKILL.md` (`branch prepare` base-SHA-only → stage plan → Orca prepare → GraphQL `createLinkedBranch` at the sealed SHA → `branch prepare --link-verified`). Do not reorder it from here.

   **No cycle yet, but the work is meant to run as an issueops cycle?** Run **`issueops`** first and stop here. `issueops execution prepare` provisions the canonical worktree itself, and a worktree this skill created standalone is adopted only when its path, branch, and HEAD all match the recorded base SHA exactly — one commit in it and `execution prepare` fails with `existing canonical worktree identity does not match branch and base_head`. Use this skill standalone only for work that will stay outside issueops.
2. **Orca available?** Resolve the executable exactly as the installed `orca-cli` skill requires, then load its version-matched guide with `<orca> skills get orca-cli`. If `<orca> status --json` reports a ready runtime, register the finished Git worktree after provider linking. Never use `orca worktree create` for this flow: it creates a second checkout and mints its own branch, conflicting with the pinned SHA, issue-number-first branch, and sibling-path contracts below.
3. **Provider?** Determine GitHub vs GitLab from the issue URL or `git remote get-url origin`. Verify auth early: `gh auth status` / `glab auth status`.

## Safety rules

- Never create the branch from an unpinned moving ref. Resolve and record the exact base SHA first.
- Branch and worktree creation are local and safe. **Provider registration (remote branch creation, linked-branch mutation, issue notes) is an external write — state what will be written and get user confirmation before the first remote mutation, then proceed without re-asking for the rest of the registration.**
- Never reuse an existing local or remote branch name; collision means stop and report.
- Detect whether the command starts in the primary checkout or a linked worktree with `git rev-parse --path-format=absolute --git-common-dir`. Report linked-worktree execution and derive the sibling worktree root from the primary checkout that owns the common Git directory, never from the current linked worktree's basename.
- The primary source checkout must be clean before creation and remain unchanged throughout.
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
- Resolve the canonical primary checkout and worktree path before checking collisions:

```bash
COMMON_GIT_DIR=$(git rev-parse --path-format=absolute --git-common-dir)
SOURCE_ROOT=$(dirname "$COMMON_GIT_DIR")
REPO_NAME=$(basename "$SOURCE_ROOT")
WORKTREE_ROOT="$(dirname "$SOURCE_ROOT")/${REPO_NAME}.worktrees"
WORKTREE_PATH="$WORKTREE_ROOT/<branch-with-slashes-replaced-by-dashes>"
```

- `SOURCE_ROOT` must be the checkout whose `.git` directory equals `COMMON_GIT_DIR`. Stop if the repository is bare or this invariant does not hold.
- Worktree path: `$WORKTREE_PATH`, always one directory above the canonical primary checkout under `<repo>.worktrees`, never inside the repository and never relative to a linked worktree.
- Collision check: `git branch --list <branch>`, `git ls-remote --heads origin <branch>`, `git worktree list`.

### 2. Pin the base SHA

```bash
git -C "$SOURCE_ROOT" fetch origin <base-branch>
BASE_SHA=$(git -C "$SOURCE_ROOT" rev-parse "origin/<base-branch>")
```

- Base branch is the repo default unless the user or the parent issue names another (child work targets the parent issue branch — see **`gitlab-usecase`**).
- Record `BASE_SHA` in your report; every later step uses it, never the branch name.

### 3. Create branch + worktree atomically

```bash
git -C "$SOURCE_ROOT" worktree add "$WORKTREE_PATH" -b "<branch>" "$BASE_SHA"
```

- One command creates both; do not pre-create the branch separately.
- Immediately verify: `git -C "$WORKTREE_PATH" rev-parse HEAD` equals `$BASE_SHA`, and `git -C "$SOURCE_ROOT" status --short` is empty.

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
git -C "$WORKTREE_PATH" branch --set-upstream-to "origin/<branch>"
```

- GitLab shows any `<iid>-*` branch under the issue's related branches automatically; no extra mutation needed. Optionally add an issue note naming the branch and worktree if the user wants an audit trail.

### 5. Register the existing worktree with Orca when available

Do this only after the Git worktree exists and the provider branch is verified. The public Orca CLI can discover and update an external worktree, but current releases cannot force-import an undiscoverable path. Never compensate by creating a second Orca worktree.

1. Resolve the executable and load the installed guide as required by the `orca-cli` skill.
2. If no executable exists, record `orca_registration=not_installed` and continue.
3. If the executable exists but `status --json` cannot reach a ready runtime, record the exact error as `orca_registration=runtime_unavailable` and continue; do not switch to a different Orca binary.
4. Ensure the canonical source repository is present in `repo list --json`; if absent, register only that repository with `repo add --path "$SOURCE_ROOT" --json`.
5. Probe the existing worktree without creating anything:

```bash
<orca> worktree show --worktree "path:$WORKTREE_PATH" --json
```

6. When the probe succeeds, register visible metadata and read it back. GitHub supports a native issue field:

```bash
<orca> worktree set --worktree "path:$WORKTREE_PATH" \
  --display-name "<branch>" --comment "Issue #<number>: <issue-url>" \
  --workspace-status todo --issue <number> --json
<orca> worktree show --worktree "path:$WORKTREE_PATH" --json
```

For GitLab, omit `--issue` because the public Orca CLI has no GitLab IID write flag; preserve the issue URL in `--comment` and rely on the verified `<iid>-` branch/provider relation:

```bash
<orca> worktree set --worktree "path:$WORKTREE_PATH" \
  --display-name "<branch>" --comment "Issue #<iid>: <issue-url>" \
  --workspace-status todo --json
<orca> worktree show --worktree "path:$WORKTREE_PATH" --json
```

7. When the probe reports that the external worktree is hidden or unknown, record `orca_registration=external_worktree_not_discoverable` and the exact error. Current Orca requires the user to select **Non-Orca worktrees → Show** for that path; do not edit Orca state files or call `worktree create`.

### 6. Record in issueops (when a cycle is active)

```bash
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" \
  --provider github|gitlab --issue-url <URL> \
  --branch "<branch>" --base-branch <base> --base-sha "$BASE_SHA" \
  --link-verified --json
agent-harness issueops link-worktree --id "$ISSUEOPS_ID" \
  --worktree-path "$WORKTREE_PATH" --json
```

- Before running these commands, execute the provider-link check listed in step 7. Pass `--link-verified` only after that check succeeds. GitHub Orca mode: omit it on the first `branch prepare` per the #176 ordering.

### 7. Verify

- `git worktree list` shows the new worktree at the expected path.
- Worktree `HEAD` == `$BASE_SHA`; worktree `git status --short` is clean.
- Provider link visible:
  - GitHub: `gh api graphql` query on the issue's `linkedBranches` includes the branch.
  - GitLab: `glab api "projects/:id/issues/<iid>/related_merge_requests"` is not the check — open the issue's related branches (`glab api "projects/:id/repository/branches/<branch>"` proves the remote branch; the `<iid>-` prefix guarantees the relation).
- Primary source checkout branch and status are unchanged, even when the flow was invoked from a linked worktree.
- If Orca registration succeeded, `worktree show --worktree "path:$WORKTREE_PATH" --json` returns the exact path and branch. Otherwise report one of the explicit non-success statuses from step 5 without weakening the Git/provider result.
- Report: issue URL, branch, base SHA, worktree path, provider registration evidence, and Orca registration status/evidence.

## What comes next

Branch and worktree are the start of a cycle, not a cycle. When an issueops cycle
owns this work, hand back to **`issueops`**: it drives problem, grill, plan, and
compatibility-review, then provisions the execution lease and enters implement,
where **`issueops-implement`** takes over. Do not invoke `issueops-implement`
directly from here — with no record it has nothing to hold.

## Failure handling

- Branch/worktree name collision: stop, list the colliding refs/paths, ask whether to reuse or rename. Never delete to make room.
- `createLinkedBranch` rejected (e.g. name taken remotely, insufficient scope): report the exact API error; do not fall back to `gh issue develop` silently.
- Partial completion (worktree exists, registration failed): keep the worktree, report exactly which step remains, and make the retry idempotent — re-check existence before every create.
- Orca discovery failure never permits a second checkout. Keep the Git worktree, preserve provider registration, and report the exact Orca recovery action.
