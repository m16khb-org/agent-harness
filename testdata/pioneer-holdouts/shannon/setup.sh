#!/usr/bin/env bash
# Rebuilds the SHANNON-H1 git working-tree state in a throwaway dir.
# (A nested .git cannot be committed, so the state is scripted instead.)
# Expected result: `git status --short` -> "MM calc.py" + "?? evidence.md".
set -euo pipefail
DEST="${1:?usage: setup.sh <dest-dir>}"
mkdir -p "$DEST"
cd "$DEST"
git init -q
GC=(git -c user.email=fixture@local -c user.name=fixture)

printf 'def add(a, b):\n    return a + b\n' > calc.py
git add calc.py
"${GC[@]}" commit -qm "add calc.py"

# Staged edit (index ahead of HEAD):
printf 'def add(a, b):\n    return a + b\n\n\ndef sub(a, b):\n    return a - b\n' > calc.py
git add calc.py

# Further unstaged edit on top of the staged one (=> "MM"):
printf 'def add(a, b):\n    return a + b\n\n\ndef sub(a, b):\n    return a - b\n\n\ndef mul(a, b):\n    return a * b\n' > calc.py

# Untracked file:
printf 'scratch notes\n' > evidence.md

echo "built SHANNON-H1 state in $DEST"
git status --short
