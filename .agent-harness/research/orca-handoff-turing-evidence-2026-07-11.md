# Orca Handoff Turing Evidence — 2026-07-11

## Superseded nested-layout attempt cleanup

- IssueOps cycle: `io-7e43bae11120`
- Branch: `16-orca-live-20260711012631`
- Ownership epoch: `51a320e61d5618c11661755bb89429ee`
- Orca worktree ID: `682d2268-d88a-4b01-8728-25e967d26a76::/Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-live-20260711012631`
- Orca worktree instance ID: `a8b5abc7-71ef-44ac-879a-6eacc29523ad`
- Rejected path: `/Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-live-20260711012631`
- Required path: `/Users/m16khb/Workspace/agent-harness.worktrees/16-orca-live-20260711012631`
- Recovery receipt: `handoff recover --action cancel --confirm` returned `state=closed`, `disposition=cancelled`, `attempt=1`, and the persisted `pending_operation` became `null`.
- Terminal receipt: exact-ID `terminal stop` returned `stopped=2`; `term_03a9252c-dedd-4872-b9af-be9dab21e211` / `ssh:ssh-1783678538888-sl28jq@@pty-21` and `term_0d118962-fffd-468c-a61a-ce4a16afc622` / `ssh:ssh-1783678538888-sl28jq@@pty-22` both reported `connected=false`, `writable=false`, and `wait.status=exited`.
- Worktree receipt: exact-ID `worktree rm --force` returned `removed=true`; the same ID then returned `selector_not_found`, and `git worktree list --porcelain` contained neither the rejected path nor branch.
- Branch receipt: the local branch was absent after worktree removal. Read-only `git ls-remote --heads origin refs/heads/16-orca-live-20260711012631` was empty; no remote-ref deletion was attempted.
- Task receipt: no disposable orchestration task or dispatch was created for this attempt.

Root cause: Orca 1.4.134 had global **Nest Workspaces** enabled, so its documented path calculation appended the repository name between the configured `../agent-harness.worktrees` root and the worktree name. Nested-path acceptance code and tests were reverted; the canonical validator remains strict.

## Scope and durable identities

- Issue: GitHub issue #16.
- Primary IssueOps cycle: io-47c93d1ef742.
- Implementation worktree: /Users/m16khb/Workspace/agent-harness.worktrees/agent-harness/16-orca-supervised-handoff.
- Implementation branch: 16-orca-supervised-handoff.
- Base SHA: 2ba240b94477190071598b3f1c7278312b296611.
- Plan: docs/superpowers/plans/2026-07-11-orca-aware-issueops-handoff.md.
- Plan SHA-256: 4b903845dc7ead19f205ca9d45f8aeeff673658ffe49cda7b9a9be2ab00b3ec1.
- Design: docs/superpowers/specs/2026-07-11-orca-aware-issueops-handoff-design.md.
- Design SHA-256: 34eb992f809593a9341160aac2a31f76dd23f9c3dcc7cbfc79b2e39cdeb9d8f0.
- Verification mode: full Turing loop with focused, full, race, build, golden, native-host, installed CLI, live E2E, cleanup, and self-verify channels.

## Implemented contract

IssueOps remains the only durable authority. Orca is an optional bounded process adapter: auto readiness failure preserves the inline path with execution_handoff absent, while explicit Orca failure stops before mutation. Every external create or dispatch is preceded by a durable pending_operation checkpoint, and any ambiguous post-invocation failure enters recovery_required without automatic retry or inline fallback.

The implementation adds:

- Versioned execution_handoff state, deterministic/redacted/bounded worker context, CAS-fenced claim, heartbeat, finish, submit, accept, cancel, retry, and reconcile transitions.
- A concrete Orca CLI adapter using argv execution and public v1.4.134 surfaces only; no generic registry, private RPC, settings toggle, or Orca installation.
- Exact provider tracking ref creation input derived from the verified repo remote name. No local branch materialization is introduced.
- Strict post-create branch/base/path/instance validation. Canonical live paths are /Users/m16khb/Workspace/agent-harness.worktrees/<branch>, and Nest Workspaces must be OFF.
- Exactly-once worktree, terminal, task, and dispatch orchestration with marker/baseline reconciliation.
- Optional RuntimeTerminalCreate.ptyId handling: create validates nonempty handle plus exact worktree identity, then core lists terminals and requires exactly one new connected/writable exact-worktree PTY. A returned PTY ID is cross-checked when present.
- Read-only terminal refresh before later dispatch. A rotated runtime handle is used for dispatch and persisted atomically with the dispatch result.
- Crash continuation after terminal_create or task_create reconciliation without repeating already completed mutations.
- SessionStart claim guidance and PreToolUse native host/session/agent/worktree ownership enforcement only.
- CLI/MCP parity through one MCP action tool, updated IssueOps/Turing skills, response-contract goldens, and cross-host GJC native identity/block propagation.

