# Issue 46 worker Turing report

Scope: plan Tasks 1 through 5 production implementation at supervised attempt 3, followed by attempt 4 metadata-only reconciliation. Tasks 6 and 7 remain coordinator-owned.

## Outcome

- G1-C1 passed: genuinely immutable macOS system config can authorize publication without a sibling lock, while owner-controlled and drifting configs fail closed.
- G1-C2 passed: origin/include/URL rewrite inventories are bounded-complete through 1 MiB and never return partial overflow results.
- G1-C3 passed: existing writable lock collision and URL rewrite race defenses remain green.
- G2-C1 passed: valid branch_prepare.base_sha is the immutable implementation evidence base; legacy moving refs remain fallback-only.
- G3-C1 passed: focused, full, race, vet, golden, source build, Windows compile/link, and deterministic self-verify exited zero; coordinator cleanup evidence closes the outer-state-root receipt.

## Platform boundary

Unix provides effective UID, Stat_t identity, access checks, and the narrow permission/read-only lock error allowlist. The non-Unix helper refuses immutable fallback, so lack of metadata support cannot weaken authority.

## TDD receipt

RED failures reproduced partial 4096-byte inventory, missing explicit overflow, moving-origin evidence loss, owner/path-chain misclassification, and persistent snapshot drift. GREEN tests are named in the five criterion artifacts under `.agent-harness/turing/evidence/issue46-*`.

## Cleanup and remaining ownership

All Go fixture/process cleanup completed. The enabled worker hook rejected both removal and read-only absence inspection of the exact isolated outer self-verify state root, so attempt 3 reported it without bypassing the guard. The coordinator then verified lsof had no users, removed the exact root, and ran a separate absence check with exit 0 before dispatching clean-HEAD attempt 4 for evidence-only reconciliation. Native install, live hooks E2E, publication, push, PR, merge, issue closure, and main reconciliation were intentionally not performed.
