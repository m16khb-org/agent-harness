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
- Supervised create refuses a non-`pr` phase or an existing `RemoteArtifact` before provider mutation. GitHub and GitLab creates and immediate readbacks use bounded, timed, redacted subprocess capture; exact canonical URL/head/base/draft and requested label/assignee inclusion are verified. The continuation below replaces the original GitLab `glab api` readback with a same-canonical-full-URL `glab mr view` contract so custom web ports are preserved without a bespoke HTTP adapter. Invalid output is not exposed, and post-start timeout/nonzero/malformed/mismatch is unknown/needs-reconciliation without retry. Provider success then passes through durable `IssueURL` project authority and atomic `RemoteArtifact` persistence in `VerifyIssueOpsRemoteArtifact`; legacy nil-handoff retains manual verification behavior.

## Schema-v6 and remote-create continuation

Task/dispatch: `task_3921e58c8849` / `ctx_f6e944788f99`

The resumed writer separately attested the exact cwd and git root, branch `16-orca-supervised-handoff`, immutable parent `0af38c679b849f4e6eda6b51128f2c9522950899`, dirty-path inventory, and a clean observation-only main checkout. Raw Orca inventory reported one exact-worktree terminal (`totalCount=1`, `truncated=false`) and one server-filtered dispatched task: this task assigned to terminal `term_5f020a6b-8d10-4048-9ab0-88f0e74be19f`. The resumed banner remained `gpt-5.6-sol high` after the natural reset, without `/usage`; an exact-cwd app-server probe reported enabled/trusted `SessionStart` and `PreToolUse` hooks with zero warnings or errors and exited cleanly.

### Continuation RED/GREEN evidence

- RED: copied coordinator mailbox text plus a worker-native session/source cwd initially passed remote-create reconciliation; GREEN: one shared sealed native host/session/agent/source-cwd verifier now rejects the falsifier in both lifecycle hooks and core, while the mailbox handle remains routing-only.
- RED: source-project mismatches and order-sensitive reconcile sets initially passed or failed for the wrong reason; GREEN: GitHub fork origin and GitLab source-project mismatch are rejected, and labels/assignees compare as canonical sets.
- RED: the provider request could retain pre-claim values; GREEN: the captured provider request equals the complete canonical claim projection, including canonical project, title, rendered-body hash, `FinalHead`, branch/base, labels, assignees, draft, remote/ref, receipt fingerprint, and exact claim identity.
- RED: the immutable-push fake and MCP lifecycle parity fixture exposed the new config-origin and authenticated coordinator/readback requirements; GREEN: the fixtures now exercise the exact shared production paths and pass without weakening the checks.
- Real-git coverage proves bounded recursive `insteadOf`/`pushInsteadOf` resolution and rejects cycles, overflow, ambiguous rewrites, incomplete origins, pre-existing effective include locks, and the alias-to-good-to-evil authority swap before push or provider invocation. Deterministic config locks cover common/worktree/user/XDG and effective include files through re-enumeration, push, and readback.
- Publication and reconciliation use target → receipt → local head → exact ephemeral remote-ref head ordering. Reconcile validates before probe and immediately before finalize/approved zero-clear; force-push or fingerprint drift retains the unknown claim.
- Schema v6 enforces bounded claims/receipts and claim/artifact exclusion. Raw schema-v5 authority-bearing rows remain byte-identical on rejection; the locked old-receipt re-attestation path performs bounded exact target/local/remote verification without inventing claim authority or pushing, while valid v5 without new authority migrates and v7 is rejected.
- CLI and MCP now share one durable request-bound create wrapper and one reconcile projection. Named two-process coverage proves a single provider invocation; post-claim `Invoked=false` validation failure clears only the exact live claim, while invoked/ambiguous failures retain unknown state and ordinary verify stays blocked.
- GitHub uses explicit `HOST/OWNER/REPO` selectors and exact `@me` username resolution only for the legacy GitHub placeholder. GitLab uses the same canonical full HTTPS URL for `mr create` and `mr view`; custom-port or IPv6 authority requires proven glab `>=1.82.0`, with visible v1.81 rejection/v1.82 acceptance for custom port, implicit-port IPv6, and explicit-443 IPv6 before push/provider calls.
- Independent review sequence 525 found that provider reconciliation projected nonempty title/body but then performed a second create-style readback with an empty title/body request. Named GitHub and GitLab tests reproduced the false rejection; reconciliation now returns the single bounded list projection to the shared core verifier without a duplicate provider readback, and core retains exact durable-claim title/rendered-body-hash rejection tests.
- Independent review sequence 541 closed three further authority gaps and four ambiguity gaps. MCP start now matches coordinator identity fields to the native hook event before core sealing; accept/publish carry and verify the sealed host/session/agent/source cwd in CLI, MCP, lifecycle, and core before any push. Publication locks every full-enumeration effective config origin, including unrelated global files, with real-git regressions proving pre-existing locks fail and a push-time good-to-evil rewrite cannot redirect the destination. A literal frozen v5 receipt without project/base/fingerprint migrates only after no-push live re-attestation. Reconcile searches project+head without base prefiltering, rejects JSON `null`, and lets the shared core reject base drift; supervised body whitespace is canonicalized once before claim, hashing, provider request, and readback while nil-handoff legacy remains unchanged.

