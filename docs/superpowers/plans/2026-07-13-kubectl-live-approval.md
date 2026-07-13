# Kubectl Live-Access One-Shot Approval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` task-by-task. Repository policy requires the main agent to perform this cross-cutting hook/state change directly; do not dispatch sub-agents.

**Goal:** Let Codex consume a user-approved `kubectl exec` or `kubectl port-forward` grant exactly once without weakening GitOps mutation blocking or Claude native approval.

**Architecture:** Add a host-neutral `internal/core/lifecycle/liveapproval` state machine backed by the existing project lifecycle namespace, atomic JSON writes, and keyed locks. Codex PreToolUse creates or consumes a grant; UserPromptSubmit promotes an exact `승인 AH-XXXXXX` prompt. Claude and non-live-access kubectl paths remain unchanged.

**Tech Stack:** Go 1.26, standard-library `crypto/rand` and `crypto/sha256`, existing lifecycle state namespace, existing SQLite-backed `state.WithKeyLock`, Go tests.

## Global Constraints

- Follow [the approved design](../specs/2026-07-13-kubectl-live-approval-design.md).
- Store runtime state only under `$HARNESS_STATE_DIR/projects/<repo-id>/`, never in the target repo.
- Never persist or echo the raw command, kube context, namespace, workload, environment, or secrets in approval state.
- Bind grants to host, session ID, canonical repo root, cwd, tool, and exact trimmed command.
- Pending and granted records expire after exactly 10 minutes.
- Delete a grant under lock before returning allow; never restore it.
- Preserve GitOps mutation blocks, read-only/dry-run allows, and Claude native `ask`.
- Do not change hook response schemas or add a generic approval framework.
- Use TDD for every behavior change: observe the targeted failure before production edits.
- Do not commit or push unless the user separately requests it.

---

## File Map

- Create `internal/core/lifecycle/liveapproval/live_approval.go`: record schema, fingerprint, token, state transitions, atomic consume.
- Create `internal/core/lifecycle/liveapproval/live_approval_test.go`: deterministic unit and concurrency tests.
- Modify `internal/core/lifecycle/dependencies.go`: adapt lifecycle namespace, lock, and atomic writer to the new package.
- Modify `internal/core/lifecycle/lifecycle_state.go`: apply one-shot evaluation only to Codex kubectl live-access asks.
- Create `internal/core/lifecycle/lifecycle_live_approval_test.go`: lifecycle integration.
- Modify `internal/core/hookprompt/hook_prompt.go`: process exact approval prompts before normal routing.
- Modify `cmd/harness/hookcli/hook_user_prompt.go`: forward resolved host and session ID.
- Modify hook CLI tests: prove first-block/approve/one-allow/re-block and Claude preservation.
- Update `.agent-harness/CAUTIONS.md`, `CONVENTIONS.md`, `TESTING.md`, and `OPERATIONS.md`.

---

### Task 1: Implement the isolated liveapproval state machine

**Files:**
- Create: `internal/core/lifecycle/liveapproval/live_approval_test.go`
- Create: `internal/core/lifecycle/liveapproval/live_approval.go`

**Interfaces:**

```go
const ApprovalTTL = 10 * time.Minute

type Namespace struct {
    Exists   bool
    Valid    bool
    RepoRoot string
    Dir      string
}

type Store struct {
    Resolve   func(repoRoot string) (Namespace, error)
    Init      func(repoRoot string) (Namespace, error)
    WithLock  func(dir, key string, fn func() error) error
    WriteJSON func(path string, value any, perm os.FileMode) error
    Now       func() time.Time
    NewToken  func() (string, error)
}

type Request struct {
    Host      string
    SessionID string
    RepoRoot  string
    CWD       string
    Tool      string
    Command   string
}

type ApprovalRequest struct {
    Host      string
    SessionID string
    RepoRoot  string
    Prompt    string
}

type Result struct {
    Handled           bool
    Allowed           bool
    Token             string
    Reason            string
    AdditionalContext string
}

func Evaluate(store Store, req Request) Result
func Approve(store Store, req ApprovalRequest) Result
```

The package-private record has `schema_version`, `status`, `token`, `request_fingerprint`, and `expires_at`. Approval prompt parsing accepts only `^승인 (AH-[A-HJ-NP-Z2-9]{6})$`.

