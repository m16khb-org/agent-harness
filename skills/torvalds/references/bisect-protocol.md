# Bisect Protocol

## When to Use Bisect
- A regression was introduced (something that worked now fails)
- You don't know which commit caused it
- You have a reliable test command that exits 0 for good, non-0 for bad
- The history between good and bad is linear (no merge commits in the range — or you specify `--first-parent`)

## Step-by-Step

### 1. Identify Good and Bad
```bash
# Bad: usually HEAD
git log --oneline -1 HEAD

# Good: a tag, a known-working SHA, or found via:
git log --oneline -30  # scan for the last known-good state
```

### 2. Define the Test Command
The test command MUST be an executable wrapper script:
- A checked-in or temporary executable with a fixed argv
- Read-only (no file changes, no network side effects)
- Exit 0 = good, exit non-0 = bad

```bash
# Example wrapper prepared and reviewed before starting bisect.
# Its contents may run:
#   exec go test ./pkg/auth -run TestLoginFlow -count=1
BISECT_TEST="./scripts/bisect-login-flow.sh"
test -x "$BISECT_TEST"
```

### 3. Run Bisect
<!-- skill-shell: destructive recovery="record the original HEAD and complete the reset step below after diagnosis" -->
```bash
git bisect start
git bisect bad HEAD
git bisect good <known-good-sha-or-tag>
git bisect run "$BISECT_TEST"
```

### 4. Record and Reset
<!-- skill-shell: destructive recovery="git bisect reset returns to the original recorded HEAD" -->
```bash
# Record the bisect session
git bisect log > .agent-harness/evidence/bisect-<slug>.log

# The breaking commit is now at HEAD (bisect checked it out)
BREAKING_SHA=$(git rev-parse HEAD)
BREAKING_SUBJECT=$(git log --oneline -1 HEAD)

# Return to original state
git bisect reset

# Report
echo "Breaking commit: $BREAKING_SHA - $BREAKING_SUBJECT"
```

## When NOT to Use Bisect

| Situation | Alternative |
|-----------|-------------|
| Test command is flaky (non-deterministic) | Manual binary search: `git checkout <sha>`, run test, repeat |
| The breakage is intermittent (race condition) | Bisect with `--no-checkout`, run test N times per commit |
| History has many merge commits | Use `git bisect start --first-parent` to stay on the main line |
| The test command modifies files | Write a wrapper script that copies to a temp dir first |
| You don't know a good commit | Use `git log -S "<broken-feature>"` to find when the feature last changed |
