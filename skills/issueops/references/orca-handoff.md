# Optional Orca Supervised Handoff

Choose the worktree mode after the IssueOps issue is linked, the provider branch is verified, and the design review is approved. After the resulting worktree exists, link the plan there and complete the normal compatibility, worktree-tool, execution-decision, and devil's-advocate gates before dispatch. IssueOps remains the single durable authority. Orca is an optional process/worktree driver and never owns phase state, evidence acceptance, PR creation, or cleanup decisions.

## Choose The Mode

Preview before mutation:

```bash
agent-harness issueops worktree prepare --id "$ISSUEOPS_ID" --orchestrator auto --agent codex --json
```

The three modes are intentionally small:

- `--orchestrator auto`: probe Orca. If the probe fails before any mutation, return the unchanged inline result and leave `execution_handoff` absent. If Orca is ready, preview or prepare the supervised worktree.
- `--orchestrator orca`: require a ready Orca installation and fail before mutation when the probe fails.
- `--orchestrator inline`: use the legacy sibling-worktree flow and leave `execution_handoff` absent.

Before confirming Orca mode, turn **Settings > General > Workspace > Nest Workspaces** off. IssueOps requires the flat canonical path `<repo>.worktrees/<branch>` and does not read private Orca settings, toggle global layout preferences, or accept the nested `<repo>.worktrees/<repo-name>/<branch>` form. The returned worktree path is validated after the exactly-once create call. A mismatch moves the handoff to `recovery_required` with an actionable diagnostic; it never adopts the mismatched path or falls back inline. Cancel the handoff, remove the mismatched resources, and start a fresh IssueOps cycle after correcting the setting.

The Orca repo projection must expose `gitRemoteIdentity.remoteName`. The confirmed create uses `refs/remotes/<remote>/<branch>` as its base so an already-linked provider branch keeps the exact branch and directory name without pre-creating or checking out a local branch. A missing remote name is a pre-mutation probe failure.

GitHub and GitLab both use that exact verified provider ref. GitHub worktree creation additionally uses Orca's public GitHub-only `--issue` flag. GitLab omits `--issue` and never invents a GitLab flag or creates a remote provider object. A returned nonzero GitHub link or mismatched nonzero GitLab link is an identity failure. Null/zero `linkedGitLabIssue` is accepted only with `orca_gitlab_native_metadata_unavailable`; JSON includes it in `warnings` and human output prints `warning: orca_gitlab_native_metadata_unavailable`. Exact native metadata clears it. The observation is durable, so a restart or repeated prepare reports the same truth.

Do not attach that warning to an inline fallback. In GitLab `auto`, missing/unready/capability-failed Orca and every later definitive pre-mutation fallback preserve the legacy inline response with no Orca warning or handoff state. Only a successfully resolved Orca preview or confirmation may carry the warning. The sealed context must show the exact verified provider and Issue URL before confirmation.

Do not infer Orca readiness from `orca version`; the adapter uses structured `orca status --json`. Run the confirmed command only after reviewing the preview:

```bash
agent-harness issueops worktree prepare \
  --id "$ISSUEOPS_ID" \
  --orchestrator auto \
  --agent codex \
  --confirm \
  --json
```

If `resolved_mode` is `inline`, continue the existing in-session bind, heartbeat, TDD, verification, and PR-readiness flow. Do not synthesize an `execution_handoff` record for inline fallback.

## Current Cycle Plan Checkpoint