### Ordered verification receipt

- Focused immutable push, MCP parity, provider source/selector/readback, lifecycle authority, schema-v6/raw-v5, shared CLI/MCP, two-process single-call, and glab 1.81/1.82 boundary tests: PASS with visible named `RUN`/`PASS` output.
- `gofmt` on explicit Go paths and `git diff --check`: exit 0; no repository `*.lock` residue.
- `go test ./... -count=1`: exit 0; `go test -race ./... -count=1`: exit 0.
- `go build -o bin/agent-harness ./cmd/harness`: exit 0.
- IssueOps and Turing skill validators: exit 0 (`Skill is valid!`).
- CLI/MCP contract goldens and response-contract goldens: exit 0.
- `self-verify --seed=100 --target-score=95 --llm-eval=false --json`: exit 0, `ok=true`, 24/24 steps passed, minimum goal score 100, termination eligible.

The final commit SHA, its literal Lore proof, the post-commit clean-worktree receipt, the bounded current-task inbox, and the repeated sole-writer inventory are intentionally recorded in the external `worker_done` receipt: a commit cannot contain its own content hash.

### IssueOps/Turing real-loop receipt (inbox sequence 546)

Success criteria: the exact worker checkout and branch are durably recorded in a schema-v6 IssueOps cycle; a real problem-to-grill transition and `grill:turing` routing event are persisted and scored; applicable `ORCA-01` through `ORCA-14` observations are binary; all scenario-only state is removed without remote mutation.

Evidence artifact: this report plus the captured stdout for isolated cycle `io-cd1d06f2dced`. The scenario used only `HARNESS_STATE_DIR=/tmp/agent-harness-issueops-turing.hS2HFG` and the exact worker checkout `/Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-supervised-handoff`; it created no worktree, terminal, task, dispatch, push, issue, PR, or MR.

Exact scenario commands and terminal results:

- `mktemp -d /tmp/agent-harness-issueops-turing.XXXXXX` → exit 0, `/tmp/agent-harness-issueops-turing.hS2HFG`.
- `HARNESS_STATE_DIR=/tmp/agent-harness-issueops-turing.hS2HFG ./bin/agent-harness issueops start --repo /Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-supervised-handoff --branch 16-orca-supervised-handoff --json` → exit 0, `ok=true`, schema 6, cycle `io-cd1d06f2dced`, phase `problem`.
- The first direct `phase --to implement --force` observation exited 1 and listed the sealed missing readiness artifacts; the next direct `phase --to grill` observation exited 1 with `missing intent_contract`. Failure-first triage therefore recorded the required intent instead of bypassing readiness.
- `... issueops intent record --id io-cd1d06f2dced ... --intent-class standard --json` → exit 0; the durable intent includes three binary success criteria, `No remote mutation`, and the handoff non-goal.
- `... issueops phase --id io-cd1d06f2dced --to grill --json` → exit 0; `problem.completed_at` and `grill.entered_at` were persisted with the `intent_contract` artifact.
- `... issueops record-routing --id io-cd1d06f2dced --phase grill --skill turing --json` → exit 0; the durable routing trace contains exactly `grill:turing`.
- `... issueops routing-score --id io-cd1d06f2dced --expect grill:turing --json` → exit 0, `ok=true`.
- `... issueops status --id io-cd1d06f2dced --json` → exit 0; schema 6, exact repo/branch, phase `grill`, intent, routing trace, and phase ledger read back from the isolated store.