The unrelated workpool reminder defect remains out of scope. Custom HARNESS_STATE_DIR propagation into a fresh Orca terminal/native hooks is also out of scope and tracked by issue #17; V1 hook-enforced positive evidence uses the default IssueOps state.

## Logical implementation commits

1. 8cb0e13 — docs(issueops): record Orca supervised handoff contract
2. a0a30ef — feat(issueops): add durable supervised execution lease
3. 9c645b1 — feat(orca): add bounded optional cli adapter
4. bee1c80 — feat(issueops): prepare optional Orca worktree exactly once
5. 9aecbae — feat(issueops): dispatch fresh Orca worker with crash fencing
6. 024e79c — feat(issueops): fence supervised worker lifecycle
7. 25d78e5 — feat(hooks): enforce supervised handoff ownership
8. 662dcf5 — fix(gjc): forward native session identity and hook blocks
9. cee06f9 — feat(issueops): expose supervised handoff through cli and mcp
10. 45a02ac — docs(skills): teach optional Orca supervised handoff
11. a9065e9 — fix(issueops): explain Orca flat-layout recovery
12. 7e1de6d — fix(issueops): preserve provider branch in Orca create
13. 83a5b0c — fix(issueops): resume reconciled Orca dispatch safely
14. 1fbbc43 — fix(orca): reconcile optional terminal PTY after create
15. 9306141 — fix(issueops): authenticate supervised worker ownership
16. 406d004 — refactor(issueops): isolate handoff dispatch stages
17. 9344923 — refactor(issueops): isolate handoff prepare stages
18. b4563c8 — fix(hooks): recover cached Codex host identity
19. 2f03461 — fix(hooks): exclude Codex transcript metadata
20. a7bcde8 — fix(hooks): distinguish quoted finish evidence
21. 5fc6ad9 — fix(skills): document prompt-only self-verify gate
22. 785599d — test(hooks): pin supervised handoff verification

## Live verification mistakes promoted to contracts

- Prompt-only self-verify drift RED: the shell intentionally exported `HARNESS_SELF_VERIFY_LLM_EVAL=gate`. A deterministic quick pass completed every step and scored every goal at 100, but the current runtime only rendered the evaluator packet, so `llm_eval.ok=false`, score 0, and the top-level gate remained non-passing. The stale self-verify skill incorrectly said this setting invoked Z.AI. Commit 5fc6ad9 now states that no Z.AI request is sent, pins deterministic completion commands to explicit `--llm-eval=false`, and requires restart from gate 1 after an interrupted or prompt-only attempt. Phrase contracts and all three skill validators passed.
- Focused-package RED: `go test` was first invoked with the plausible but nonexistent `./internal/core/hookinput` path and failed with `stat .../internal/core/hookinput: directory not found` after other packages had begun. The corrected package is `./cmd/harness/hookcli/hookinput`. TESTING, verification operations, self-verify, IssueOps, Turing, and the plan now pin the complete focused command, and a doc contract rejects any executable `go test` line that reintroduces the nonexistent path.
- GJC probe RED: plugin listing, `skills.enabled=true`, and the expected custom skill directory all passed, but a literal `--host gjc` grep returned exit 1 because the TypeScript shim represents the flag and value as adjacent array elements. `scripts/smoke-gjc-native-hook.ts` now imports the selected shim, executes its HookAPI bridge with a native session/cwd, and verifies forwarded `host=gjc`, session, cwd, tool input, enforcement, and the returned block shape. Both repository and installed paths returned `{"ok":true,"host":"gjc","session_id":"gjc-session-1","cwd":"/repo.worktrees/16-demo","blocked":true,"reason":"owned-by-other-session"}`; the Go adapter regression test executes the same smoke.
- Orca enum source proof: an invalid-type probe to a nonexistent terminal returned `invalid_argument` with the exact accepted set `status|dispatch|worker_done|merge_ready|escalation|handoff|decision_gate|heartbeat`. IssueOps and Turing now pin that enum, reserve `status` for progress, `heartbeat` for liveness, `escalation` for the single blocker notice, and `worker_done` for exactly-once completion, and reject invented `progress`, `blocked`, or `completed` types.
- Golden review: tracking this required report raised the repository docs projection from 81 to 82. Regeneration changed only the projected count, the explicit report path in both CLI/MCP docs lists, and the derived self-augment `docs_indexed` value; no command name, MCP tool, schema, or required response field changed.

