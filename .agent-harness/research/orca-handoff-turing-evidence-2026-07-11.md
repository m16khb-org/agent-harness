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
- Focused-package RED: `go test` was first invoked with the plausible but nonexistent `./internal/core/hookinput` path and failed with `stat .../internal/core/hookinput: directory not found` after other packages had begun. The corrected package is `./cmd/harness/hookcli/hookinput`. TESTING, verification operations, IssueOps, Turing, and the plan now pin the complete focused command, and a doc contract rejects any executable `go test` line that reintroduces the nonexistent path. The generic self-verify skill intentionally contains no Orca/GJC-specific recipe.
- GJC probe RED: plugin listing, `skills.enabled=true`, and the expected custom skill directory all passed, but a literal `--host gjc` grep returned exit 1 because the TypeScript shim represents the flag and value as adjacent array elements. `scripts/smoke-gjc-native-hook.ts` now imports the selected shim, executes its HookAPI bridge with a native session/cwd, and verifies forwarded `host=gjc`, session, cwd, tool input, enforcement, and the returned block shape. Both repository and installed paths returned `{"ok":true,"host":"gjc","session_id":"gjc-session-1","cwd":"/repo.worktrees/16-demo","blocked":true,"reason":"owned-by-other-session"}`; the Go adapter regression test executes the same smoke.
- Orca enum source proof: an invalid-type probe to a nonexistent terminal returned `invalid_argument` with the exact accepted set `status|dispatch|worker_done|merge_ready|escalation|handoff|decision_gate|heartbeat`. IssueOps and Turing now pin that enum, reserve `status` for progress, `heartbeat` for liveness, `escalation` for the single blocker notice, and `worker_done` for exactly-once completion, and reject invented `progress`, `blocked`, or `completed` types.
- Golden review: tracking this required report produced exactly six semantic changes. The four numeric changes are CLI `docs_index.docs_count` 81→82, MCP `docs_index.docs_count` 81→82, CLI self-augment `repo_signals.docs_indexed` 81→82, and MCP self-augment `repo_signals.docs_indexed` 81→82. The other two changes insert `$HARNESS_ROOT/.agent-harness/research/orca-handoff-turing-evidence-2026-07-11.md` once in the CLI required-docs projection and once in its MCP mirror. No command name, MCP tool, schema, or required response field changed.
- Accepted-retry state-machine RED: the design transition table permits retry only from `closed/worker_failed`, `closed/cancelled`, or an explicitly abandoned disposition that V1 does not implement, but `retryIssueOpsHandoff` checked only `StateClosed`. `TestHandoffRetryRejectsAcceptedDisposition` demonstrated that `closed/accepted` was reopened as attempt 2. GREEN now requires `DispositionWorkerFailed` or `DispositionCancelled` before replacing the persisted attempt/epoch; accepted retry returns an error and preserves `state=closed`, `closed_disposition=accepted`, `attempt=1`, and the original ownership epoch. The existing cancelled path and a new worker_failed path both still create attempt 2.
- The in-progress full self-verify had completed six iterations when this finding arrived. It was interrupted and all partial results were discarded before the state-machine edit; final verification restarts from gate 1 after this correction is committed.

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

## Corrective checkpoint after live and independent audits