Binary ORCA observations:

| Criterion | Result | Current binary observation |
|---|---|---|
| ORCA-01 | PASS | `TestWorktreePrepareAutoProbeFailurePreservesLegacyInlineResult` ran and passed. |
| ORCA-02 | PASS | `TestWorktreePrepareExplicitOrcaProbeFailureHasProbeOnlyTrace` ran and passed. |
| ORCA-03 | PASS | `TestHandoffStartCrashMatrixNeverRepeatsCreate` ran all terminal/task/dispatch crash subtests and passed. |
| ORCA-04 | PASS | `TestWorktreePrepareCrashAfterInvocationNeverCreatesTwice` ran and passed. |
| ORCA-05 | PASS | `TestWorktreePrepareExactOneMarkerRecovery` ran zero/one/sibling/multiple subtests and passed. |
| ORCA-06 | PASS | `TestIssueOpsExecutionHandoffRejectsStaleAttemptEpochAndContext` ran attempt/epoch/context subtests and passed. |
| ORCA-07 | PASS | `TestHandoffGuardBlocksBeforeClaim`, `TestHandoffGuardAllowsMatchingClaimedWorkerInTree`, and `TestHandoffGuardBlocksWrongOrRestartedSession` ran and passed. |
| ORCA-08 | PASS | `TestIssueOpsExecutionHandoffTransitionTable` ran and passed. |
| ORCA-09 | PASS | `TestHandoffResumeIsReadOnly` ran and passed. |
| ORCA-10 | PASS | `TestIssueOpsReadNormalizesLegacySchemaVersion` and `TestIssueOpsReadRejectsFutureSchemaVersion` ran and passed. |
| ORCA-11 | PASS | The deterministic/bounded, secret-redaction, and source-hash-change context tests ran with visible names and passed. |
| ORCA-12 | PASS | The exact claimed Codex hook test plus sealed coordinator shell/MCP/native-session falsifier tests ran and passed. |
| ORCA-13 | SKIPPED | The sealed current task forbids `worker_done`, acceptance, and cleanup before an unconditional independent-review GO; no substitute finish or external mutation was invented. This criterion remains a final lifecycle receipt, not pre-GO authority. |
| ORCA-14 | PASS | Focused authority/provider/schema/CLI/MCP/hook suites, full Go and race suites, build, goldens, both skill validators, diff check, and deterministic 24/24 self-verify all reached exit 0 before this evidence-only append and are restarted below after it. |

Cleanup receipt: `rm -rf /tmp/agent-harness-issueops-turing.hS2HFG` → exit 0; `test ! -e /tmp/agent-harness-issueops-turing.hS2HFG` → exit 0. No scenario process, terminal, worktree, task, dispatch, remote ref, or provider artifact was spawned.

Verification mode: full local IssueOps/Turing scenario plus named ORCA contract observations and the ordered repository gate; remote/lifecycle-finalization actions remain bounded by the sealed worker packet.

Skipped checks: only ORCA-13's actual finish/accept/cleanup observation, for the explicit pre-GO authorization reason above. No remote probe or mutation was substituted.

### Fresh-review sequence 556 secret-durability correction

The authority reviewer returned Critical 0 / Important 0, while the independent integration reviewer found Critical 1 / Important 1, so the result was not GO. The Critical RED used only synthetic markers: both `TestCreateRemotePullRequestRejectsSecretLikeTitleAndBodyBeforeClaim` subtests first persisted the title/body marker into a durable claim, and both `TestRemoteCreateClaimEnvelopeRejectsSecretLikeTitleAndBody` subtests first accepted the corrupted envelope. No real credential was used or printed.