- [ ] **Step 1: Write failing fingerprint and storage-hygiene tests**

Use a temp namespace, fixed clock, and fixed token. Assert the first request creates `AH-ABC234`, the same pending request reuses it, and changing session/repo/cwd/tool/command replaces it. Read JSON and assert it excludes the raw command and has mode `0600`.

```go
first := Evaluate(store, baseRequest)
if !first.Handled || first.Allowed || first.Token != "AH-ABC234" {
    t.Fatalf("unexpected first evaluation: %+v", first)
}
same := Evaluate(store, baseRequest)
if same.Token != first.Token {
    t.Fatalf("same pending request rotated token: %+v", same)
}
```

- [ ] **Step 2: Run the test and observe RED**

```bash
go test ./internal/core/lifecycle/liveapproval -run 'TestEvaluate' -count=1
```

Expected: compile failure because the package API does not exist.

- [ ] **Step 3: Implement minimal pending logic**

Fingerprint a length-delimited sequence containing domain tag, normalized host, session, canonical repo, cwd, tool, and exact trimmed command:

```go
func requestFingerprint(req Request, canonicalRepo string) string {
    fields := []string{
        "kubectl-live-approval:v1",
        strings.ToLower(strings.TrimSpace(req.Host)),
        strings.TrimSpace(req.SessionID),
        canonicalRepo,
        strings.TrimSpace(req.CWD),
        strings.TrimSpace(req.Tool),
        strings.TrimSpace(req.Command),
    }
    h := sha256.New()
    for _, field := range fields {
        _ = binary.Write(h, binary.BigEndian, uint64(len(field)))
        _, _ = h.Write([]byte(field))
    }
    return hex.EncodeToString(h.Sum(nil))
}
```

Hash the session ID for the filename and lock key. Resolve or initialize the namespace, hold `Store.WithLock`, reuse an unexpired matching pending token, otherwise atomically write a new pending record. Production token generation uses `crypto/rand.Reader` and `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`.

- [ ] **Step 4: Run the focused test and observe GREEN**

