# Issue 46 worker verification report

Scope: plan Tasks 1 through 5 production implementation at supervised attempt 3, attempt 4 metadata-only reconciliation, then the Claude recovery cycle io-d492e1a529e3 fixing the three binding Important findings B-69/B-70/B-71. Tasks 6 and 7 remain coordinator-owned.

## Outcome

- G1-C1 pass: B-71 fixed — immutable fallback now requires every path component to be root-owned; ordinary non-current UID chains fail closed (RED admissible=true → GREEN false) and the real root-owned macOS system config success path is retained.
- G1-C2 pass: B-69 fixed — include enumeration uses `git config --type=path` so Git itself performs canonical `~/`, `~user/`, `%(prefix)/` interpolation; the inventory equals the real-Git oracle and unresolved interpolation residue fails closed. Bounded >4096/1 MiB overflow contracts are unchanged.
- G1-C3 pass: B-70 fixed — the initially absent default XDG config is sealed by transiently creating its parent chain and holding the sibling lock; the transient create/rewrite/remove RED scenario now pushes only to the intended destination with no marker and no residue. All pre-existing lock collision and rewrite-race regressions stay GREEN.
- G2-C1 pass: valid branch_prepare.base_sha remains the immutable implementation evidence base; the recovery diff does not touch evidence code and the named tests re-ran GREEN.
- G3-C1: recovery gate wave re-run from the final worker HEAD; see the recovery section below and `.issueops/verified-execution/evidence/issue46-G3-C1-full-gates.txt`.

## Platform boundary

Unix provides effective UID, Stat_t identity, access checks, and the narrow permission/read-only lock error allowlist. The non-Unix helper refuses immutable fallback, so lack of metadata support cannot weaken authority.

## TDD receipt

RED failures reproduced partial 4096-byte inventory, missing explicit overflow, moving-origin evidence loss, owner/path-chain misclassification, and persistent snapshot drift. GREEN tests are named in the five criterion artifacts under `.issueops/verified-execution/evidence/issue46-*`.

## Cleanup and remaining ownership

All Go fixture/process cleanup completed. The enabled worker hook rejected both removal and read-only absence inspection of the exact isolated outer self-verify state root, so attempt 3 reported it without bypassing the guard. The coordinator then verified lsof had no users, removed the exact root, and ran a separate absence check with exit 0 before dispatching clean-HEAD attempt 4 for evidence-only reconciliation. Native install, live hooks E2E, publication, push, PR, merge, issue closure, and main reconciliation were intentionally not performed.

## Binding review outcome

REQUEST CHANGES. B-69 found unsealed `~user/` and `%(prefix)/` include authorities, B-70 found transient creation/removal of an initially absent XDG config authority, and B-71 found snapshot trust broader than the documented root-owned system boundary. This metadata-only lease made no production edit; a fresh TDD attempt must add the named RED cases, implement the narrow fixes, rerun all affected/full gates, and obtain unconditional approval.

## Claude recovery attempt io-d492e1a529e3 (2026-07-17)

Strict RED→GREEN TDD from exact base b348a16 as sole writer, hooks enabled throughout.

- B-71: RED `TestPublicationImmutableConfigClassificationFailsClosed` ordinary non-current UID file/parent cases; GREEN `publicationImmutableConfigAdmissible` root-owned-chain requirement.
- B-69: RED `TestPublicationIncludeInventoryResolvesTildeUserAndRuntimePrefixLikeGit` (relative-join mismatch vs real-Git `--type=path` oracle); GREEN Git-canonical interpolation plus fail-closed residue check; `TestPublicationIncludeInventoryFailsClosedOnUnresolvableTildeUser` pins git's own exit-128 fail-closed propagation.
- B-70: RED `TestPublicationAbsentXDGConfigAuthorityCannotBeTransientlyCreatedDuringPush` (marker written, push redirected to evil); GREEN transient parent-chain sealing with sibling lock, reverse-order removal, and post-operation absent verification.
- B-77: the attempt base carried a stale response-contract golden (docs_count 89 vs 90 after `.issueops/verified-execution/issue46-report.md` was committed); the golden was regenerated and the diff confirmed as the single-doc drift.
- Orchestration blockers B-72 through B-76 (claim host transcription, Orca-channel/fence mismatch, exact-form discovery, outside-worktree scratchpad/grep fences, git-config read classification) are recorded with working escapes in `.issueops/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md`.
- Cleanup: all test fixtures were `testing.T.TempDir`; the in-worktree scratch config was removed before commit; no install, push, PR, merge, source-checkout mutation, or hook bypass occurred.

## Claude recovery attempt io-d492e1a529e3 attempt 3 — B79 (2026-07-18)

Binding independent-review finding within the sealed B70 scope: the sibling
lock binds cooperating writers at one path identity only, so a same-UID racer
can rename the initially-created XDG_CONFIG_HOME tree (lock included) aside,
substitute a replacement tree whose fresh evil-rewrite config the push reads,
then remove it and restore the original so every post-operation verification
matches.

- RED `TestPublicationAbsentXDGConfigAuthorityCannotBeReplacedDuringPush` at attempt base 8886856: PushExact returned nil, the replacement rewrite marker existed, and the good destination never received the ref — the push was redirected to the evil destination through the substituted chain.
- GREEN: `publicationPinAbsentXDGConfigAuthority` pins `GIT_CONFIG_GLOBAL` to the canonical home config for the protected callback whenever `GIT_CONFIG_GLOBAL` is absent and the default XDG config file does not exist under the held seal, making any substituted XDG identity unreadable while preserving the verified effective configuration exactly; failures to pin fail closed before the callback.
- Safe cases retained GREEN: absent-XDG availability, transient-create seal (B-70), global rewrite race, missing-parent fail-closed, owner-controlled rejection, macOS root-owned system config success.
- Evidence: `.issueops/verified-execution/evidence/issue46-B79-xdg-identity-replacement.txt`; ledger entry B-79 in `.issueops/ISSUEOPS_ORCA_BLOCKERS_2026-07-16.md`.
- Gate wave from the final worker HEAD: focused/named -v, gofmt, diff --check, windows cross-compile, full go test, full -race, vet, contract/response goldens, 19 skill validators plus validator unit tests, and neutral-name build all exit 0. Independent adversarial review returned UNCONDITIONAL APPROVE with zero Critical/Important findings.
- Self-verify failed deterministically at step 23/25 "native integration" only, because the host's `~/.codex/hooks.json` is absent — an environment regression since the 2026-07-17 25/25 pass that this diff cannot influence and the worker fence cannot repair; recorded as B-80 with the coordinator-owned restoration and re-verification path.
- No other new coordinator/runtime/hook blocker beyond the already-recorded B-72–B-78 families was observed; re-observed fences (claim-before-mutation exact command, orca-channel block, shell-control fence, outside-worktree read fences) matched their documented escapes.
