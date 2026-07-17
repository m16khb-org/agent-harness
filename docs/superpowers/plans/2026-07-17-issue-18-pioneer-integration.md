# Issue #18 pioneer skill integration plan

## Goal

Integrate the three accepted Orca child reviews for GitHub issues #19, #20, and #21, verify all ten pioneer skills from the parent issue, preserve the #28 legacy-worktree E2E evidence, and publish the combined result to `main`.

## Inputs

- #19 / `io-339c2fca0e34`: accepted commit `a8e7dce90500e80a3cb3b68f889710df90ec7374`.
- #20 / `io-4f4603393a22`: accepted commit `73c44725b2693d9276df05af3c509bee5a6b21e9`.
- #21 / `io-ff473d80b45b`: accepted attempt 2 commit `defb73d8f7e6d147f0777b4c3060057f7c78dec4`.
- #28 / `io-959e7d74d2ac`: accepted attempt 5 no-change E2E at `bafcbbeb2c9ef65be2f8cd7fc770fd5e98dcd08f`.

## Steps and checks

1. Fast-forward the parent branch to `origin/main` and cherry-pick only accepted child commits.
   - Check: the parent diff contains the three child review reports and only their evidence-backed skill edits.
2. Preserve the #28 plan and all observed IssueOps/Orca blockers.
   - Check: the legacy E2E plan and `.agent-harness/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md` contain exact attempt/task/dispatch evidence without secrets.
3. Fix the failed-result JSON round-trip defect found during #21 recovery.
   - Check: a regression reproduces the pre-fix invalid envelope and passes after nil canonicalization.
4. Validate all ten pioneer skills and run the Turing verification wave.
   - Check: skill validators, focused tests, full tests, race, vet, goldens, build, and deterministic self-verify all pass.
5. Publish through a reviewed pull request and merge to `main`.
   - Check: `origin/main` contains the integration commits, GitHub has no open target issues or PRs, and the local `main` matches `origin/main`.

## Safety boundaries

- Do not rewrite the pioneer methods or add new skills.
- Do not merge unaccepted worker state or legacy branch merge commits.
- Do not disable hooks, dump environment secrets, or bypass the coordinator/worker authority split.
- Stop publication if any final verification command fails.