- Zsh equals expansion RED: `=git push ...` and `=(...)` passed the claimed-worker guard. `HasActiveZshEqualsExpansion` now detects only unquoted, unescaped word-leading equals expansion; quoted literals and `NAME=value` remain allowed. Full commandparse and lifecycle packages passed.
- Durable secret RED: the original synthetic fixture accidentally matched the generic secret-path heuristic, so it did not prove Bearer-header handling. Independent `Authorization: Bearer [REDACTED]` and `api_key=[REDACTED]` assignments reproduced the two leak classes without retaining raw values in evidence. The common policy now recognizes the Bearer header directly; state return, raw DB bytes, and recovery projection tests passed for each independent form.
- Result evidence RED: completed finish accepted an absolute Turing report until late generic envelope failure, and accept followed a committed leaf symlink through `EvalSymlinks`/`Stat`. Finish/state now require a canonical relative report, accept uses `Lstat` before resolution, and both focused regressions passed. Failure.Message is explicitly optional-but-if-present bounded/redacted, while the common bounded-list validator rejects raw secret-bearing items.
- Authorization cleanup: the old `supervisedHandoffRecord`, loose lifecycle parser, and flag helpers had zero production callers after the exact authority path landed. The empty-agent flag regression moved to `exactFlags`; the dead alternate authorization path was removed, and the full lifecycle suite passed.
- Skill RED→GREEN: the pre-existing fresh-agent baseline in `.agent-harness/research/orca-skill-baseline-2026-07-11.md` remains the no-guidance RED. IssueOps, Turing, and self-verify contract tests were written before edits. IssueOps forward pressure `/root/issueops_skill_pressure` rejected source-checkout tests, preserved literal terminal text, required standalone FinalHead/ref observation plus exact draft flags, and refused ambiguous create retry. Turing pressure `/root/turing_skill_pressure` produced the same ownership/evidence decisions. Self-verify pressure `/root/self_verify_skill_pressure` kept generic deterministic and prompt-only llm-eval semantics and refused to invent Orca/GJC recipes.
- Skill validation: `python3 scripts/validate-skill.py skills/issueops`, `skills/turing`, and `skills/self-verify` each returned `Skill is valid!`; focused skill-contract tests passed. The initially attempted `openai-bundled` writing-skills cache path was wrong; the verified 689-line source was `/Users/m16khb/.codex/plugins/cache/openai-curated-remote/superpowers/6.1.1/skills/writing-skills/SKILL.md`. That runtime path correction is execution evidence only and is not hard-coded into product code or skill guidance.
- MCP contract golden review: the earlier catalog delta is exactly 45 added schema lines for ten `issueops_handoff` inputs already implemented by the shared CLI/MCP DTO (`criteria_ids`, `force`, `heartbeat_cadence`, `reason`, `required_docs`, `required_skills`, `result_format`, `stop_conditions`, `verification_commands`, and `worker_scope`). This corrective checkpoint adds exactly one four-line property, `allow_codex_hook_trust_bypass` (description plus boolean type), and no tool, required-field, or unrelated schema entry. The focused contract-golden and response-contract-golden suites passed after review.
- First final gate-1 RED: the generic doc-upkeep classifier skipped separate-value `git -C` / `go -C` global directory flags and combined shell flags such as `sh -ec`; the full suite also exposed a stale self-verify contract assertion that still demanded the host-specific hook recipe removed from that generic skill. Directory-option parsing and combined `-c` detection now classify those representative mutations, while the self-verify contract forbids those recipe fragments and the TESTING/operations contract continues to require them in the correct locations. Both complete affected packages passed before gate 1 was restarted.
- Final-live plan-truth RED: cycle io-35b9a814c8aa was correctly provisioned at the exact flat path, but the coordinator linked an unrelated 2026-05-31 legacy plan before dispatch. Dispatch was stopped. New regressions prove a clean plan-only coordinator commit advances `attempt_base_head`, while a dirty plan, mixed commit, unsafe coordinator Git command, or unrelated replacement path fails closed. IssueOps was updated and validated first, then Turing was given the same current-cycle intent/acceptance/exact-bounded-scope recipe and validated separately. A follow-up RED removed the accidental universal `report-only` generalization; report-only remains only this disposable cycle's declared scope. The main source coordinator then committed and linked `.agent-harness/plans/io-35b9a814c8aa-final-live-e2e.md`; the clean plan HEAD and persisted attempt base are both `d8cb072add80c605e1138d38e9bc4e7e2e894b4c`, and the declared worker result is only `.agent-harness/research/orca-final-live-worker-evidence-2026-07-11.md`.
- Wrong-root terminal-steering incident: this feature-worktree session first received the correct PreToolUse block when it tried to edit the live child plan. It then incorrectly injected `apply_patch` through bare terminal pty-28 (`term_78f27ec5-9266-42e8-a199-d0118ea9c87c`), bypassing the target hook surface and creating untracked orphan `.agent-harness/plans/io-35b9a814c8aa-orca-final-live-e2e.md`. The main source coordinator removed the orphan before any commit. RED showed non-source `stop/create/switch/focus/close/rename/split` controls and a wrong `term_other` handle were accepted; GREEN classifies every installed control command plus write/input/type/paste aliases, retains only `list/show/read/wait` as observation, and binds the sole claimed source-coordinator `send` exception to a uniquely matching persisted worker terminal handle and a single-line `# agent-harness guidance:` payload. Sentinel assertions prove the block occurs at the source command; the target hook is not assumed to catch injected shell text. A real `.git` directory regression proves repeated discovery of one stable cycle ID is deduplicated. A second RED then reproduced the concurrent-cycle false ambiguity: two distinct handles now select the exact claimed record, while an unknown handle or the same persisted handle on two records fails closed as ambiguous. A final PTY-byte RED showed decoded backspace, tab, ESC, and DEL all passed; GREEN rejects every ASCII C0 rune and DEL while retaining ordinary Korean/Unicode guidance.
- Closed-attempt terminal-cleanup RED: after cycle `io-35b9a814c8aa` attempt 1 was durably `closed/cancelled`, task `task_c9f3866d42b7` was marked failed with `claim_observed=false`, and exact worker terminal `term_fe14fab3-a819-489b-8389-ce41082758fa` was closed, source review showed a fresh hook would have blocked that exact close before reaching the existing cleanup allowlist. GREEN moves only the exact source-root `orca terminal close --terminal <persisted-worker-mailbox-handle> --json` form ahead of the general terminal-control block for `closed/worker_failed` and `closed/cancelled`. Wrong handle, accepted/active state, worker/non-source identity, extra flags, stop, and create remain blocked; the close remains optional and terminal inventory, not the mailbox handle, supplies final cleanup evidence. Baseline pty-28 and the exact flat worktree remain intentionally preserved for the next recovery decision.
- Codex hook-trust live RED and bounded correction: final-cycle terminal `term_fe14fab3-a819-489b-8389-ce41082758fa` / pty-29 stopped at `Hooks need review` before SessionStart, claim, or mutation; task `task_c9f3866d42b7` / dispatch `ctx_fa0683b4083c` therefore remained unclaimed until attempt 1 was cancelled as recorded above. A supported read-only `codex app-server --stdio` `hooks/list` call for the exact worker cwd returned 13 enabled hooks, `warnings=[]`, `errors=[]`, and exactly one modified entry: agent-harness PreToolUse key `/Users/m16khb/.codex/hooks.json:pre_tool_use:1:0`, current hash `sha256:bee484dbb54d6ec9a5f19207b8245f16d5bd92452ec801d08bc8a9a0934bae08`; all other Orca and agent-harness hooks were trusted. The attempted automatic verifier implementation was discarded uncommitted as out of scope for #16. RED→GREEN instead requires an explicit per-attempt start attestation, exposes required/attested state in preview, rejects unattested confirmed Codex start with zero terminal/task/dispatch calls, probes installed support for `--dangerously-bypass-hook-trust`, and passes that launch flag only to attested Codex. A second attested no-confirm preview supplies the reviewed context hash; the otherwise identical confirm must return the same hash. Retry preserves every other sealed option but clears the attestation; the additive field deliberately remains ContextVersion 1 so the live version-1 closed attempt stays readable.
- Orchestration message-type incident: the coordinator attempted `--type progress` even though skills already listed the installed enum. A focused hook RED now blocks an explicit invalid or duplicate type before supervised-record selection and lists `status`, `dispatch`, `worker_done`, `merge_ready`, `escalation`, `handoff`, `decision_gate`, and `heartbeat`; valid or omitted type falls through without gaining new authority.
- Mailbox injection incident: `orca orchestration check --unread --inject` on refreshed handle `term_2130` injected 25 historical messages from prior tasks before the latest escalation. Source review then confirmed check defaults to unread and `--all --inject` can inject even more history. RED covered implicit-default inject, unread/inject in both orders, all/inject, and equals form; GREEN blocks any explicit `--inject` on a direct check before record selection while leaving `check --all --json` and unrelated observations unchanged. The executable recipe selects the exact current task/dispatch/sequence and never equates a live terminal handle with historical mailbox identity. Urgent correction remains restricted to the existing literal-safe source-coordinator send bound to the uniquely persisted worker handle; automatic synchronization stays in issue #17.
- Linked-worker decision-gate incident: after reporting installed HEAD `9a34d3b`, this legacy linked worktree issued two direct `orca orchestration ask` commands even though it has no `execution_handoff`; both connections closed without a response. Immediate `orca status --json` showed the same ready runtime, and `orca orchestration gate-list --json` returned `gates=[]`, `count=0`, proving no gate existed. RED reproduced that both direct ask and gate-create were allowed from the linked checkout. GREEN resolves the public `.git` worktree pointer without Git, blocks both worker mutations before record selection, preserves source-coordinator ask/gate-create, and leaves worker `gate-list` read-only. After reinstalling `e10db02`, a full Codex-shaped installed-hook payload returned `decision=block` for direct ask with linked-worker guidance, while the same payload with `gate-list --json` returned the allow/no-op object.
- Self-verify redaction restart: the pre-fix seed-100 run stopped at the redaction audit because evidence retained a synthetic raw assignment value. After replacing both synthetic values with `[REDACTED]`, the exact fresh command `./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json` restarted at iteration 1 and passed 24/24 steps, minimum goal score 100, termination eligible true, with `redaction audit` PASS.
- Targeted-test receipt correction: an initial stale `-run` regex returned `ok ... [no tests to run]`, which is not GREEN. The IssueOps/Turing skill contract now requires `-v`, named `=== RUN` evidence, and rejection of `[no tests to run]`; the exact current command ran `TestHandoffGuardEnforcesInstalledOrchestrationMessageTypes` and `TestHandoffGuardBlocksExplicitHistoricalMailboxInjection` (including all five injection subcases), and every named test reported PASS.
- Committed-HEAD gate restart caught the same failure class in the stale `go test ./cmd/harness -run Golden` recipe: it returned `[no tests to run]` and was not counted as a gate. The current AGENTS and Orca plan now target `./cmd/harness/contractgolden`; that package and `./cmd/harness/harnessapp -run TestResponseContractsGolden` both ran real tests and passed before gate 1 restarted.
- Focused checkpoint verification: `go test ./internal/core/lifecycle -count=1`, `go test ./internal/core/issueops/handoff -count=1`, and `go test ./internal/core/issueops -count=1` all passed after the corrections.

