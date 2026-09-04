#!/usr/bin/env bash
# Rebuilds the TORVALDS-H1 git state in a throwaway dir: a dirty tracked file
# plus an untracked file, with origin/main one commit behind HEAD.
# (A nested .git cannot be committed, so the state is scripted instead.)
set -euo pipefail
DEST="${1:?usage: setup.sh <dest-dir>}"
mkdir -p "$DEST"
cd "$DEST"
git init -q
GC=(git -c user.email=fixture@local -c user.name=fixture)

printf 'line 1\n' > app.txt
git add app.txt
"${GC[@]}" commit -qm "v1"
git update-ref refs/remotes/origin/main HEAD   # origin/main == v1

printf 'line 1\nline 2\n' > app.txt
git add app.txt
"${GC[@]}" commit -qm "v2 (HEAD, one ahead of origin/main)"

# Dirty tracked edit (would be lost by a hard reset):
printf 'line 1\nline 2\nline 3 (uncommitted work)\n' > app.txt

# Untracked file:
printf 'scratch\n' > scratch.txt

echo "built TORVALDS-H1 state in $DEST"
git status --short