## ORCA-01 through ORCA-14

| Criterion | Status | Binary observation |
|---|---|---|
| ORCA-01 | PASS | TestWorktreePrepareAutoProbeFailurePreservesLegacyInlineResult observed probe-only trace, legacy inline projection, and absent execution_handoff. |
| ORCA-02 | PASS | TestWorktreePrepareExplicitOrcaProbeFailureHasProbeOnlyTrace returned an error before any mutation. |
| ORCA-03 | PASS | Prepare/dispatch crash-matrix tests counted one worktree, terminal, task, and dispatch at most once; the current default-state live record contains one identity for each. |
| ORCA-04 | PASS | Crash-after-invocation fixtures entered recovery_required, performed zero inline actions, and did not repeat create. |
| ORCA-05 | PASS | Worktree marker, terminal PTY delta, and task marker recovery fixtures accepted exactly one candidate and failed closed on zero/multiple; live optional-PTY reconciliation selected only pty-27 from baseline pty-26. |
| ORCA-06 | PASS | Claim/heartbeat state tests rejected stale attempt, epoch, context, host, session, and root tuples; the default-state worker claimed the exact persisted tuple before editing. |
| ORCA-07 | PASS | Lifecycle tests blocked pre-claim, coordinator, wrong-session, and out-of-tree mutation and allowed the matching claimed worker in-tree. The live worker exposed and then passed hostless identity plus transcript-metadata target checks; quoted evidence is allowed while unquoted shell operators remain blocked. |
| ORCA-08 | PASS | Transition-table tests covered completed/failed finish, submit, accept, cancel, retry, and idempotency actor rules. |
| ORCA-09 | PASS | TestHandoffResumeIsReadOnly and explicit reconciliation tests proved resume caused no state/external mutation and recover persisted only one identity. |
| ORCA-10 | PASS | Legacy missing/zero schema fixtures remained readable and inline; future schema fixtures failed safe. |
| ORCA-11 | PASS | Context tests proved deterministic output, the 64 KiB ceiling, secret redaction, and hash changes for stable source input changes. |
| ORCA-12 | PASS | Installed Codex and Claude hook smokes returned native ownership blocks; the installed GJC shim returned block=true with native identity forwarding. The final Codex full-payload probe included external transcript metadata and allowed only the two expected paths. |
| ORCA-13 | PASS | Worker d4c596d committed exactly two evidence files, submitted the exact fence, and sent worker_done once. Coordinator acceptance closed the handoff; task/dispatch completed and exact PTYs, worktree, local branch, and provider ref were removed. |
| ORCA-14 | PENDING | Final local full/race/build/golden/self-verify transcript remains before this row can pass. |

## Recovery and live-attempt ledger

### Attempt A — nested layout, rejected and removed

The superseded nested-layout receipt above records cycle io-7e43bae11120, instance a8b5abc7-71ef-44ac-879a-6eacc29523ad, pty-21/pty-22, cancelled state, and exact cleanup. No task or dispatch was created. This attempt established the Nest Workspaces OFF prerequisite and retained strict canonical validation.

### Attempt B — flat but suffixed, rejected and removed

- Cycle: io-f287e92389fe.
- Requested branch/path: 16-orca-flat-20260711015255 at /Users/m16khb/Workspace/agent-harness.worktrees/16-orca-flat-20260711015255.
- Returned path: /Users/m16khb/Workspace/agent-harness.worktrees/16-orca-flat-20260711015255-2.
- Worktree ID: 682d2268-d88a-4b01-8728-25e967d26a76::/Users/m16khb/Workspace/agent-harness.worktrees/16-orca-flat-20260711015255-2.
- Instance ID: 619cd1af-7e97-439c-b8a9-f9b13c890f84.
- Terminal: term_453d6882-fc3e-4e1e-a90e-b467fe6f2a77 / ssh:ssh-1783678538888-sl28jq@@pty-23.
- Observation: exactly one worktree create returned the suffix mismatch; IssueOps preserved recovery_required and did not dispatch or retry.
- Cleanup: handoff cancelled; pty-23 stopped/exited; exact worktree removed; local branch absent; provider ref was first verified at the base SHA and then deleted; temp state /Users/m16khb/Workspace/agent-harness-orca-flat-state.zqSQq0 was removed.

### Attempt C — exact custom-state negative test, rejected and removed

