# Issue 46 worker Turing report

Scope: plan Tasks 1 through 5 at supervised attempt 3. Tasks 6 and 7 remain coordinator-owned.

## Outcome

- G1-C1 passed: genuinely immutable macOS system config can authorize publication without a sibling lock, while owner-controlled and drifting configs fail closed.
- G1-C2 passed: origin/include/URL rewrite inventories are bounded-complete through 1 MiB and never return partial overflow results.
- G1-C3 passed: existing writable lock collision and URL rewrite race defenses remain green.
- G2-C1 passed: valid branch_prepare.base_sha is the immutable implementation evidence base; legacy moving refs remain fallback-only.
- G3-C1 technical gates passed: focused, full, race, vet, golden, source build, Windows compile/link, and deterministic self-verify exited zero. The goal remains pending only for coordinator-owned removal of the exact outer state root.

## Platform boundary

Unix provides effective UID, Stat_t identity, access checks, and the narrow permission/read-only lock error allowlist. The non-Unix helper refuses immutable fallback, so lack of metadata support cannot weaken authority.

## TDD receipt

RED failures reproduced partial 4096-byte inventory, missing explicit overflow, moving-origin evidence loss, owner/path-chain misclassification, and persistent snapshot drift. GREEN tests are named in the five criterion artifacts under `.agent-harness/turing/evidence/issue46-*`.

## Cleanup and remaining ownership

All Go fixture/process cleanup completed. The exact isolated outer self-verify state root `/var/folders/mt/cyw_xzps58768x9tq23r5t200000gn/T/tmp.hFFtPPfGPk` remains for coordinator cleanup because the enabled worker hook rejected deletion outside the claimed worktree and also rejected direct worker escalation as coordinator-owned. The binding reviewer reports no remaining Critical or Important code/docs finding, but withholds unconditional Task 5 approval until that root is removed and absence is verified. Native install, live hooks E2E, publication, push, PR, merge, issue closure, and main reconciliation were intentionally not performed.
