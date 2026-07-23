# Issue #72 Cycle-Target Fence and Orca Readiness Plan

**Goal:** Keep IssueOps authority scoped to the exact canonical worker target and lifecycle control plane, while making Orca handoff readiness deterministic before external mutation.

**Architecture:** The lifecycle guard selects a cycle only from explicit canonical targets or a command whose effective cwd is the canonical worker. Source checkout activity remains under the ordinary command, branch, remote, and destructive-action policies. Orca preparation separates the exact base commit used to create a worktree from the branch used as its upstream, refreshes incomplete terminal receipts, and rejects unsupported GitLab metadata before creating Orca resources.

## Task 1: Cycle-target fence

- Add RED matrix cases for source reads/writes/builds/tests, source-cwd absolute canonical patches, mixed targets, wrong holders, released/claimable leases, `git -C`, redirects, symlink escapes, and opaque wrappers.
- Change execution-record selection to use canonical worker targets only.
- Preserve generation, actor, symlink-containment, topology, remote, and destructive-action gates.

## Task 2: Orca handoff identity

- Add RED adapter cases proving worktree creation uses the exact base SHA while upstream configuration uses the remote issue branch.
- Refresh terminal inventory when the immediate create receipt lacks the stable marker.
- Preserve GitHub `--issue` metadata behavior.

## Task 3: Provider readiness

- Add RED cases for GitLab `/issues` and `/work_items` identity equivalence.
- Make unsupported GitLab Orca issue metadata fail before external mutation; `auto` falls back to direct and explicit `orca` returns an actionable error.
- Recheck the same readiness on confirmed preparation.

## Task 4: Verification and publication

- Run focused packages, full tests, race, vet, contract goldens, build, native dry-run, skill validation, and diff checks.
- Install the verified binary and run live hook/Orca scenarios.
- Create one atomic commit, push the issue branch, and open/read back a Draft PR.
