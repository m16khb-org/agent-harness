# State doctor reset-receipt ownership design

## Problem

`issueops update` writes the IssueOps v1 activation receipt to the
top-level state directory `issueops_reset_v1`. The state doctor currently
recognizes other harness-owned top-level stores but not this one, so a clean
installation immediately reports `unexpected_directory` and makes the
operational doctor unhealthy.

## Decision

Treat `issueops_reset_v1` as a harness-owned state directory. Keep the receipt
location and lifecycle unchanged; the mismatch is in the state doctor's
ownership inventory, not in activation persistence.

This is preferred over moving the receipt because relocation would require a
state migration and compatibility read path. Deleting the receipt after update
is rejected because it would remove the sealed activation evidence.

## Implementation

- Add `issueops_reset_v1` to the existing top-level state-directory ownership
  allowlist.
- Also add `issueops_v1` as an exact-match entry: the v1 namespace store
  created by `issueops_state.go` lives at the same top level and was flagged
  with the identical false positive on a live installation.
- Add a focused state-doctor test that creates these directories and proves
  they are accepted without warnings.
- Do not broaden recognition to arbitrary `issueops_*` directories or change
  the contents accepted inside the reset store.

## Verification

1. Run the focused test before the production change and confirm it fails with
   `unexpected_directory`.
2. Apply the one-entry ownership change and confirm the focused test passes.
3. Run the complete state/core tests, full Go tests, race tests, vet, build,
   native update, operational doctor, stability audit, and full self-verify.
4. Confirm the final checkout and `origin/main` are identical and clean.

## Risk and rollback

Risk is low and limited to state-doctor classification. The directory name is
an exact match, so foreign directories remain warnings. Rollback is a normal
revert of the implementation commit; no persisted data migration occurs.