## Final post-hardening live retry receipt

- Report-only verification scope: the sealed attempt-2 packet declared only `git diff --check` and `git status --short`. A focused IssueOps/Turing skill RED now rejects invented API, provider-ref, or history probes for report-only cycles; GREEN preserves the exact bounded scope and both skill validators pass.
- Submitted completion RED: live cycle `io-35b9a814c8aa` reached `submitted`, but the installed ownership hook treated the exact worker as unclaimed and blocked its otherwise complete `worker_done` argv. `TestHandoffGuardAllowsOnlyExactSubmittedWorkerDoneAndRetryGuidance` now proves that only the persisted Codex host/session/agent, exact task `task_5f2ec50db0f9`, exact dispatch `ctx_750942dcb75f`, concrete coordinator handle, nonempty subject, three-sentence body, one exact persisted changed file, and its absolute in-worker report path are allowed. Wrong host/session/agent/task/dispatch, group target, extra file, external report, duplicate flag, wrong message type, short body, and extra payload all remain blocked; every other submitted repository mutation remains blocked.
- Retry guidance boundary: the exact source coordinator may send one literal-safe guidance line only to persisted submitted worker handle `term_0932bcea-43e4-4b8c-8095-ae684be17842`; a wrong handle remains blocked. Commit `b0629a4` was built and installed, and source/install SHA-256 both read `dc2cd8e838c05a18266f124214d28f62d1b845d4708b22ea2a91c3b433c6db71`.
- Installed-hook proof: a full Codex PreToolUse payload for session `019f50b1-1ca0-7042-8a34-780716e31457`, canonical worker root, exact task/dispatch, exact relative changed file, and exact absolute report path returned the allow/no-op object. Changing only the task to `task_wrong` returned `decision=block`.
- Live completion: after exact-handle guidance, the same submitted worker retried once. `worker_done` message `msg_dda9f130282f7` completed task `task_5f2ec50db0f9`; the task row is `status=completed`, and IssueOps attempt 2 is durably `state=closed`, `closed_disposition=accepted`, with `accepted_at=2026-07-11T10:43:02.876401Z` and final head `ccdeaa72c36776ba5992b6babd81b7bc8a99e79c`.
- Cleanup: exact worktree removal disconnected baseline `ssh:ssh-1783678538888-sl28jq@@pty-28` and worker `ssh:ssh-1783678538888-sl28jq@@pty-30`; the complete terminal inventory contains neither PTY. The Orca repo inventory contains only the main and issue-16 implementation worktrees, `stat` reports the disposable path absent, local `refs/heads/16-orca-final-20260711165934` is absent, and `git ls-remote --heads origin refs/heads/16-orca-final-20260711165934` returns an empty result.
- Checkpoint verification before the final gate restart: the focused submitted-worker regression, both skill-contract regressions, both skill validators, complete `lifecycle`/`issueops`/`hookcli` packages, build, native install, installed-hook positive/negative probes, and `go test ./... -count=1` all passed. Race, exact golden, and ten-iteration self-verify are rerun from this evidence commit before completion.

