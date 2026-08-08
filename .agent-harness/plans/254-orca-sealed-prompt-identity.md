# #254 Orca sealed prompt identity plan

## Assumptions and boundary

- The owner prompt, context packet, and issue-body digests form one generation-bound sealed identity.
- The durable Orca binding is the surviving source of truth after the terminal preparation intent is deleted.
- Existing v1 records with neither an artifact-identity version marker nor digests remain decodable, but resume must fail closed and direct the operator through the existing preview and reseed recovery flow. Every post-#254 producer stamps identity version 1, so a versioned all-empty binding is an invariant violation rather than a legacy recovery candidate.
- This child does not change Orca provisioning, owner selection, claim authorization, or the owner prompt text.

## Execution plan

1. RED: add contract tests for versioned-complete versus unversioned-empty sealed identity, post-upgrade all-empty rejection, preparation persistence, reseed persistence, resume across template changes, drift rejection, and legacy recovery routing.
2. GREEN: add identity version 1 plus the three SHA-256 fields to the v1 Orca binding, populate them at preparation dispatch and reseed commit, and verify artifact bytes only against that durable identity during resume.
3. Recovery: make status route a claimable legacy Orca binding to replacement preview; preview returns the generation-CAS reseed command and reseed produces a normal resume command.
4. Compatibility: preserve decoding only for unversioned all-empty legacy bindings, reject versioned-empty, unversioned-complete, partial, and future-version identities, and avoid implicit migration or trusting current worktree bytes.
5. Verification: run focused tests, contract and response goldens, full tests, race tests, vet, build, diff checks, independent implementation review, PR CI, exact-head dual-host smoke, and live #248 Orca recovery. Stop live recovery before mutation if #248 is no longer generation 1 claimable or if its artifact/resource inventory differs from the recorded snapshot.

## Success criteria

- A prompt sealed with an older template resumes after the binary template changes when all three stored digests still match the artifacts.
- Prompt, packet, or issue-body drift is rejected before any Orca mutation.
- New preparation and every Orca reseed persist identity version 1 and the complete identity in the binding; a versioned all-empty binding fails contract validation.
- Legacy #248 is recovered only by the explicit status preview to reseed to resume chain, after which a real Orca owner claims the new generation and mutates production code.

Rollback is a revert of this child merge. Existing v1 records remain byte-compatible because the added fields are optional on decode; no state is rewritten automatically.