GREEN applies the shared bounded `policy.RedactFreeform` classification at all three supervised boundaries: before publication validation/claim, again inside the locked claim operation, and during schema-envelope validation. Secret-like supervised title/body now returns a bounded value-free diagnostic before claim or provider invocation, keeps the raw state bytes unchanged, and leaves no synthetic opaque marker. `TestCreateRemotePullRequestPreservesSecretLikeLegacyInputBytes` proves a nil-handoff request still reaches the legacy provider callback byte-for-byte and does not mutate state.

Named correction receipt after the production edit:

- `go test -v ./internal/core/issueops -run '^(TestCreateRemotePullRequestRejectsSecretLikeTitleAndBodyBeforeClaim|TestRemoteCreateClaimEnvelopeRejectsSecretLikeTitleAndBody|TestCreateRemotePullRequestPreservesSecretLikeLegacyInputBytes)$' -count=1` → exit 0; all title/body and legacy subtests emitted visible RUN/PASS.
- `go test ./internal/core/issueops -count=1` → exit 0 (`82.315s`).

Final ordered post-report gate receipt (all commands ran after the preceding IssueOps/Turing and sequence-556 evidence sections were appended):

- `gofmt -w internal/core/issueops/issueops_remote_create_claim.go internal/core/issueops/issueops_remote_create_claim_test.go internal/core/issueops/handoff/envelope.go` → exit 0; `git diff --check` → exit 0.
- `python3 scripts/validate-skill.py skills/issueops` and `python3 scripts/validate-skill.py skills/turing` → exit 0, `Skill is valid!` for each.
- `go test -v ./cmd/harness/contractgolden -run Golden -count=1` → exit 0 with visible CLI usage, MCP tools, and MCP resources golden RUN/PASS; `go test -v ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1` → exit 0 with visible RUN/PASS.
- `go test ./... -count=1` → exit 0; the corrected IssueOps package passed in `86.856s`.
- `go test -race ./... -count=1` → exit 0; the corrected IssueOps package passed in `119.012s` and the lifecycle package in `13.142s`.
- `go build -o bin/agent-harness ./cmd/harness` → exit 0.
- `./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json` → exit 0, `ok=true`, 24/24 steps passed, minimum goal score 100, termination eligible, elapsed `322100ms`; its internal Go test and risk-QA stages included the corrected tree.

Because writing this literal receipt is itself an evidence-only edit, the same ordered affected gate is run once more after this line; that terminal exit bundle is carried in the next independent review message without another report mutation.

### Redispatch fresh-review Critical corrections

The fresh read-only reviewer returned `NO GO — Critical 2, Important 0`. Critical 1 reproduced an exact namespace collision: `mcp__evil__issueops_remote_create_pr` reached an allow decision because privileged tool recognition used `HasSuffix`. Critical 2 used literal HEAD-era schema-v5 publication bytes with `coordinator_session` removed: the old receipt returned no bounded projection, disappeared from closed-handoff scans, and could not pass the v6-only coordinator verifier.

Named RED evidence before production edits:

- `TestHandoffLifecycleBlocksForeignMCPNameCollisions` failed because the foreign create tool returned `Decision:"allow"`; `TestLiteralV5CoordinatorSealRequiresExplicitSourceApprovalInShellAndMCP` failed because the explicit transition was not represented.
- `TestSupervisedHandoffCyclesKeepsInvalidClosedV5PublicationAuthority` failed with an empty active set.
- `TestRawSchemaV5PublishReceiptHasExecutableLockedReattestationToV6` failed at the missing legacy seal path, and `TestSchemaV5OldPublishReceiptRequiresReattestOrReconcileWithoutRewriting` showed a zero-value projection.