## Runtime-rollover recovery checkpoint

- Live restart observation: Orca runtime changed from `b5b60c39-4aff-4197-90fe-c0a0db1b3253` to `2e36e33e-cf56-43aa-8c19-8136b86c9498`. The same logical worker was reissued from `term_13923673` / `pty-31` to `term_f60ac104-0dc7-4899-8b38-e8428bcc4d8a` / `pty-6`; the sealed dispatch still named the stale handle.
- Stable-layout observation: direct terminal title became the dynamic `16-orca-supervised-ha...`, while tab `1e2dd098...`, leaf `2845646c...`, and the joined visual-layout custom title `issue-16-review-hardening` remained stable. The installed-shape fixture retains the full identifiers and layout nesting rather than the abbreviated evidence display.
- Worktree observation: the exact current-runtime row carried instance `fbb13645-16bb-48e2-8820-4c576aa7067d`. Runtime recovery now requires a complete unique repo/base/path/branch/HEAD/comment-marker match and atomically refreshes runtime, worktree instance, handle, PTY, tab, and leaf. A nonempty instance equal to the persisted value is also valid; missing, terminal/worktree mismatch, or conflicting duplicate evidence is not.
- Replacement safety: a replacement terminal observed the dirty preserved WIP and was aborted without edits. Core regressions keep any recovered connected/writable terminal or uncommitted worker root authoritative and make no duplicate terminal create.
- Capability observation: installed `orca terminal create --help` currently exposes the fixed `--command` shape and not `--agent`. The adapter rechecks help immediately before mutation, accepts only a harness-generated fixed host command or verified built-in agent form, and preserves a provisioned lease as `recovery_required` if capability disappears before create.
- Relay observation: environment key inspection confirmed `ORCA_RELAY_DIR` and `ORCA_RELAY_SOCKET_PATH` without printing values. A handshake through stale pinned relay state is only a local transport observation and never sufficient to adopt runtime, worktree, or terminal identity.
- Observation-cancellation correction: the canonical recipe uses bounded terminal reads and caller-side Ctrl-C or host tool cancellation. It does not mutate the target PTY to stop a handshake-only observation.
- Mailbox-envelope correction: the first read-only projection used `jq '.messages[]?'` and silently returned no rows. The raw installed response showed five messages nested at `.result.messages`; `orca orchestration check --all --json | jq '.result.messages[]'` is now pinned only in the canonical IssueOps reference, while Turing links that reference and retains the mailbox-observation receipt.
- Runtime-refresh CAS RED→GREEN: before the fix, a durable row mutation during inventory was ignored until a later dispatch mismatch, while context-source and dirty-checkout drift allowed the new tuple to be adopted. The runtime-only locked completion now exact-compares the journal snapshot, revalidates context source and clean exact branch/HEAD, and leaves all three injected drift rows in `recovery_required` with the old runtime/instance/handle/PTY/tab/leaf, pending `runtime_refresh`, and zero later mutation.
- Focused receipts: adapter rollover/capability/total-layout-bound fixtures, core runtime-refresh/CAS/cancellation/no-duplicate tests, schema v1→v2 and v2→v3 compatibility probes, prior-attempt stable terminal identity, force-abandon rejection of read-only runtime refresh, and IssueOps/Turing ownership contract tests all report named PASS results in this bundle's verification transcript.

