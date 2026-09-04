# #256 Orca branch-link readiness recovery

## Goal

Make the `execution status -> resume` command actionable for a claimable GitHub
Orca generation whose linked branch is intentionally verified by the owner after
claim. Keep GitLab and every non-launch identity use fail-closed.

## Evidence

- Lifecycle `io-268bd6ac6e7a` generation 2 is claimable and status emits resume.
- Resume fails with `intent_identity_mismatch` while its durable
  `branch_prepare.link_verified` is false.
- GitHub `linkedBranches` and `refs/heads/248-orca-ready-issueops-dogfood`
  both identify issue #248 and base `511457c2e56e73a2c7451ba547a8fd9cfa58ab74`.
- The sealed owner prompt orders claim before its exact linked-branch reader and
  `branch prepare --link-verified` mutation, so an authoritative pre-launch link
  requirement makes that recovery step unreachable.

## Implementation

1. RED: add tests showing an unverified GitHub resume launch is accepted while
   an unverified GitLab resume remains rejected.
2. RED: prove that an active Orca holder with an unverified link can run the
   exact branch-link reader/recorder but cannot edit production files or enter
   implement, including through shell and MCP typed execution controls.
3. GREEN: replace the prepare-only identity helper with a launch identity helper
   used only by prepare/resume owner-launch intent sealing and persistence
   validation. Keep `authoritativeOrcaIssueIdentity` on every non-launch path.
4. GREEN: add a pre-link hook fence for active Orca holders. Admit observations
   and the exact `branch prepare --link-verified` owner mutation; reject other
   mutations, including typed execution control-plane requests, with
   `branch_link_verification_required` until the durable record is verified.
5. VERIFY: cover provider/URL/issue drift, unverified GitHub non-launch reject,
   unverified GitLab launch reject, marker identity validation, and the
   status-to-resume executable parity, then run full tests, race, vet, build,
   response goldens, and the architecture gate.
6. DOGFOOD: install the exact child binary, resume #248 generation 2, and verify
   the Orca owner claims before it records the already-proven GitHub link and
   mutates production code. Stop immediately if the generation changes or the
   exact branch-link recorder is not admitted for the claimed owner.
7. PUBLISH: independent review, exact-head Codex/Claude smoke, PR/CI/merge,
   parent acceptance, and IssueOps cleanup.

## Safety boundary

- Only GitHub launch intents may use prepared-but-unverified branch identity.
- Provider, issue URL, issue number, lifecycle, generation, and marker equality
  remain sealed and validated.
- The hook, not only the prompt, gates production and implement mutation until
  the exact holder records link verification.
- GitLab continues to require `link_verified=true` before every launch.
