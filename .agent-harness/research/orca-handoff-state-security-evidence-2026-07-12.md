# Orca Handoff State/Security Evidence — 2026-07-12

Issue: GitHub #16
IssueOps cycle: `io-47c93d1ef742`
Task/dispatch: `task_826582536d33` / `ctx_b264706d35c9`

## Scope and lease

The fresh worker attested the exact worktree, branch, base HEAD, clean status, runtime identity, complete non-truncated terminal inventory, and all dispatched tasks before mutation. Every connected or writable terminal was treated as a possible writer; the designated dispatched worker was the only terminal that was both, and the exact task/dispatch was the only writing assignment. No sub-agent, real remote mutation, host install/update, PR/MR create, push, acceptance, or worktree removal was used.

Bootstrap exception: issue #16 cycle `io-47c93d1ef742` predates `execution_handoff`; its schema-v4 record has `branch_prepare.base_sha=2ba240b94477190071598b3f1c7278312b296611` and no supervised envelope. `fe2ed683bd02a5b0e7b029eb10a82e59777b9dbb` is the observed pre-correction local/remote published branch HEAD and `ai_slop_clean_head`, not the cycle base. The new automatic publication fence therefore cannot protect this current cycle and must continue to refuse nil-handoff publish. Only after this worker fully exits may the coordinator verify exact untruncated terminal/task inventory, the immutable commit SHA, and local/remote ref equality. Automatic publication-fence claims below apply only to future supervised envelopes.

## RED evidence

- Sole-writer tests failed before the core treated any pre-existing baseline terminal that was connected or writable as a competing possible writer and before it inspected server-filtered dispatched tasks immediately before create/dispatch.
- Retry fencing failed before durable retry cleanup receipts, exact terminal/task/dispatch quiescence, and a final complete sole-writer attestation were required.
- Publication tests initially failed to compile because no provider-neutral receipt model/API existed; lifecycle tests then showed wrong local FinalHead, missing/stale remote receipt, direct PR/MR create, and arbitrary PR body-file paths were not fenced equally for GitHub and GitLab.
- A real submitted record carrying a terminal `worker_done_projection` could not force-cancel/finalize because the envelope rejected the projection outside submitted/accepted states.
- Cleanup tests initially failed to compile because cleanup approval/disposition and ordered receipt DTOs/actions did not exist; lifecycle guards allowed cleanup operations without the durable sequence.
- Auto fallback characterization failed before orchestration-only JSON/text fields were removed and durable row/state-artifact byte equality was asserted.
- Baseline terminal inventory errors initially returned only `list terminals before create` and did not persist `recovery_required`.

## GREEN evidence

- Complete exact-worktree terminal and server-filtered dispatched-task inventories fence every harness create/dispatch boundary. Baseline terminals that are connected or writable and dispatched assignments block; only the designated active worker must be both. Ambiguous/truncated/unparseable/incomplete identity becomes durable `lease_attestation` recovery. Because Orca exposes no atomic lease, snapshots detect and refuse observed external writers but cannot prove absolute exclusion against an actor racing after the final snapshot.
- Retry preserves the prior task/terminal/dispatch identities until cleanup disposition `retry`, ordered task/terminal receipts, exact quiescence, and immediate sole-writer re-attestation all pass.
- Accepted publication derives provider/remote/branch/base/ref/FinalHead only from durable IssueOps authority. Exact local and remote heads create one schema-v5 provider-neutral receipt; GitHub and GitLab wrapper creation revalidates it and rejects direct create or PR/MR body-file paths.
- Submitted terminal completion projection survives force-cancel and deterministic quiescent finalization. Failed/cancelled cleanup requires prior disposition and ordered idempotent `task_terminal` and `terminal_quiescent` receipts. Raw supervised worktree removal remains blocked; after an explicit user-directed external/manual deletion, complete canonical inventory may record optional `worktree_removed`. Accepted cleanup remains forbidden.
- Pre-external-mutation `auto` fallback is byte-identical to the legacy JSON and human output and leaves the IssueOps row and state directory unchanged.
- Root schema v5 preserves v4 sealed identity without rerunning legacy mailbox migration; the frozen v4 reader rejects v5 byte-equivalently.

## Focused verification observed before final gate

- `go test ./internal/core/issueops -count=1` — PASS (`71.069s`).
- `go test ./internal/core/lifecycle ./internal/adapter/orca ./internal/adapter/mcp ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli/... -count=1` — PASS.
- Schema migration/rejection, baseline ambiguity recovery, fresh operation timestamps, and legacy v3 live-terminal migration focused tests — PASS.

## Final ordered gate

The final report records gofmt, focused packages, both skill validators, diff check, full test, race test, build, both response-contract goldens, deterministic self-verify, exact inbox sequence, commits, and clean-status evidence after they reach terminal exit codes. The final commit SHA is intentionally carried in the external `worker_done` receipt because embedding a commit's own SHA in this committed file would change that SHA.

## Coordinator correction sequences 450–458

- Sequence 450 recorded an unauthorized host model switch after a usage-limit dialog. This worker did not attempt another switch or reset. The IssueOps skill/reference and SessionStart hook guidance now treat usage-limit, rate-limit, reset, and model-selection prompts as user-decision boundaries: dismiss/stop and relay, never navigate or confirm automatically.
- Sequences 451 and 454 required publication and cleanup inventories to treat `Connected OR Writable` as a possible writer and persist publication ambiguity durably without changing accepted result authority.
- Sequences 455–458 preserved explicit inline semantics and moved push inside the supervised publish action. Raw accepted `git push` is blocked; the action re-attests all writers, pushes the immutable full FinalHead object ID as `<FinalHead>:refs/heads/<branch>` without force, verifies the remote equality, and only then persists the receipt. A production-argv test captures the exact refspec.
- The yielded-package verification was resumed through its original session to an explicit exit. No duplicate overlapping test was started; the canonical skill and CAUTIONS now preserve that operational rule.
- The final Brooks correction classifies known sole-writer conflicts explicitly. Create/dispatch persist `lease_attestation` recovery for both known terminal/task conflicts and ambiguous inventories; publication persists its accepted-state recovery marker for either class. Cancellation finalization reuses the complete terminal/dispatched-task attestation and cannot close while any exact-worktree terminal is connected or writable or any dispatched assignment remains.
- A second all-writer attestation now runs after each `BeforeJournal` seam and before the pending journal/external call. A dispatch-stage competitor test observes zero terminal/task/dispatch mutation and exactly one durable `recovery_required` transition with pending `lease_attestation` and `sole_writer_conflict`.
- Provider-neutral `Draft` is true only for supervised envelopes. GitHub emits `--draft`; GitLab emits `--yes --draft` and excludes `--push`/`--fill`; nil-handoff legacy requests retain non-draft argv.
- Supervised create refuses a non-`pr` phase or an existing `RemoteArtifact` before provider mutation. GitHub and GitLab creates and immediate readbacks use bounded, timed, redacted subprocess capture; exact canonical URL/head/base/draft and requested label/assignee inclusion are verified, with GitLab using the parsed host/project/IID through `glab api`. Invalid output is not exposed, and post-start timeout/nonzero/malformed/mismatch is unknown/needs-reconciliation without retry. Provider success then passes through durable `IssueURL` project authority and atomic `RemoteArtifact` persistence in `VerifyIssueOpsRemoteArtifact`; legacy nil-handoff retains manual verification behavior.
