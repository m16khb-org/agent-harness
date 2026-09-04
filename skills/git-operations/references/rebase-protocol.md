# Rebase Safety Protocol

## Pre-Rebase Checklist

Run these commands and verify their output before starting any interactive rebase:

```bash
# 1. Confirm clean working tree
git status --short
# Expected: empty output (no unstaged or staged changes)

# 2. Confirm current branch
git branch --show-current
# Note this — you'll need it for backup

# 3. Understand what you're about to rewrite
git log --oneline -n 20
# Identify which commits to keep, squash, reword, or drop

# 4. Find the merge-base
git merge-base HEAD <target-branch>
# This is the common ancestor — rebase rewrites commits AFTER this point

# 5. Create a safety backup
BACKUP="backup/$(git branch --show-current)-pre-rebase-$(date +%Y%m%d-%H%M%S)"
git branch "$BACKUP"
echo "Backup created: $BACKUP"
# To undo: git reset --hard "$BACKUP"
```

## During Rebase

### When a conflict occurs:
<!-- skill-shell: destructive recovery="the verified pre-rebase backup branch remains available for rollback" -->
```bash
# Identify conflicted files
git status --short | grep '^UU'

# For each conflicted file, see both sides:
git show :2:<file>  # Ours (the rebased version)
git show :3:<file>  # Theirs (the original commit's version)

# After resolving:
git add <file>
git rebase --continue
```

### Abort if needed:
<!-- skill-shell: destructive recovery="git rebase --abort returns to the pre-rebase state and preserves the backup branch" -->
```bash
git rebase --abort
# Returns to the pre-rebase state. 
# Your backup branch is still there as extra safety.
```

## Post-Rebase Verification

<!-- skill-shell: destructive recovery="retain and verify the backup ref before optional deletion" -->
```bash
# 1. Verify no unintended changes
git diff <backup-branch>..HEAD --stat

# 2. Verify history structure
git log --oneline --graph -n 10

# 3. If satisfied, retain the backup as a safety reference.
#    Backup refs persist until explicitly deleted.
#    Only after this confirmation ladder, bounded cleanup is optional:
git branch -D <backup-ref>
```

## Squash Rules

| Combine | Do NOT combine |
|---------|---------------|
| Behavior change + its direct tests | Behavior change + unrelated docs |
| Fixup into the commit it fixes | Two different feature changes |
| Typo fix into the commit that introduced it | Dependency update + any code change |
| Generated file + the source that generates it | Refactor + feature in one commit |