- Cycle: io-c8278fca178f.
- Temp state: /Users/m16khb/Workspace/agent-harness-orca-exact-state.jX20DN/state.
- Branch/path: 16-orca-exact-20260711020634 at /Users/m16khb/Workspace/agent-harness.worktrees/16-orca-exact-20260711020634.
- Worktree ID: 682d2268-d88a-4b01-8728-25e967d26a76::/Users/m16khb/Workspace/agent-harness.worktrees/16-orca-exact-20260711020634.
- Instance ID: 553c338a-4d33-4ae7-b011-a0bdb8d7b05d.
- Baseline terminal: term_b6aecb00-fd61-4570-bd1e-2ee35bc05957 / pty-24.
- Worker terminal: term_26804dc0-28c1-4fcc-8d3a-7ec0e6db3657 / pty-25.
- Task/dispatch: task_1ae0ea213951 / ctx_3d204993573c.
- Worker session: Codex session 019f4ef1-7f3d-7eb3-b712-7ab6b292a253.
- Recovery observation: after the initial terminal-create response lacked full row metadata, exact-one pty-25 reconciliation persisted the checkpoint. Resume then skipped terminal creation and created the task/dispatch once.
- Negative hook observation: native default-root PreToolUse reported that branch 16-orca-exact-20260711020634 had no active IssueOps cycle. The temp-root worker was therefore not positive hook-enforcement evidence.
- Worker receipt: msg_9016493565a7 reported custom_state_hook_block and two untracked evidence files; there was no commit or push.
- Cleanup: task result recorded custom_state_hook_block and issue-17 follow-up; both pty-24/pty-25 exited; exact worktree removed and selector returned not_found; local branch absent; provider ref was verified before deletion; only /Users/m16khb/Workspace/agent-harness-orca-exact-state.jX20DN was deleted.

### Attempt D — final default-state positive E2E

- Cycle: io-01cca427c087.
- Branch/path: 16-orca-final-20260711023755 at /Users/m16khb/Workspace/agent-harness.worktrees/16-orca-final-20260711023755.
- Provider tracking ref: refs/remotes/origin/16-orca-final-20260711023755, verified at 2ba240b94477190071598b3f1c7278312b296611 before create.
- Worktree ID: 682d2268-d88a-4b01-8728-25e967d26a76::/Users/m16khb/Workspace/agent-harness.worktrees/16-orca-final-20260711023755.
- Instance ID: 56e30f51-b5e7-43a3-abe5-eebbde21cc8c.
- Runtime: b5b60c39-4aff-4197-90fe-c0a0db1b3253.
- Baseline terminal: term_62a925ce-2c25-4731-b8b7-96dcd5933437 / pty-26.
- Worker terminal: term_d9892f32-fb48-450c-ad8c-fef653fe6ff8 / pty-27.
- Task/dispatch: task_b84cd118fb7e / ctx_01fb15ad9e18.
- Attempt/epoch/context: 1 / 40c14b54c362811651906eaec0438b7c / b2ac173223f675c5502c7c5f0cc5627effc6ffeaa5128d65ab7c9e7ac67f95da.
- Worker session: Codex session 019f4f0b-e5e2-7fe0-9b28-fa27047820a4.
- Creation observation: exactly one confirmed Orca create returned the exact path, refs/heads/16-orca-final-20260711023755, baseRef refs/remotes/origin/16-orca-final-20260711023755, and base HEAD. No local branch existed before create.
- Optional-PTY observation: baseline contained only pty-26; post-create list contained exactly one new connected/writable candidate pty-27. Reconciliation persisted it, and resume did not create a second terminal.
- Native hook observation: SessionStart rendered the exact claim command for io-01cca427c087; claim persisted state=claimed with the native session and exact worktree. Heartbeats advanced last_heartbeat_at.
- Live hook repair sequence: the retained hostless command first failed native identity; b4563c8 made an empty payload/CLI host default to Codex without weakening explicit host or session checks. The next full live event passed identity but exposed top-level transcript metadata as a false external target; 2f03461 ignored only `transcript_path` / `agent_transcript_path` outside `tool_input`. A full-payload probe with both external metadata fields then allowed exactly the two relative evidence targets, and the same worker patch succeeded without bypass or a fresh session.
- Worker result: d4c596dc7413c75fd88d0fb14c313269e4f12913 has parent 2ba240b94477190071598b3f1c7278312b296611, contains only the two authorized evidence files, and left a clean worktree. `handoff finish` persisted `state=submitted`, and worker_done message msg_14b203ec78d420 completed task_b84cd118fb7e / ctx_01fb15ad9e18 exactly once.
- Finish-guard finding: the first finish invocation was rejected because raw separator scanning treated a shell-quoted semicolon in `--verification` as a compound command. The worker safely submitted with punctuation-free prose; a7bcde8 then replaced the raw scan with quote-aware control-operator validation and proved quoted `; & |` allowed while unquoted operators/newlines remain blocked.
- Acceptance receipt: coordinator acceptance returned `state=closed`, `closed_disposition=accepted`, `accepted_at=2026-07-11T03:36:56.611345Z`, and the exact final head.
- Terminal cleanup receipt: exact worktree stop returned `stopped=2`; term_d9892f32-fb48-450c-ad8c-fef653fe6ff8 / pty-27 and term_dca70fdd-1024-4661-b9b7-3e0eed158894 / pty-26 both reached `status=exited`, then read back `connected=false`, `writable=false`.
- Worktree/branch cleanup receipt: exact-ID removal returned `removed=true`, subsequent lookup returned `selector_not_found`, and the path disappeared. Orca intentionally preserved the ahead local branch at d4c596d; its head was verified before exact deletion. The provider ref was independently verified at the base SHA before deletion. Final path, worktree list, local ref, and remote `ls-remote` checks were all empty.