```bash
go test ./internal/core/lifecycle/liveapproval -run 'TestEvaluate' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing approval, expiry, one-shot, and concurrency tests**

```go
approved := Approve(store, ApprovalRequest{
    Host: "codex", SessionID: baseRequest.SessionID,
    RepoRoot: baseRequest.RepoRoot, Prompt: "승인 " + first.Token,
})
if !approved.Handled || approved.Allowed {
    t.Fatalf("approval prompt did not grant: %+v", approved)
}
consumed := Evaluate(store, baseRequest)
if !consumed.Allowed {
    t.Fatalf("granted request was not allowed: %+v", consumed)
}
again := Evaluate(store, baseRequest)
if again.Allowed || again.Token == "" {
    t.Fatalf("grant was reusable: %+v", again)
}
```

Also cover wrong token, extra prose, lowercase token, different session, exactly 10-minute expiry, corrupt/future schema, and two concurrent consumers with exactly one allow.

- [ ] **Step 6: Run these tests and observe RED**

```bash
go test ./internal/core/lifecycle/liveapproval -run 'TestApprove|TestExpiry|TestConsume|TestConcurrent' -count=1
```

Expected: FAIL because approval and consume transitions are absent.

- [ ] **Step 7: Implement approval and atomic consume**

Under the session lock, reject invalid records, rewrite a matching pending record as `granted` with a fresh expiry, and remove a matching grant before setting `Allowed=true`. A different request discards an existing grant and creates a new pending token. Removal failure is fail-closed.

- [ ] **Step 8: Verify the package**

```bash
go test ./internal/core/lifecycle/liveapproval -count=1
go test -race ./internal/core/lifecycle/liveapproval -count=1
```

Expected: both PASS.

- [ ] **Step 9: Review the task diff**

Run `git diff -- internal/core/lifecycle/liveapproval`. Do not commit.

---

### Task 2: Integrate Codex PreToolUse

**Files:**
- Modify: `internal/core/lifecycle/dependencies.go`
- Modify: `internal/core/lifecycle/lifecycle_state.go`
- Create: `internal/core/lifecycle/lifecycle_live_approval_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_gitops_staged_test.go`

**Interfaces:**

```go
func liveApprovalStore() liveapproval.Store
func ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt string) liveapproval.Result
```

The existing `BuildLifecyclePreToolUseDecision` signature and result schema remain unchanged.

- [ ] **Step 1: Write failing lifecycle integration tests**

With `HARNESS_STATE_DIR=t.TempDir()`, a temp repo, `Host:"codex"`, and a session ID, assert the first live-access decision contains `승인 AH-`, approval makes the next exact request allow once, and the following request asks again.

Add preservation cases: Claude returns `ask` without state; missing session blocks without token; mutation remains GitOps-blocked; read-only/dry-run remains allow without state initialization.

- [ ] **Step 2: Run and observe RED**

```bash
go test ./internal/core/lifecycle -run 'TestCodexKubectlLiveApproval' -count=1
```

Expected: FAIL because live-access asks are not state-aware.

- [ ] **Step 3: Add the store adapter**

Map `ValidateProjectLifecycleState`, `InitProjectLifecycleState(repo, true)`, `state.WithKeyLock`, and `writeJSONAtomic` into `liveapproval.Store`. Convert `ProjectLifecycleStatePlan` to `liveapproval.Namespace` without importing the parent lifecycle package from the child package.

- [ ] **Step 4: Add the Codex-only decision branch**

In the GitOps kubectl branch, invoke liveapproval only when the command guard returns `ask` and host is Codex. Return allow only for `Result.Allowed`; otherwise use its token-bearing or fail-closed reason. Claude bypasses this branch.

- [ ] **Step 5: Run lifecycle and commandguard regression tests**

```bash
go test ./internal/core/lifecycle ./internal/core/commandguard -run 'Kubectl|LiveApproval' -count=1
```

Expected: PASS.

- [ ] **Step 6: Update the Codex CLI test first**

Add `session_id`, expect a token-bearing block, and assert a repeated pre-approval attempt returns the same token. Keep the Claude native-ask assertion and add absence of approval state.

- [ ] **Step 7: Run CLI PreToolUse tests**

```bash
go test ./cmd/harness/hookcli -run 'KubectlLiveAccess' -count=1
```

Expected: PASS after integration.

- [ ] **Step 8: Review the task diff**

Run `git diff -- internal/core/lifecycle cmd/harness/hookcli/hook_pre_tool_gitops_staged_test.go`. Do not commit.

---

### Task 3: Promote exact UserPromptSubmit approval

**Files:**
- Modify: `internal/core/hookprompt/hook_prompt.go`
- Modify: `internal/core/hookprompt/hook_prompt_test.go`
- Modify: `cmd/harness/hookcli/hook_user_prompt.go`
- Modify: `cmd/harness/hookcli/hook_prompt_session_test.go`
- Modify: `cmd/harness/hookcli/hook_pre_tool_gitops_staged_test.go`

**Interfaces:**

```go
type HookUserPromptRequest struct {
    Prompt               string `json:"prompt"`
    Repo                 string `json:"repo,omitempty"`
    Host                 string `json:"host,omitempty"`
    SessionID            string `json:"session_id,omitempty"`
    EnableLLMHints       bool   `json:"enable_llm_hints,omitempty"`
    DisableKarpathyFirst bool   `json:"disable_karpathy_first,omitempty"`
}
```

- [ ] **Step 1: Write failing hookprompt approval tests**

Create a pending approval through lifecycle, then call `BuildUserPromptMCPHints` with the exact token. Assert `ShouldInject=true`, bounded success context, no raw command/fingerprint, and no Karpathy augmentation. Cover wrong token, extra prose, missing session, and Claude.

- [ ] **Step 2: Run and observe RED**

```bash
go test ./internal/core/hookprompt -run 'TestBuildUserPrompt.*Approval' -count=1
```

Expected: compile or assertion failure because host/session approval processing is absent.

- [ ] **Step 3: Process approval before routing**

After initializing the base result and before `karpathyFirstDecision`, call `ApproveCodexKubectlLiveAccess`. If handled, return only the bounded success/failure context:

```go
result.ShouldInject = true
result.AdditionalContext = approval.AdditionalContext
return result
```

Non-approval prompts continue through existing routing unchanged.

- [ ] **Step 4: Run hookprompt tests**

```bash
go test ./internal/core/hookprompt -run 'Approval|Karpathy' -count=1
```

Expected: PASS.

- [ ] **Step 5: Write a failing CLI end-to-end test**

Within one temp state and repo:

1. Codex PreToolUse with session and live command -> token-bearing block.
2. Codex UserPromptSubmit with same session and `승인 <token>` -> approval context.
3. Same PreToolUse -> `{}` allow.
4. Same PreToolUse again -> new token-bearing block.

Also assert another session or command cannot consume the grant.

- [ ] **Step 6: Run and observe RED**

```bash
go test ./cmd/harness/hookcli -run 'TestRunHookCodexKubectlLiveApprovalFlow' -count=1
```

Expected: FAIL because UserPromptSubmit does not forward host/session.

- [ ] **Step 7: Forward host and session**

Resolve payload host and `--host` consistently with PreToolUse, default to Codex, reject explicit host conflicts, and pass `SessionIDFromHookInput(stdin)`. Preserve Claude/Reasonix `UserNotice` formatting and raw JSON output.

- [ ] **Step 8: Run CLI hook tests**

```bash
go test ./cmd/harness/hookcli -run 'KubectlLiveApproval|KubectlLiveAccess|HookUserPrompt' -count=1
```

Expected: PASS.

- [ ] **Step 9: Run combined hook regression**

```bash
go test ./internal/core/lifecycle/... ./internal/core/hookprompt ./internal/core/commandguard ./internal/adapter/hook ./cmd/harness/hookcli -count=1
```

Expected: PASS.

- [ ] **Step 10: Review the task diff**

Run `git diff -- internal/core/hookprompt cmd/harness/hookcli`. Do not commit.

---

### Task 4: Document and verify the final contract

**Files:**
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/CONVENTIONS.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Existing: `docs/superpowers/specs/2026-07-13-kubectl-live-approval-design.md`
- Existing: `docs/superpowers/plans/2026-07-13-kubectl-live-approval.md`

- [ ] **Step 1: Update docs after behavior tests are green**

Record: Codex first-block/token/one-allow flow; Claude native ask; project-scoped 10-minute state without raw commands; required mismatch/expiry/concurrency tests; and recovery by retrying for a new token rather than disabling the GitOps gate.

- [ ] **Step 2: Verify docs and focused behavior**

```bash
git diff --check
rg -n "one-shot|AH-|10분|native.*ask|raw command" .agent-harness docs/superpowers
go test ./internal/core/lifecycle/... ./internal/core/hookprompt ./internal/core/commandguard ./internal/adapter/hook ./cmd/harness/hookcli -count=1
```

Expected: no whitespace errors, contract phrases present, tests PASS.

- [ ] **Step 3: Run full test and race suites**

```bash
go test ./... -count=1
go test -race ./... -count=1
```

Expected: both exit 0 with no failures or race reports.

- [ ] **Step 4: Build and smoke with isolated state**

```bash
go build -o bin/agent-harness ./cmd/harness
tmp_state="$(mktemp -d)"
payload='{"cwd":"'"$PWD"'","session_id":"approval-smoke","tool_name":"Bash","tool_input":{"command":"kubectl exec -n stg deploy/rest-api-gateway -- getent hosts grpc-user"}}'
printf '%s\n' "$payload" | HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness hook pre-tool-use --host codex --enforce-gitops-kubectl
```

Expected: token-bearing block. Extract the token, submit an exact UserPromptSubmit approval with the same session, verify the next PreToolUse emits `{}`, and verify the following call blocks with a new token. Remove `$tmp_state` afterward.

- [ ] **Step 5: Inspect security and scope**

```bash
git status --short
git diff --stat
git diff --check
```

Inspect the isolated state directory before removal and verify the raw smoke command is absent. Confirm no unrelated files changed.

- [ ] **Step 6: Leave verified changes uncommitted**

Report exact verification results. If the user later requests a commit, use `atomic-commit-push`, stage exact files, and follow `.agent-harness/COMMIT_POLICY.md`.

---

## Plan Self-Review

- Spec coverage: token UX, first-attempt issuance, exact binding, TTL, one-shot consumption, concurrency, host split, state hygiene, and GitOps preservation each map to a task and test.
- Placeholder scan: no placeholder, deferred implementation, or unspecified error-handling steps remain.
- Type consistency: `liveapproval.Request`, `ApprovalRequest`, `Result`, lifecycle wrapper names, and `HookUserPromptRequest` fields are consistent.
- Scope: generic approvals, audit history, multiple pending queues, remote approval, and mutation bypass remain excluded.