GREEN now uses exact bare or exact `mcp__agent_harness__` tool identities, retains a bounded invalid closed-v5 receipt projection in lifecycle scans, and shares `LegacyCoordinatorIdentityCanBeSealed` between lifecycle and core. The explicit CLI/MCP `approve_legacy_coordinator_seal` input is accepted only from the exact source checkout with matching native event identity; the implicit attempt leaves raw v5 bytes identical, and the successful locked re-attestation writes the coordinator seal and v6 receipt atomically after target/local/remote verification. All four named RED groups now emit visible PASS; the focused lifecycle, active, IssueOps, CLI, MCP, and catalog packages exit 0. The ordered full-gate receipt after this evidence edit follows in the external review packet so this report does not recursively invalidate its own final command set.

### Redispatch fresh-review Important corrections

The next fresh read-only review returned `NO GO — Critical 0, Important 2`. The first finding showed that the real `BuildLifecyclePreToolUseDecision` path rejected every invalid projection before reaching the explicit raw-v5 publication seal verifier. The second showed that privileged MCP identity comparison lowercased and trimmed the supplied tool name, so case and surrounding-whitespace aliases could enter the authority path.

Named RED evidence before production edits:

- `TestLiteralV5CoordinatorSealRequiresExplicitSourceApprovalInShellAndMCP` persisted literal raw-v5 bytes and failed because the actual lifecycle guard returned `Decision:"block"` for the explicit source-native shell seal.
- `TestHandoffLifecycleBlocksForeignMCPNameCollisions` failed because uppercase `MCP__AGENT_HARNESS__ISSUEOPS_REMOTE_RECONCILE_CREATE` returned `Decision:"allow"`.

GREEN admits an invalid record before the generic fail-closed branch only when it is the exact raw-v5 authority shape, exact source/native identity, exact `handoff publish` shell command or exact `issueops_handoff` MCP tool, and carries explicit approval plus confirmation. Privileged MCP recognition now compares the original tool string byte-for-byte against only the four bare names or four `mcp__agent_harness__` names; case, leading/trailing whitespace, foreign namespace, prefix, and suffix collisions remain blocked through the real lifecycle decision builder. Both named tests emit visible PASS and the complete lifecycle package exits 0; the ordered repository gates and next independent review consume this final report content without another evidence edit.

### Final-review frozen-reader and legacy-output corrections

The next independent review returned `NO GO — Critical 0, Important 2`: the v5+claim test exercised the current v6 decoder rather than a frozen-v5 reader rejecting raw v6, and legacy create tests did not pin user-visible CLI/MCP bytes. The output-golden RED failed on the absent CLI text/JSON and MCP content/response fixtures. The undefined-helper compile failure was not counted as RED; after the fixture compiled, its version bound was deliberately opened to v6 and the named test produced two valid runtime REDs: the frozen reader returned nil instead of rejecting raw v6, and its read-modify-write changed the stored bytes.

GREEN restores the frozen-v5 boundary and uses a sqlstore read-modify-write fixture parallel to the historical v1 through v4 fixtures. `TestFrozenSchemaV5ReaderRejectsRawSchemaV6WithoutRewriting` now has independently visible rejection and byte-preservation subtests; both pass against a real raw schema-v6 row, while the separate raw-v5 claim rejection remains intact. The current v6 ADR section states both directions explicitly: authority-free historical v5 rows may be preserved and stamped only on a valid subsequent write, while a frozen v5 reader rejects v6 bytes before rewrite.

`TestRunRemoteCreatePRLegacyAtMeReachesProviderWithoutStateMutation` now captures the real CLI stdout path and pins byte-exact text and indented JSON fixtures while preserving exact `@me`, whitespace normalization, draft false, and state bytes. `TestMCPRemoteCreatePRLegacyAtMeParityAndNoStateMutation` now invokes the real MCP SDK handler, pins both its exact text-content JSON and serialized MCP response envelope, and retains provider argv/state assertions. The named GREEN run emits visible PASS for all three tests; the full ordered gate and a new fresh review follow after this report edit.

### Final-review raw-v5, GitLab adapter, and reconcile corrections