## GitLab supervised handoff checkpoint

- Installed contract observation: runtime `0.1.0+540e30aed735` exposes worktree-create `--issue` as a linked GitHub issue number and exposes no GitLab issue option in public CLI help. Installed worktree rows expose nullable `linkedGitLabIssue`; the fixture now retains explicit `null` rather than relying on a missing field. This help receipt does not claim exhaustive public RPC-schema absence.
- RED→GREEN: GitLab `auto` and explicit Orca use the exact verified provider tracking ref while omitting GitHub `--issue`; null/zero native metadata yields `orca_gitlab_native_metadata_unavailable` in JSON and human warning output, exact metadata clears it, and conflicting GitHub/mismatched GitLab identities fail closed. The bounded observation persists in the Orca identity and remains exact after disk read/reprojection and runtime refresh.
- Compatibility correction: GitLab `auto` with missing, unready, capability-failed, or errored Orca returns the unchanged inline contract without a warning or `execution_handoff`. A later post-probe inline fallback also removes the warning. A nil legacy `BranchPrepare` remains panic-free and preserves its original inline command/base projection.
- Context/provider boundary: provider and exact Issue URL participate in the sealed source fingerprint. No test or production path creates or mutates a GitLab remote object.

## Verification-wrapper correction

- Observed incident: `go test ./internal/core/issueops -count=1` emitted `ok ... 52.760s`; the following zsh bookkeeping assignment failed with `read-only variable: status`. This was a wrapper failure, not a Go test failure.
- Search-pattern incident: a later `rg` pattern placed Markdown backticks inside double quotes, so zsh executed the backticked `status` word and emitted `command not found: status`. Turing now requires a single-quoted pattern or literal argv.
- Mailbox-filter incident: a read compared opaque `msg_*` IDs lexicographically to approximate recency. The canonical IssueOps recipe now selects numeric `sequence` with exact `taskId`/`dispatchId` and handle direction from `.result.messages`; opaque IDs are never ordering evidence.
- Fresh verdict: the same package was rerun via `exec go test ./internal/core/issueops -count=1` and exited 0 with `ok ... 51.280s`.
- Retained regressions: Turing requires `rc` or `exit_code` instead of zsh's reserved `status`, separates the test verdict from wrapper bookkeeping, and quotes backtick-bearing searches literally. IssueOps contract tests pin numeric mailbox selection and forbid chronological inference from opaque message IDs.

## Remaining coordinator-only actions

No push, PR, merge, implementation-handoff acceptance, or implementation worktree/branch cleanup is performed by this worker. The disposable live E2E was accepted and cleaned as recorded above; the parent coordinator still owns the issue branch handoff, push, PR, merge, and final implementation worktree cleanup after receiving worker_done.
