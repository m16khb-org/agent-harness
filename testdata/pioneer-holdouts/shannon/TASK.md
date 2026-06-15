# SHANNON-H1

Run `bash setup.sh /tmp/shannon-h1`, then analyze the git working-tree state in
that directory: signal vs noise of the pending changes. Account for staged,
unstaged, AND untracked files — do not rely on `git diff` alone, and do not
ignore untracked content. Be robust to a zero-change input (no divide-by-zero).