The next fresh review returned `NO GO — Critical 1, Important 2`. Critical RED `TestRawSchemaV5RejectsCoordinatorSessionWithoutRewriting` used a canonical copied native session plus mailbox in raw schema-v5 bytes and the current reader returned nil, proving that v6-only coordinator authority survived normalization. GitLab RED `TestGitLabCreateGatesCustomPortAndIPv6OnGlab182BeforeMutation` showed v1.81 and unparseable capability executed `mr create` for custom-port, implicit-port IPv6, and explicit-443 IPv6 authorities; v1.82 controls succeeded. Reconcile REDs showed `MarkIssueOpsRemoteCreateUnknown` accepted a different URL and the forced finalize seam returned only the primary error without persisting the verified candidate URL.

The earlier combined gofmt-and-characterization shell command is not evidence. Before these REDs, explicit `gofmt` exited 0 and the one-candidate characterization was rerun as a separate command with visible PASS and exit 0, proving the finalize seam itself did not change behavior.

GREEN raw decoding rejects schema-v5 `coordinator_session` authority before normalization and preserves the sqlstore bytes. The GitLab create adapter repeats the glab v1.82 capability gate immediately before mutation and returns typed `Invoked=false`; all nine old/unknown/current custom-port and IPv6 boundary cases emit visible PASS. Reconcile now preserves an existing known URL, rejects a different candidate, attempts an exact-claim unknown transition with the canonical verified URL after finalize failure, combines a secondary transition failure, and reports needs-reconciliation/no-retry guidance. The named known-URL, finalize durable-state, and combined-error tests emit visible PASS. The ordered full gate and another fresh review follow this final report edit.

### Final-review migration fallback, claim-swap, and diagnostic corrections

The next fresh review returned `NO GO — Critical 1, Important 2`. `TestRawSchemaV5ReattestationRejectsInjectedCoordinatorSessionWithoutRewrite` first proved that the locked fallback accepted and rewrote a raw v5 old receipt containing an injected matching coordinator session. The post-probe claim-swap RED changed ClaimID during final publication readback and observed nil error, proving the stale candidate could finalize against the replacement claim. Provider diagnostic REDs passed 128-KiB secret-like URLs to the GitHub/GitLab created-artifact error paths and observed unbounded raw inclusion.

GREEN makes the locked raw-v5 fallback inspect and reject `coordinator_session` before unmarshalling or any publication read, preserving the row bytes. One-candidate reconciliation compares the complete durable claim after the probe and again after publication revalidation, so a replacement ClaimID/full claim cannot receive the stale candidate URL. GitHub, GitLab, and core reconciliation diagnostics no longer interpolate returned candidate URLs: the canonical URL remains in the typed result or durable claim, while the error remains bounded, redacted, and explicit about reconciliation/no retry. The raw fallback, claim-swap, provider mismatch, oversized secret-like diagnostic, and core durable-URL tests all emit visible PASS; the ordered gate restarts after this report edit.

### Final-review absent config authority and inner-error sanitization

The next fresh review returned `NO GO — Critical 1, Important 1`. `TestPublicationMissingConfigAuthorityParentFailsBeforePushAndLeavesDestinationsUnchanged` first showed both absent-XDG and absent effective-include-parent cases returned nil and pushed the ref. Provider and core diagnostic REDs then placed the 128-KiB URL/secret in the wrapped inner error—not only the discarded URL parameter—and observed 128–262-KiB leakage.

GREEN always includes the user XDG config candidate, rejects an unresolved user home, and fails closed if any common/worktree/user/XDG/rewrite/include authority parent is absent instead of silently skipping it. Both absent-parent subtests now stop before push and prove neither the intended nor unintended bare destination received the ref. Shared `policy.RedactDiagnostic` removes secret-like content and raw HTTP(S) URLs; providerutil and the core claim wrapper impose hard diagnostic bounds before composing errors, including invoked create errors, probe failures, and secondary transition failures. All absent-parent, GitHub, GitLab, and core inner-error tests emit visible PASS; the ordered gate and final review restart after this report edit.

### Final-review shared reconcile projection correction