## Native installation and host receipts

- The feature binary was rebuilt and scripts/install-native.sh completed after each live fix. Final installed commit a7bcde8 produced matching build/install SHA-256 `04fc2d4496982081db5f2f85be5b17b2e5a5edfe7457f29073fc238fc90e3b35`.
- Isolated and real native installation smokes completed without making Orca a dependency.
- Codex native payload produced a blocking decision for a non-owner.
- Claude native payload produced the corresponding deny/block result.
- GJC installed hook/shim forwarded native session identity and returned a JSON block result with block=true.
- Installed Codex config continues to emit `--host codex`; live evidence also proved the backward-compatible hostless path because the active session retained its earlier command after file replacement. Installed-file readback was not treated as runtime proof.
- The authoritative Codex probe included external `transcript_path` and `agent_transcript_path`, exact session/cwd, and both relative patch targets. It returned only those two paths, and the real worker event then succeeded.
- The custom-state live attempt is explicitly excluded from positive hook evidence; only the default-state attempt qualifies.

## Verification transcript

Completed before the final documentation pass:

- Focused IssueOps handoff state/context/prepare/dispatch/lifecycle/recovery suites: PASS.
- Orca adapter suite, including optional ptyId and terminal refresh: PASS.
- Lifecycle guard and hook-input suites: PASS.
- CLI/MCP parity and response-contract golden updates: PASS.
- go test ./... -count=1 after commit 1fbbc43: PASS.
- scripts/install-native.sh after commit 1fbbc43: PASS.

Final fresh commands and exact outcomes: PENDING.

## AI-slop-clean

- Pre-clean measurement: 77 changed files / 52 Go-or-TypeScript files; approximate added-line SNR was 5,106 signal / 7 comment-only noise candidates = 0.998631. `gocyclo` reported `StartIssueOpsHandoff` at 43 and `PrepareIssueOpsHandoffWorktree` at 37.
- Cleanup diff: 406d004 changed only `issueops_handoff_dispatch.go` (120 insertions, 98 deletions) to expose terminal, task, and dispatch durability stages. 9344923 changed only `issueops_handoff_prepare.go` (99 insertions, 75 deletions) to expose mode, journal begin, external create, and persistence stages. Neither commit changed external call order or recovery fences.
- Post-clean measurement after all review fixes: 6,011 signal / 8 comment-only noise candidates across 6,019 added Go-or-TypeScript lines = 0.998671. `golangci-lint --enable-only=gocyclo,dupl --new-from-rev 2ba240b... --tests=false ./...` returned `0 issues`.
- Manual scan found no added TODO/FIXME/placeholder/future-proof/AI-generated markers. Nine added source comments were reviewed; one redundant two-line control-operator comment was tightened to one contract-bearing line. Five Markdown trailing-space findings in the plan/design headers were removed.
- The exact cleanup remains limited to feature-introduced files and prose; the unrelated workpool reminder defect and custom-state propagation issue #17 were not touched.

## Remaining coordinator-only actions

No push, PR, merge, implementation-handoff acceptance, or implementation worktree/branch cleanup is performed by this worker. The disposable live E2E was accepted and cleaned as recorded above; the parent coordinator still owns the issue branch handoff, push, PR, merge, and final implementation worktree cleanup after receiving worker_done.