Before dispatch, the supervised plan must state the current issue and cycle intent and its acceptance criteria directly. It must include the exact branch, canonical worker path, attempt base SHA, exact bounded worker scope (report-only only when that is the current cycle's declared scope), exact claim/finish/accept commands, verification commands, and coordinator cleanup order. Never link an unrelated legacy plan merely to satisfy the readiness gate.

Create the plan under the linked worktree's approved plan convention while the handoff is still `coordinator_preparing`, the context is unsealed, and no external-operation journal is pending. Preserve it as one coordinator plan commit containing only that Markdown file:

Plan file edits must originate from the source coordinator root: both the hook CWD and repo identity must exactly equal `record.Repo`. A feature-worktree session must not steer a child plan; hand the edit and commit to the main source coordinator instead of relaying a shell mutation into the child terminal.

```bash
git -C <worker-root> add -- <absolute-current-cycle-plan-path>
git -C <worker-root> commit --only -m 'docs: record current cycle handoff plan' -- <absolute-current-cycle-plan-path>
agent-harness issueops link-plan --id <cycle-id> --plan-path <absolute-current-cycle-plan-path> --json
git -C <worker-root> status --short
git -C <worker-root> rev-parse --verify HEAD
```

`link-plan` requires the exact branch to be clean, requires the new commit to descend from the previous checkpoint, and requires its committed diff to contain only the current-cycle plan. It then moves the persisted attempt base head to that coordinator plan commit. The worker therefore starts from the plan commit, and its submitted `changed_files` contains only worker-owned result files rather than the coordinator plan.

## Coordinator Dispatch

For `resolved_mode: orca`, the coordinator reviews readiness and creates the bounded context packet. Repeat flags are allowed where shown:

Before any replacement or dispatch, inspect exact-worktree terminals and active orchestration tasks. Any connected/writable possible writer or dispatched task blocks another writer, even when preserved WIP or the diff appears stable. A stable diff is not ownership evidence. Do not adopt WIP until the original task is terminal and the original writer is exited or closed.

After any targeted terminal close, reread the complete exact-worktree terminal inventory before dispatch and resolve the fresh connected/writable live handle again. Closing one pane may roll over or exit a sibling; a pre-close inventory is never post-close sole-writer evidence.

Use server-filtered task inventory for sole-writer attestation, then inspect the exact current dispatch:

```bash
orca orchestration task-list --status dispatched --json
orca orchestration dispatch-show --task <current-task-id> --json
```

`orca orchestration task show`, `orca orchestration dispatch show`, and status `in_progress` are invalid; use only the exact inventory forms above. Do not infer task absence from a local filter over broad output. For this fence, truncated or unparsable JSON is ambiguity, never absence; rerun the server-filtered observation and keep mutation blocked until the exact task and dispatch are proven.

Start the fresh worker from a login shell and require the actual host banner. Immediately before dispatch, obtain a fresh `connected=true` and `writable=true` check for the exact terminal. One `tui-idle` sample alone is insufficient. After an authorized terminal send delivers interrupt text plus Enter, read the target and verify that UserPromptSubmit or working state actually began. If the full instruction remains at the idle prompt, send exactly one Enter and read again. Never resend the instruction body.

Codex supervised startup uses an explicit per-attempt attestation; Claude and GJC do not. First run `codex --help` as a standalone observation and require `--dangerously-bypass-hook-trust`. Then open the supported read-only catalog with `codex app-server --stdio`, send these JSONL messages one at a time, and keep stdin open until response id 2 arrives:

```text
{"method":"initialize","id":1,"params":{"clientInfo":{"name":"agent_harness","title":"agent-harness","version":"1"}}}
{"method":"initialized","params":{}}
{"method":"hooks/list","id":2,"params":{"cwds":["<exact-worker-cwd>"]}}
```

Do not call `config/batchWrite` or trust hooks from this recipe. Attest only when the response has exactly the requested cwd, warnings and errors are both empty, required SessionStart and PreToolUse command hooks are enabled, and every untrusted or modified entry is the exact generated agent-harness hook command for the current installed binary. Any unrelated untrusted/modified entry, missing ownership hook, cwd mismatch, or malformed response fails closed. Record the reviewed keys, `currentHash` values, source paths, and binary target in evidence without copying secrets. Explicit nonsecret Orca environment-key allowlist: never dump broad ORCA-prefixed env output or use prefix filtering for identity probes. Allow only explicitly named nonsecret keys such as `ORCA_TERMINAL_HANDLE`, `ORCA_TAB_ID`, and `ORCA_WORKTREE_ID`, and never record secret values in tests, docs, logs, or evidence.

Preview `handoff start` without confirmation first. For Codex it must report `codex_hook_trust_bypass_required=true` and `codex_hook_trust_bypass_attested=false`. After the review above, run a second no-confirm preview with every delivery option plus `--allow-codex-hook-trust-bypass`; require `codex_hook_trust_bypass_attested=true` and record the reviewed context hash from that final attested preview. Preview returns `context_sha256`. Final confirmed start must add `--expected-context-sha256` with the exact final attested preview hash plus `--confirm`; all delivery options remain identical. The confirmed start recomputes the sealed context and fails closed before any terminal, task, dispatch, or journal mutation when the submitted hash is missing, malformed, or differs from the freshly recomputed sealed context — including post-preview source drift and delivery-option drift. Confirm must never introduce a delivery option absent from that final preview. The adapter then launches only that Codex worker with `codex --dangerously-bypass-hook-trust`. Omit the flag for Claude, GJC, inline mode, and any unreviewed Codex attempt. This optional additive field remains ContextVersion 1 for legacy retry compatibility. Retry preserves the sealed delivery options but clears the per-attempt attestation, so the coordinator must perform and attest the review again. General automatic trust probing remains issue #17.

Every preview and confirm supplies `--coordinator-recipient` with the same concrete historical coordinator mailbox handle. Confirmation persists that recipient with the sealed context before terminal/task/dispatch mutation and passes it as Orca dispatch `--from`; refreshable current coordinator control identity never replaces it. The returned preamble is accepted only when it contains the official exact coordinator and task label lines (`Your coordinator's terminal handle is:` and `Your task ID is:`) plus the exact `--dispatch-id` token; arbitrary substring matches fail closed.

```bash
agent-harness issueops handoff start \
  --id "$ISSUEOPS_ID" \
  --coordinator-recipient "$COORDINATOR_MAILBOX_HANDLE" \
  --criteria-id ORCA-01 \
  --criteria-id ORCA-02 \
  --criteria-id ORCA-03 \
  --criteria-id ORCA-04 \
  --criteria-id ORCA-05 \
  --criteria-id ORCA-06 \
  --criteria-id ORCA-07 \
  --criteria-id ORCA-08 \
  --criteria-id ORCA-09 \
  --criteria-id ORCA-10 \
  --criteria-id ORCA-11 \
  --criteria-id ORCA-12 \
  --criteria-id ORCA-13 \
  --criteria-id ORCA-14 \
  --required-doc "$PLAN_PATH" \
  --required-skill superpowers:test-driven-development \
  --required-skill superpowers:verification-before-completion \
  --worker-scope "$WORKER_SCOPE" \
  --verification "go test ./... -count=1" \
  --heartbeat-cadence "every 5 minutes and at task boundaries" \
  --stop-condition "do not push, open or merge a PR, or accept the handoff" \
  --result-format "final head, changed files, Turing report, verification, and cleanup receipts" \
  --allow-codex-hook-trust-bypass \
  --expected-context-sha256 "$REVIEWED_CONTEXT_SHA256" \
  --confirm \
  --json
```

The returned `attempt`, `ownership_epoch`, `context_sha256`, sealed coordinator mailbox recipient, Orca worktree id, task id, and dispatch id are a single bounded worker context. The mailbox numeric `sequence` is message selection evidence, not part of the lease fence. The coordinator passes that tuple to the fresh worker without copying credentials, conversation transcripts, or unbounded environment data.

## Worker Lease

The fresh worker starts in the exact Orca worktree and claims before any mutation:

The source implementation checkout is read-only; the source checkout is observation-only. From it, use only explicit non-executing observations such as `git status`, `git diff`, `git log`, `git show`, `git rev-parse`, `git ls-files`, and `rg`. Tests, builds, formatting, installation, and generation run only in the claimed worker root; test initialization, fixtures, caches, binaries, and goldens can mutate state. This includes `go build -o`, every test runner, native install, formatting, golden updates, and generators. If PreToolUse blocks an operation, do not bypass the hook through a different tool. The eval and source primitives are shell reinterpretation and are forbidden wrappers.

For a report-only cycle, run only the verification commands declared in the sealed worker packet. Do not invent API, provider-ref, or history probes; the bounded report is not authority to widen verification or inspect unrelated external state. If a declared command cannot run, record the exact failure instead of substituting a new probe.

Use the installed `agent-harness` command in a fresh worker unless the bounded context proves `./bin/agent-harness` exists in that exact worker checkout. For workers, self-verify requires binary/source contract parity: when a base-checkout evidence worker is running against an installed feature-HEAD binary, record any response-contract mismatch without changing the base checkout and leave final self-verify to the coordinator on matching feature HEAD. The current opt-in LLM evaluation path only renders a read-only prompt. No Z.AI request is sent, and `gate` cannot pass without an ingested verdict. If the coordinator environment intentionally exports `HARNESS_SELF_VERIFY_LLM_EVAL=gate`, run the required deterministic completion sequence with explicit `--llm-eval=false`, record the override, and restart from its first gate after an interrupted or prompt-only run. Native hook lookup uses the default IssueOps state root in V1; safe custom `HARNESS_STATE_DIR` propagation remains issue #17.

The focused hook-input package is `./cmd/harness/hookcli/hookinput`; `./internal/core/hookinput` does not exist. After installing native integrations, run `bun scripts/smoke-gjc-native-hook.ts "$HOME/.gjc/agent/hooks/agent-harness.ts"` and require its JSON host/session/cwd/block assertions. Do not use a literal `--host gjc` grep because the TypeScript shim constructs argv as separate array elements and text layout is not execution evidence.

A targeted Go test is GREEN only when the intended test names actually ran. `[no tests to run]` is not GREEN; update stale regex names and rerun with `-v`, requiring the named `=== RUN` lines and PASS:

```bash
go test -v ./internal/core/lifecycle -run '^(TestHandoffGuardBlocksExplicitHistoricalMailboxInjection|TestHandoffGuardEnforcesInstalledOrchestrationMessageTypes)$' -count=1
```

A verification command routed through a pipeline such as `go test ... | tail` or `go test ... | grep` is non-evidence unless the test process's own exit status is separately proven; rerun required suites as direct commands. If an execution tool yields a shell `session_id` without `exit_code`, resume that exact shell session with `write_stdin` until terminal output and the real exit code arrive. Waiting on an already-completed outer tool cell is not a substitute for resuming the yielded shell session. `tui-idle`, missing task heartbeat, filesystem quiescence, a spinner, or partial package output alone proves neither worker completion/hang nor validation success. Immediately before interrupt or close, inspect the host session's active tool/process and latest `tool_result`; if the exact verification process remains active, wait and poll it to terminal exit.

Codex 0.144.1 initializes hooks during session setup, while its `refresh_runtime_config` path can rebuild and publish them later. Replacing `~/.codex/hooks.json` through native install did not refresh the observed live worker, so an active Codex session may retain its previously loaded hook command until runtime config refresh or a new session. Installed-file readback alone is insufficient; the live current-session probe is authoritative. Keep the installer `--host codex` flag for fresh or refreshed PreToolUse sessions; for a retained older PreToolUse command, hookcli defaults the native host to `codex` only when both the payload host and `--host` are empty. The installed Codex Stop command is also hostless, so Stop applies that default only to completed-worker native identity matching when both sources are empty; explicit payload/flag conflicts remain fail-closed and output formatting still follows the original flag. This compatibility path still requires an exact nonempty session, canonical cwd/repo, persisted fence, and in-tree target. It never overwrites an explicit host. After installing a compatible binary, authorize at most one same-worker retry; do not bypass the guard or start a fresh session unless the compatibility repair cannot be made safe.

When both host sources are explicit, an explicit payload and CLI hosts conflict is an identity error; neither value may override the other. Canonical Codex/Claude/GJC host, exact session, and optional agent identity form one authority tuple.

In Codex PreToolUse input, top-level `transcript_path` and `agent_transcript_path` are hook metadata, not mutation targets, and may point outside the repository. Ignore those keys only outside `tool_input`; tool_input paths and patch targets remain enforced. Before authorizing a repaired live retry, require a full-payload probe containing the external transcript metadata as well as the exact session, cwd, and proposed tool input. A synthetic payload that omits host metadata is not sufficient evidence.

Quoted semicolons, ampersands, and pipes in evidence values are argument data, not compound-command operators. The lifecycle guard parses quote boundaries so legitimate `--verification` and `--cleanup-receipt` prose remains allowed; unquoted shell control operators and newlines remain blocked. Omit `--agent-id` when the native agent id is empty because a quoted empty argument may be lost by host tokenization. The examples below therefore omit the optional flag; add `--agent-id "$AGENT_ID"` only when the native payload supplies a nonempty value.

Supervised shell red flags include active command substitution, unquoted process substitution, zsh equals expansion (`=git` and `=(...)`), parameter/tilde expansion, and unquoted brace/glob pathname expansion. Single-quoted or escaped literal evidence remains data; double-quoted process-substitution spelling is also literal in the verified Bash/zsh contract. Use explicit canonical paths and an explicit argv instead of `eval`, `source`, wrappers, globs, or expansion-generated controller names.

The raw terminal steering surface is coordinator-only correction, not a worker escape hatch. A claimed worker, a feature-worktree session, and every non-source session must not call terminal `send`, `stop`, `create`, `switch`, `focus`, `close`, `rename`, or `split` (or write/input/type/paste aliases). The target hook is not a safety boundary for injected shell text. During preparation or dispatch, use `issueops handoff start`; arbitrary terminal mutation is forbidden. `list`, `show`, `read`, and `wait` remain read-only observations.

The only steering exception is literal-safe claimed-worker guidance issued from the exact source coordinator root while the selected handoff is already `claimed`. The installed argv shape is `orca terminal send --terminal <handle> --text <payload> --enter --json`, but authorization narrows both values: request CWD and repo identity must equal `record.Repo`, and `--terminal` must be a uniquely matching persisted worker terminal handle across active handoffs; an unknown, duplicated, or arbitrary `term_*` is never sufficient. This lets concurrent cycles with distinct handles select the claimed worker without weakening ambiguity safety. The payload must be `# agent-harness guidance: <single-line-literal>`; decoded guidance must contain no ASCII C0 control rune (`0x00`–`0x1F`) or DEL (`0x7F`): backspace, tab, and ESC can alter a bare PTY even when the visible text begins with `#`; ordinary Korean and Unicode text remain valid. Prefer a direct argv call with payload as one argument. If a relay accepts only shell text, apply a POSIX single-quote encoder exactly once (`'` becomes `'\''`) to each dynamic value before composing the command. Never pass freeform text through JSON double-quoting, shell command substitution, backticks, or JS template interpolation such as `${...}`; build a plain literal string first and encode it once.

```bash
agent-harness issueops handoff claim \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --cwd "$WORKTREE_PATH" \
  --orca-worktree-id "$ORCA_WORKTREE_ID" \
  --json
```

Only that native host/session/agent identity may heartbeat or finish the claimed attempt:

```bash
agent-harness issueops heartbeat \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --json
```

If a worker is blocked, send one escalation to the concrete coordinator handle, keep heartbeat at the required cadence, remain mutation-free, and wait for coordinator repair, retry, or cancel. After escalating, the worker must not invoke `orca orchestration ask`, create a duplicate ask or decision gate, or start a separate human-choice workflow.

The current `orca orchestration send --type` enum is exactly `status`, `dispatch`, `worker_done`, `merge_ready`, `escalation`, `handoff`, `decision_gate`, and `heartbeat`. Use `status` for non-terminal progress, `heartbeat` for liveness, one `escalation` for a blocker, and `worker_done` exactly once at completion. Do not invent message types such as `progress`, `blocked`, `completed`, or `task_bounded`; `task_bounded` is not a message type and may appear only as a subject label. Report non-terminal progress with the exact persisted identifiers:

```bash
orca orchestration send --to "$COORDINATOR_HANDLE" \
  --type status --subject "<short label>" \
  --body "<bounded progress>" \
  --task-id "$ORCA_TASK_ID" \
  --dispatch-id "$ORCA_DISPATCH_ID"
```

PreToolUse blocks every other explicit message type, including malformed duplicate `--type` flags, before supervised-record selection; a valid or omitted type still passes through the existing authority checks. The mailbox repeat-prevention guard blocks any explicit `--inject` on direct `orca orchestration check`, including implicit-default unread, `--unread`, `--all`, reordered, and equals forms; in particular, never use `--unread --inject`. Read and project the bounded mailbox with the installed envelope shape:

```bash
orca orchestration check --all --json | jq '.result.messages[]'
```

The array is nested under `.result.messages`; querying top-level `.messages` silently yields no rows and is not authoritative absence. Never order or filter opaque `msg_*` IDs: they are not chronological. Select the numeric `sequence` plus exact `taskId`, `dispatchId`, sender and recipient direction before acting. Sequence is evidence, not a lease fence. Never prestate or predict a future mailbox sequence; only the returned send envelope or a subsequent bounded mailbox observation supplies the sequence. A live terminal handle is not historical mailbox identity. For an urgent current-worker correction, use only the existing exact literal-safe source-coordinator terminal guidance bound to the uniquely persisted worker handle; automatic handle/mailbox synchronization remains issue #17.

PreToolUse blocks direct `orca orchestration ask` and `orca orchestration gate-create` from a linked worker checkout even when `execution_handoff` is absent. The source coordinator owns decision gates; `gate-list` remains read-only for workers. After one escalation, heartbeat and wait rather than opening a duplicate gate.

On completion, submit bounded evidence. A completed result requires the final head, Turing report, at least one verification entry, and at least one cleanup receipt:

The Turing report path is a safe relative path from the canonical worker root. The Turing report must exist inside the canonical worker root as committed regular-file content; an absolute path, `..` escape, or leaf symlink is rejected. The worker worktree must be clean before acceptance. The context source fingerprint is re-rendered at claim and finish so plan or intent drift cannot be joined later.

```bash
agent-harness issueops handoff finish \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --host "$HOST" \
  --session-id "$SESSION_ID" \
  --outcome completed \
  --final-head "$FINAL_HEAD" \
  --changed-file "$CHANGED_FILE" \
  --turing-report "$TURING_REPORT" \
  --verification "$VERIFICATION_RECEIPT" \
  --cleanup-receipt "$CLEANUP_RECEIPT" \
  --task-id "$ORCA_TASK_ID" \
  --dispatch-id "$ORCA_DISPATCH_ID" \
  --json
```

Completed `handoff finish` is the automatic best-effort projection boundary. It persists the submitted result and projection intent in the same cycle lock, so no claimed-to-submitted crash boundary can expose final authority without the no-retry intent. Only after that write is visible does the argv-only adapter attempt one external `worker_done` outside the lock. The projection derives its concrete coordinator recipient, exact persisted task and dispatch, changed files, absolute in-worker report path, final head, host/attempt identity, subject, and three-sentence body from the durable record and freshly verified committed worker evidence.

The external message `--from` is the sealed historical worker mailbox recorded by the original dispatch, because Orca completion reconciliation requires the dispatch assignee identity. `WorkerTerminalHandle` is the refreshable live terminal control identity only; runtime rollover may update it but must never replace either sealed mailbox recipient or the projection sender. Success stores bounded message identity/sequence. Failure, timeout, malformed output, or a crash before/after the send leaves submitted authoritative with a terminal diagnostic or intent and is never automatically retried. An identical finish returns that stable evidence without another call. The manual shell `worker_done` is blocked, as is coordinator guidance asking a submitted worker to retry it.

Orchestration reviews and task-scoped messages target the sealed `WorkerMailboxHandle`; the refreshed `WorkerTerminalHandle` is only terminal read/send/close control. Automatic `worker_done` uses the sealed `WorkerMailboxHandle` as `--from`.

After verification and immediately before `handoff finish` triggers automatic projection, the worker performs a bounded current-task inbox check on its sealed historical mailbox. Select only messages with the numeric `sequence`, exact `taskId` and `dispatchId`, and sender and recipient direction; process every newly arrived current-task `status` or `escalation` through the observed maximum sequence. If a message changes the result, repeat any affected verification and commit before finish, then run the fence again. Hooks may only observe, block, or relay this boundary and must never execute the workflow, tests, commits, task creation, dispatch, finish, or `worker_done`.

Before this fence, each worker commit must use a Conventional Commit subject and a literal `Lore:` block with `Intent`, `Why`, `Changes`, `Verify`, and `Risk` as required by `.agent-harness/COMMIT_POLICY.md`.

After `worker_done` succeeds, the worker stops. The worker must not push, create or merge a PR/MR, accept its own result, delete the provider branch, or remove the coordinator-owned worktree. The coordinator owns PR, acceptance, and cleanup. A completed worker never manually re-sends `worker_done`; the Stop hook's numbered-next-action relay and missing-choice re-entry are suppressed only for the exact worker whose durable handoff record already carries a terminal `worker_done_projection` (sent, failed, or intent) — the suppression reads that persisted projection plus native host/session/worktree identity, never a transcript, shell output, or `ORCA_TERMINAL_HANDLE`. Every other Stop check (Engelbart Canvas gate, `--json` fields, metrics) is unaffected, and any mismatch or ambiguity falls back to legacy Stop behavior unchanged.

A completed dispatch is never a mutation lease. After a valid `worker_done`, the coordinator closes the exact worker terminal. Review feedback that requires edits starts with a new ready task. That fresh ready task, dispatch, host attestation, and sole-writer proof are required before any edit, with a fresh dispatch bound to the new task and an exact sole-writer attestation. Never send edit instructions to a completed worker.

## Coordinator Acceptance

The coordinator inspects the submitted head and evidence, reruns the required checks, and then accepts the exact fence:

```bash
agent-harness issueops handoff accept \
  --id "$ISSUEOPS_ID" \
  --attempt "$ATTEMPT" \
  --ownership-epoch "$OWNERSHIP_EPOCH" \
  --context-sha256 "$CONTEXT_SHA256" \
  --final-head "$FINAL_HEAD" \
  --json
```

`issueops resume --bind` is intentionally read-only for supervised handoffs. It cannot transfer the lease; use the explicit claim/recovery commands.

## Coordinator Publish

After acceptance, verify the accepted FinalHead before any publish action. Run the ref query as a standalone read-only command, inspect its stdout, and continue only when that full SHA exactly equals the accepted FinalHead. Do not turn the comparison into shell reinterpretation, command substitution, or a wrapper.

```bash
git rev-parse --verify refs/heads/<branch>
# Stop unless the stdout above exactly equals <accepted-final-head>.
git push <remote> <branch>
gh pr create --head <branch> --base <base-branch> --draft --title <title> --body <body>
# GitLab equivalent:
glab mr create --source-branch <branch> --target-branch <base-branch> --draft --title <title> --description <body>
```

The order is accepted FinalHead versus `refs/heads/<branch>`, exact branch push, then explicit head/source and base/target flags for a draft PR/MR. `HEAD`, force/delete push, implicit current-branch PR creation, merge, close, reopen, fill, web, and wrapper-side push are outside this authority.

## Failure And Recovery

IssueOps persists `pending_operation` before every external Orca create or dispatch. Once a mutation has been invoked, never retry a create operation automatically, even when the adapter reports a timeout or ambiguous error. The record moves to `recovery_required`, inline fallback is forbidden, and the coordinator inspects status before choosing an explicit action.

Reconcile first:

```bash
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action reconcile --json
```

Use the public Orca CLI with these exact copyable inspection forms:

```bash
orca orchestration task-list --ready --json
orca orchestration dispatch-show --task <id> --json
orca worktree show --worktree id:<id> --json
```

`orca task show` is not a command in Orca 1.4.134, and dispatch-show accepts `--task`, not `--task-id`. A failed inspection syntax is not evidence that an external identity is absent.

Recovery accepts exactly one candidate relative to the persisted baseline and marker. Zero or multiple matching worktrees, terminals, tasks, or dispatches fail closed and preserve the pending journal. Never guess an identity and never issue another create to discover whether the first one worked.

### Runtime rollover recovery

An Orca runtime restart may reissue the runtime ID, live terminal handle and PTY, and worktree instance while the same logical terminal keeps its public `tabId and leafId`. The dynamic terminal title is presentation state and is never the sole durable locator. Persist `runtime_refresh` before adopting replacement identities, then collect complete bounded inventories. The locked refresh may update only runtime/worktree-instance/PTY/tab/leaf and the refreshable live terminal handle; sealed coordinator and worker mailbox recipients are immutable:

```bash
orca worktree list --repo path:<exact-repo> --limit 512 --json
orca terminal list --worktree id:<persisted-worktree-id> --limit 512 --json
```

Require exactly one worktree row matching the sealed repo, base, path, branch, clean exact HEAD, and comment marker. Its instance ID must be nonempty but may equal the previously persisted instance. Require the adopted terminal to name that worktree and current runtime. When stable terminal IDs were recorded, match the exact `tabId and leafId`; for a legacy row that never recorded them, join the terminal by exact tab/leaf to `visualLayouts[].root.tabs[].title` and require the exact bounded marker title. Missing fields, mismatches, duplicate candidates, or conflicting instance evidence leave the lease in `recovery_required`. The runtime-only locked completion exact-compares the journal snapshot and revalidates the sealed context source and clean exact branch/HEAD immediately before refreshing runtime, worktree instance, handle, PTY, tab ID, and leaf ID in one compare-and-set write.

Never launch a replacement while an exact recovered terminal is connected and writable or the worker root contains uncommitted WIP. Monitor only with bounded reads, carrying the previous response's `nextCursor` forward:

```bash
orca terminal read --terminal <recovered-current-handle> --cursor <nextCursor> --limit 1000 --json
```

A handshake-only local Orca observation is bounded by caller-side Ctrl-C or host tool cancellation. Do not send control input into the target PTY to stop an observation; terminal mutation remains authority-gated.

If reconciliation proves no safe continuation, close the attempt explicitly. A later retry is a new attempt and ownership epoch, and is allowed only after the prior attempt is safely closed with no ambiguous pending operation:

```bash
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action cancel --confirm --json
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action finalize-cancel --confirm --json
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action retry --confirm --json
```

`closed/accepted` is terminal and cannot be retried. Only `closed/worker_failed` and `closed/cancelled` may start a new attempt; an accepted handoff has already transferred the verified result back to the coordinator and must remain closed.

A claimed cancel is fail-closed without explicit stale or force evidence, and an unresolved pending operation journal survives cancel inside the tombstone. A provisioned cancel first writes a durable `recovery_required` cancellation tombstone and preserves the operation journal, worker identity, result, and lifecycle guard. Claimed and submitted attempts additionally require `--force` with a bounded nonempty reason; use that authority only after investigating the worker lease. Only a truly pre-mutation preparation closes directly.

`finalize-cancel` releases the guard only after authoritative evidence proves every pending exact candidate absent, the exact terminal disconnected or absent, the exact task/dispatch terminal or authoritatively absent, and any claimed heartbeat older than the minimum age. An inventory row may be ignored as unrelated only when it has a unique stable identity and all fields needed to classify exact-vs-unrelated; missing, duplicate, or incomplete rows remain ambiguous. A failed finalization leaves the tombstone active and byte-equivalent.

Force-abandon is a separate last-resort operation for an old ambiguous journal. It requires complete non-truncated inventory, the minimum operation age, explicit confirmation, `--force`, and a bounded reason:

```bash
agent-harness issueops handoff recover --id "$ISSUEOPS_ID" --action abandon --confirm --force --reason "<verified reason>" --json
```

It may ignore only fully identifiable and classifiable nonmatching rows. An exact candidate, malformed identity, duplicate stable identity, incomplete classification row, incomplete inventory, or non-authoritative dispatch error blocks abandonment; a successfully force-abandoned attempt is not retryable.

Before retry, require a clean exact branch and HEAD checkpoint. Persist the current clean commit as the new `attempt_base_head`; dirty, detached, mismatched, or unreadable Git evidence stops before any external mutation. Classifier gaps are handled by promoting each observed representative mutation family into a retained hook test rather than relying on prose-only denial.

## Coordinator Cleanup

For `execution_handoff.driver=orca`, Git worktree removal is not the cleanup route. The coordinator performs and verifies this order after explicit user cleanup approval:

```bash
orca orchestration task-update --id <persisted-task-id> --status <completed-or-failed> --result <bounded-result> --json
orca terminal close --terminal <resolved-worker-terminal-handle> --json
orca terminal stop --worktree id:<persisted-worktree-id> --json
orca worktree rm --worktree id:<persisted-worktree-id> --force --json
orca terminal list --worktree id:<persisted-worktree-id> --limit 512 --json
```

The exact `orca terminal close --terminal <resolved-worker-terminal-handle> --json` form closes one pane using the currently resolved `WorkerTerminalHandle`; `orca terminal stop --worktree id:<persisted-worktree-id> --json` stops the exact persisted worktree terminal set. Never use `terminal rm`: no such public cleanup command exists. Each is an optional bounded cleanup attempt for source-root coordinators only after `closed/worker_failed` or `closed/cancelled`; it is blocked for accepted or active attempts, worker/non-source sessions, wrong identities, and extra flags. The sealed `WorkerMailboxHandle` is never terminal-control authority: never target it with `orca terminal send`, close, stop, or injected `exit`; it remains the sealed orchestration `worker_done` sender. A successful close or stop is not complete cleanup evidence. A worktree removal is not terminal cleanup evidence: verify every exact spawned handle and PTY is connected=false or absent from terminal list; nested shells require repeated inspection until fully gone. Also verify the exact worktree selector and path are absent before handling provider refs. Inline records retain the legacy Git cleanup recipe; never substitute `git worktree remove` for Orca-owned removal.

`auto` fallback is allowed only after a pre-mutation probe failure. It is never a recovery strategy for `coordinator_preparing`, `dispatched`, `claimed`, `submitted`, or `recovery_required` state.