The next fresh review returned `NO GO — Critical 0, Important 1`: CLI and MCP independently constructed the durable claim to provider-reconcile request, so their parity was accidental and there was no shared adapter contract. Before the RED, explicit `gofmt` exited 0 and the exact one-candidate characterization `TestRemoteCreateReconcileAuthorityZeroOneManyAndAmbiguity/exactly_one_verified_candidate_finalizes` ran separately with visible PASS and exit 0; no combined formatting/test command is counted as evidence.

Named RED `TestRemoteCreateReconcileAdaptersUseOneDurableClaimProjection` failed at runtime for both adapter source paths because neither called the shared projection and both contained a direct provider request literal. GREEN adds `ProjectIssueOpsRemoteCreateClaimForProviderReconcile` in core, makes both actual CLI and MCP reconcile callbacks call it, and removes both duplicated literals. `TestRemoteCreateReconcileProviderRequestEqualsCompleteDurableClaimProjection` pins repo, canonical project key, head/base, final-head SHA, title, rendered-body SHA-256, labels, assignees, draft, and non-aliasing slices; the structural adapter parity test and complete projection test both emit visible PASS, while focused CLI, MCP, and zero/one/many reconcile packages exit 0.

The valid frozen-reader evidence remains the runtime RED/GREEN described in “Final-review frozen-reader and legacy-output corrections”: an actual raw schema-v6 sqlstore row was rejected by the frozen-v5 reader and its stored bytes were preserved. The earlier undefined-helper compile failure remains excluded from RED evidence. The ordered full gate and unconditional independent review follow after this final evidence edit.

### Final-review lifecycle projection and live-invocation exclusion corrections

The next independent review returned `NO GO — Critical 2, Important 0`. First, the raw-v5 injected `coordinator_session` rejection returned only an ID, so the rejected supervised handoff disappeared from lifecycle scans. Second, approved authoritative-zero reconciliation could probe and clear a durable claim while its provider create callback was still in flight.

Named runtime RED `TestRawSchemaV5RejectsCoordinatorSessionWithoutRewriting` rejected the row and preserved its bytes but returned no invalid handoff projection. GREEN decodes the same bounded invalid projection used for other unsupported authority, marks it invalid with a bounded reason, and keeps repo/worker authority available to the hook scan; `TestSupervisedHandoffCyclesKeepsInvalidV5CoordinatorAuthority` pins that fail-closed lifecycle retention.

Named runtime RED `TestRemoteCreateReconcileAuthorityZeroOneManyAndAmbiguity/authoritative_zero_cannot_clear_while_provider_create_is_in_flight` reached the reconcile probe before the blocked provider callback returned. GREEN uses a dedicated per-cycle sqlstore span shared by create and reconcile: create holds it from before the atomic claim through provider completion and the final durable transition, reconcile holds it from before its first read through clear/finalize. The span is process-death-safe, so a crash releases the live exclusion while leaving the durable ambiguous claim available to the dedicated reconcile path. The regression now shows reconcile cannot probe during the live callback, provider success finalizes first, the later stale reconcile rejects the missing exact claim, and no claim/artifact authority is lost. Focused IssueOps, active, lifecycle, CLI, and MCP suites exit 0; the ordered full gate and another fresh review follow this final evidence edit.

### Commit-boundary adapter capture parity correction

The bounded pre-commit inbox reported that the structural shared-helper contract alone was not adapter capture parity. The unconditional GO was therefore held before staging. Production CLI and MCP reconciliation callbacks are now individually executable factories used by the real handlers, while both still call the one core durable-claim projection.

`TestCLIAndMCPRemoteCreateReconcileCallbacksCaptureIdenticalCompleteClaimProjection` invokes both production callback factories with the same canonical GitLab nested/custom-port claim, captures provider name and request at each provider boundary, and compares both captures byte-for-field against the shared core projection. It pins repo, ProjectKey, head/base, FinalHead, title, body SHA-256, labels, assignees, and draft; the named runtime test emits visible PASS. The earlier structural RED remains evidence of the removed duplication, while this runtime capture regression is the required adapter parity evidence. The ordered full gate and fresh review restart after this final report edit.
