# Kubectl Session Read-Only Exec Approval Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Use `superpowers:subagent-driven-development` only if the user explicitly requests delegation. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow a Codex-approved, exact-allowlisted diagnostic `kubectl exec` repeatedly within one session/repo/context/namespace scope using a 30-minute sliding TTL, while blocking unsafe Codex exec and preserving Claude native ask plus one-shot port-forward approval.

**Architecture:** `commandguard` performs deterministic shell and kubectl argv classification without knowing the host. `lifecycle` applies host policy: Codex safe exec enters a scoped approval state machine, Codex unsafe exec blocks without a token, and Claude keeps native ask. `liveapproval` retains the schema-v1 exact-command state for port-forward and adds an independent schema-v2 read-only exec record under the same session lock.

**Tech Stack:** Go 1.26.3, standard library (`crypto/sha256`, `encoding/json`, `regexp`, `time`), existing lifecycle project state and keyed lock, table-driven Go tests.

## Global Constraints

- Source of truth: `docs/superpowers/specs/2026-07-14-kubectl-session-readonly-exec-approval-design.md`.
- Scope is normalized host + session ID + canonical repo + explicit kube context + explicit namespace.
- Pending and pre-first-use activation TTL are exactly 10 minutes; every allowed scoped exec sets expiry to exactly `now + 30 minutes`.
- Only the spec's literal remote argv grammars are eligible. Unknown or ambiguous commands are never inferred read-only.
- Codex unsafe exec blocks without issuing or refreshing a token; Claude continues native `ask`.
- `kubectl port-forward` keeps schema-v1 exact-command one-shot approval and an independent state file.
- State stores hashes, timestamps, status and pending token only; never raw command, repo path, context, namespace, workload, container or DNS name.
- State mutation is serialized by one session approval lock and fails closed on read/write/lock errors.
- No new dependency, host JSON field, MCP/CLI response field or general-purpose approval framework.
- Do not push without a new explicit user request.

---

### Task 1: Classify Exact Read-Only Kubectl Exec Commands

**Files:**
- Create: `internal/core/commandguard/kubectl_readonly_exec.go`
- Create: `internal/core/commandguard/kubectl_readonly_exec_test.go`
- Modify: `internal/core/commandguard/lifecycle_command_kubectl.go`
- Modify: `internal/core/commandparse/tokens.go`
- Modify: `internal/core/commandparse/tokens_test.go`

**Interfaces:**
- Consumes: existing `commandparse.SplitCommandTokens`, shell-expansion detectors and `searchrouting.IsShellTool`.
- Produces:

```go
type KubectlLiveAccessKind string

const (
	KubectlLiveAccessNone         KubectlLiveAccessKind = ""
	KubectlLiveAccessPortForward  KubectlLiveAccessKind = "port_forward"
	KubectlLiveAccessReadOnlyExec KubectlLiveAccessKind = "readonly_exec"
	KubectlLiveAccessUnsafeExec   KubectlLiveAccessKind = "unsafe_exec"
)

type KubectlExecScope struct {
	Context   string
	Namespace string
}

type KubectlEvaluation struct {
	Decision   string
	Reason     string
	LiveAccess KubectlLiveAccessKind
	ExecScope  KubectlExecScope
}

func EvaluateGitOpsKubectl(tool, command string) KubectlEvaluation
func GitOpsKubectlDecision(tool, command string) (string, string)
func HasActiveInputRedirect(command string) bool
```

`GitOpsKubectlDecision` remains a compatibility wrapper. `EvaluateGitOpsKubectl` still returns `Decision: "ask"` for all exec/port-forward requests and distinguishes safe/unsafe exec through `LiveAccess`; lifecycle owns host-specific block policy.

- [ ] **Step 1: Add a RED test for input redirection**

Append to `tokens_test.go`:

```go
func TestHasActiveInputRedirect(t *testing.T) {
	for _, command := range []string{
		"kubectl exec pod/api -- cat /etc/resolv.conf < /tmp/input",
		"kubectl exec pod/api -- cat /etc/resolv.conf 0</tmp/input",
		"kubectl exec pod/api -- cat /etc/resolv.conf <<<value",
	} {
		if !HasActiveInputRedirect(command) {
			t.Fatalf("active input redirect must be detected: %q", command)
		}
	}
	for _, command := range []string{`printf '%s' '<'`, `printf "%s" "<"`, `printf %s \<`} {
		if HasActiveInputRedirect(command) {
			t.Fatalf("quoted or escaped input punctuation must remain data: %q", command)
		}
	}
}
```

- [ ] **Step 2: Run the parser test and confirm RED**

Run `go test ./internal/core/commandparse -run TestHasActiveInputRedirect -count=1`.

Expected: compile failure containing `undefined: HasActiveInputRedirect`.

- [ ] **Step 3: Implement input redirect detection**

Add this beside `HasActiveOutputRedirect`:

```go
func HasActiveInputRedirect(command string) bool {
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '<' {
			return true
		}
	}
	return false
}
```

Run `go test ./internal/core/commandparse -count=1`; expected PASS.

- [ ] **Step 4: Add RED positive classification tests**

Create `kubectl_readonly_exec_test.go` with every allowlist grammar:

```go
func TestEvaluateGitOpsKubectlClassifiesExactReadOnlyExec(t *testing.T) {
	commands := []string{
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl exec --context=bc-stgdev --namespace=stg deploy/gateway -- nslookup grpc-user.stg.svc.cluster.local",
		"kubectl --namespace stg exec deploy/gateway --context bc-stgdev -- dig grpc-user",
		"kubectl --context bc-stgdev exec -n stg deploy/gateway -- dig +short grpc-user",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig grpc-user A",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig grpc-user AAAA",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig +short grpc-user A",
		"kubectl --context bc-stgdev -n stg exec -c proxy deploy/gateway -- dig +short grpc-user AAAA",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- cat /etc/resolv.conf",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -fsS http://localhost:4191/metrics",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -fsS http://127.0.0.1:4191/metrics",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			got := EvaluateGitOpsKubectl("Bash", command)
			if got.Decision != "ask" || got.LiveAccess != KubectlLiveAccessReadOnlyExec || got.ExecScope.Context != "bc-stgdev" || got.ExecScope.Namespace != "stg" {
				t.Fatalf("classification = %+v", got)
			}
		})
	}
}
```

- [ ] **Step 5: Add RED rejection and compatibility tables**

Add literal rows for missing/duplicate/empty context and namespace, `--kubeconfig`/`--server`/`--token`/`--user`/`--as` auth overrides, every other unknown flag, env/sudo/timeout prefixes, `/usr/bin/kubectl`, interactive flags, shell controls, every expansion detector, input/output redirects, `env`, `printenv`, arbitrary `cat`, alternate curl options/URLs, custom DNS server, `dig -f`, extra remote argv, invalid DNS labels, slash executables and combined commands. Each exec row must remain `Decision: "ask"` but return `KubectlLiveAccessUnsafeExec` and an empty scope.

Also assert port-forward is `PortForward`, mutation is `block`, normal read-only kubectl is empty, and non-shell tools are ignored. Run:

```bash
go test ./internal/core/commandguard -run 'TestEvaluateGitOpsKubectl' -count=1
```

Expected: compile failure for the new types/function.

- [ ] **Step 6: Implement exact parsing and evaluation**

Create `kubectl_readonly_exec.go`. Reject safe classification if any active shell detector is true, including new input redirect detection. Require token zero to be exactly `kubectl`. Before remote `--`, accept only:

- `--context value` or `--context=value` exactly once;
- `-n value`, `--namespace value`, `-n=value` or `--namespace=value` exactly once;
- `exec` as the first positional token and one target after it;
- `-c`/`--container` with one non-empty value at most once after `exec`.

Require context, namespace, target, `--` and remote argv. Validate DNS operands with total length <=253, labels 1..63, alphanumeric boundaries, internal alphanumeric/hyphen, and optional final dot. Match remote argv by exact slice equality except the one validated DNS operand. Executable tokens must be slash-free literal `getent`, `nslookup`, `dig`, `cat` or `curl`.

Modify `lifecycle_command_kubectl.go` so `EvaluateGitOpsKubectl` owns the existing scan and sets `ReadOnlyExec`, `UnsafeExec` or `PortForward`. Keep the old function as:

```go
func GitOpsKubectlDecision(tool, command string) (string, string) {
	result := EvaluateGitOpsKubectl(tool, command)
	return result.Decision, result.Reason
}
```

- [ ] **Step 7: Verify and commit Task 1**

Run:

```bash
go test ./internal/core/commandparse ./internal/core/commandguard -count=1
```

Expected: PASS including existing mutation/read-only/dry-run tests.

Commit exact Task 1 paths with subject `feat(commandguard): classify readonly kubectl exec` and a Lore body recording focused evidence and host-policy deferral.

---

### Task 2: Add the Scoped Read-Only Exec Approval State Machine

**Files:**
- Create: `internal/core/lifecycle/liveapproval/readonly_exec.go`
- Create: `internal/core/lifecycle/liveapproval/readonly_exec_test.go`
- Modify: `internal/core/lifecycle/liveapproval/live_approval.go`
- Modify: `internal/core/lifecycle/liveapproval/live_approval_test.go`

**Interfaces:**
- Consumes: existing `Store`, `Namespace`, random token generator, atomic JSON writer and session hash.
- Produces:

```go
const ReadOnlyExecGrantTTL = 30 * time.Minute

type ReadOnlyExecRequest struct {
	Host      string
	SessionID string
	RepoRoot  string
	CWD       string
	Tool      string
	Command   string
	Context   string
	Namespace string
}

func EvaluateReadOnlyExec(store Store, req ReadOnlyExecRequest) Result
```

Existing `Evaluate` stays the schema-v1 exact-command evaluator used by port-forward. `Approve` checks both state files under the same `approvalKey(sessionID)` lock.

- [ ] **Step 1: Preserve one-shot tests as port-forward fixtures**

Change `testRequest().Command` to `kubectl --context bc-stgdev -n stg port-forward svc/api 8080:80` and rename the one-shot test to mention port-forward. Keep request binding, exact prompt, 10-minute expiry, concurrent single-consumer, no raw command and `0600` assertions.

- [ ] **Step 2: Add RED scoped-state tests**

Create `readonly_exec_test.go` and use this fixture:

```go
func testReadOnlyExecRequest() ReadOnlyExecRequest {
	base := testRequest()
	return ReadOnlyExecRequest{
		Host: base.Host, SessionID: base.SessionID, RepoRoot: base.RepoRoot,
		CWD: base.CWD, Tool: base.Tool,
		Command: "kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user",
		Context: "bc-stgdev", Namespace: "stg",
	}
}
```

Tests must prove pending creation/reuse/replacement, no raw strings and `0600`; exact token approval; 10-minute activation; first and repeated same-scope allow; different target/container/allowlisted argv reuse; 30-minute sliding expiry with injected clock; session/repo/context/namespace isolation; CWD/tool excluded from granted scope but included in pending request identity; one active scope replacement; non-approval user prompts leave expiry unchanged; fail-closed I/O/corrupt/future schema; concurrent serialized allow; rejection when the same token ambiguously matches both pending kinds; and rejection of a matching legacy schema-v1 exec grant.

Use this exact expiry boundary assertion:

```go
if first := EvaluateReadOnlyExec(store, req); !first.Allowed {
	t.Fatalf("first scoped exec was not allowed: %+v", first)
}
*now = now.Add(29*time.Minute + 59*time.Second)
if second := EvaluateReadOnlyExec(store, changedSameScope); !second.Allowed {
	t.Fatalf("same-scope exec did not extend grant: %+v", second)
}
*now = now.Add(ReadOnlyExecGrantTTL)
expired := EvaluateReadOnlyExec(store, changedSameScope)
if expired.Allowed || expired.Token == "" {
	t.Fatalf("idle grant did not expire: %+v", expired)
}
```

Run `go test ./internal/core/lifecycle/liveapproval -run 'Test(ReadOnlyExec|Approve)' -count=1`.

Expected: compile failure for the new API.

- [ ] **Step 3: Implement schema-v2 record and state transitions**

Use this record:

```go
type readonlyExecRecord struct {
	SchemaVersion      int    `json:"schema_version"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	Token              string `json:"token,omitempty"`
	RequestFingerprint string `json:"request_fingerprint,omitempty"`
	ScopeFingerprint   string `json:"scope_fingerprint"`
	ExpiresAt          string `json:"expires_at"`
}
```

Use file key `kubectl-readonly-exec-approval-<session-hash>` but lock key `approvalKey(sessionID)`. Request fingerprint includes host/session/canonical repo/cwd/tool/exact command/context/namespace. Scope fingerprint includes only normalized host/session/canonical repo/context/namespace. Reuse the length-delimited SHA-256 encoding.

Within the lock: remove only a schema-v1 record whose fingerprint matches this exact exec; refresh a matching granted scope to `now + 30m` and persist before allow; reuse matching pending token; otherwise replace only the exec record with a 10-minute pending record. Conditional validation requires token/request/scope for pending and empty token/request plus scope for granted.

- [ ] **Step 4: Make `Approve` select exactly one pending kind**

Refactor `Approve` to resolve once and lock once. Read schema-v1 one-shot and schema-v2 exec files and count valid pending records matching the exact token.

- Zero or two matches: no writes and `approvalRejected()`.
- One schema-v1 match: existing 10-minute `pending -> granted` one-shot transition and message.
- One schema-v2 match: clear token/request, retain scope, set granted with 10-minute activation expiry, and return a message stating first use within 10 minutes and 30-minute same-scope reuse.

Keep exact prompt matching and non-Codex no-op behavior unchanged.

- [ ] **Step 5: Verify and commit Task 2**

Run:

```bash
go test ./internal/core/lifecycle/liveapproval -count=1
go test -race ./internal/core/lifecycle/liveapproval -count=1
```

Expected: PASS. Schema-v1 concurrency permits exactly one consumer; schema-v2 concurrency permits both read-only calls and leaves valid expiry.

Commit exact Task 2 paths with subject `feat(liveapproval): retain scoped readonly exec grants` and Lore recording the two-file/one-lock invariant and TTL evidence.

---

### Task 3: Wire Host-Specific Lifecycle and Hook Behavior

**Files:**
- Modify: `internal/core/lifecycle/lifecycle_state.go`
- Modify: `internal/core/lifecycle/lifecycle_live_approval_test.go`
- Modify: `cmd/issueops/hookcli/hook_pre_tool_gitops_staged_test.go`

**Interfaces:**
- Consumes: `commandguard.EvaluateGitOpsKubectl`, `commandguard.KubectlLiveAccess*`, `liveapproval.Evaluate`, and `liveapproval.EvaluateReadOnlyExec`.
- Produces: unchanged `HookPreToolUseDecisionResult` and unchanged Codex/Claude hook JSON shapes.

- [ ] **Step 1: Add RED lifecycle tests**

Replace the old exec one-shot lifecycle test with a reusable scope test:

```go
func TestCodexKubectlReadOnlyExecApprovalAllowsSameScopeRepeatedly(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	firstReq := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "session-1", Tool: "Bash",
		Command: "kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user",
		EnforceGitOpsKubectl: true,
	}
	first := BuildLifecyclePreToolUseDecision(firstReq)
	token := liveApprovalTokenPattern.FindString(first.Reason)
	if first.Decision != "ask" || token == "" {
		t.Fatalf("first request = %+v", first)
	}
	if approved := ApproveCodexKubectlLiveAccess(repo, firstReq.Host, firstReq.SessionID, "승인 "+token); !approved.Handled {
		t.Fatalf("approval = %+v", approved)
	}
	if allowed := BuildLifecyclePreToolUseDecision(firstReq); allowed.Decision != "allow" {
		t.Fatalf("first allow = %+v", allowed)
	}
	secondReq := firstReq
	secondReq.Command = "kubectl --context bc-stgdev -n stg exec -c linkerd-proxy pod/gateway-2 -- curl -fsS http://localhost:4191/metrics"
	if allowed := BuildLifecyclePreToolUseDecision(secondReq); allowed.Decision != "allow" {
		t.Fatalf("same-scope allow = %+v", allowed)
	}
}
```

Add Codex unsafe cases for `env`, missing context and `sh -c`; each returns block without `AH-`, does not expose context/namespace in its reason, and creates no exec approval state. Add namespace-change requiring a new token. Add a port-forward exact one-shot regression. Keep missing-session port-forward failure and Claude native ask/no-state coverage.

Run:

```bash
go test ./internal/core/lifecycle -run 'Test(Codex|Claude)Kubectl' -count=1
```

Expected RED: safe exec is still one-shot or unsafe exec still receives a token.

- [ ] **Step 2: Route classified live access by host**

In `BuildLifecyclePreToolUseDecision`, call:

```go
evaluation := commandguard.EvaluateGitOpsKubectl(req.Tool, req.Command)
```

Apply this exact routing:

- non-ask result: preserve existing decision/reason;
- non-Codex ask: preserve native ask for every exec/port-forward classification;
- Codex + `commandguard.KubectlLiveAccessPortForward`: call existing `liveapproval.Evaluate`;
- Codex + `commandguard.KubectlLiveAccessReadOnlyExec`: call `liveapproval.EvaluateReadOnlyExec` with exact request fields plus parsed context/namespace;
- Codex + `commandguard.KubectlLiveAccessUnsafeExec`: block with a stable reason and do not call liveapproval;
- unknown Codex live-access kind: fail closed with block.

Map a liveapproval result exactly as today: `Allowed` becomes allow with empty reason, a token becomes ask for raw lifecycle JSON and Codex host rendering converts it to block, and a handled failure becomes block. Extract a private mapper only if it removes duplicated branches; do not alter public DTOs.

- [ ] **Step 3: Run lifecycle tests GREEN and race**

```bash
go test ./internal/core/lifecycle/... -count=1
go test -race ./internal/core/lifecycle/... -count=1
```

Expected: PASS.

- [ ] **Step 4: Update hook E2E tests RED-first**

In `hook_pre_tool_gitops_staged_test.go`:

- add `--context bc-stgdev -n stg` to the safe exec fixture;
- change approval context assertions from “next identical command once” to the 10-minute activation and 30-minute scope wording;
- after first no-op allow, submit a different allowlisted command/target/container in the same scope and expect another no-op;
- submit a safe command in namespace `prod` and expect a block with a new token;
- submit Codex `exec -- env` and expect block without a token;
- change the host-conflict fixture to port-forward so it still tests one-shot approval;
- retain the Claude interactive shell native ask assertion unchanged.

Run `go test ./cmd/issueops/hookcli -run 'TestRunHook.*Kubectl' -count=1` after assertion changes to observe RED, then after lifecycle wiring to obtain PASS.

- [ ] **Step 5: Verify host contracts and commit Task 3**

Run:

```bash
go test ./cmd/issueops/hookcli ./internal/adapter/hook -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
```

Expected: PASS with no golden update because schemas/tool lists are unchanged.

Commit exact Task 3 paths with subject `feat(hooks): reuse approved readonly exec scope`. Lore must state Codex/Claude divergence is intentional and port-forward remains one-shot.

---

### Task 4: Align Operations Documentation and Run the Full Gate

**Files:**
- Modify: `.issueops/CAUTIONS.md`
- Modify: `.issueops/OPERATIONS.md`

**Interfaces:**
- Consumes: completed behavior and evidence from Tasks 1-3.
- Produces: operator-visible instructions matching runtime behavior.

- [ ] **Step 1: Update the caution contract**

Replace the paragraph that calls both exec and port-forward one-shot. State all of these facts:

- Claude uses native ask for live access.
- Codex port-forward is exact-command one-shot with a 10-minute pending/granted TTL.
- Codex read-only exec requires explicit context/namespace and the exact DNS/resolver/Linkerd metrics allowlist.
- Approval activates within 10 minutes and each safe allow refreshes a 30-minute idle TTL.
- Scope is session + canonical repo + context + namespace; target/container may vary.
- Unsafe/unclassified Codex exec blocks without a token and cannot be approved around.
- State stores hashes, not raw commands or cluster identifiers.

- [ ] **Step 2: Update operator examples**

Rewrite `OPERATIONS.md`'s `Kubectl Live-Access Approval` section around these commands:

```bash
kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user
kubectl --context bc-stgdev -n stg exec -c linkerd-proxy deploy/rest-api-gateway -- curl -fsS http://localhost:4191/metrics
```

Explain first-token approval, same-scope reuse while the 30-minute idle TTL remains active, context/namespace reapproval, and one-shot port-forward. Explicitly forbid disabling `--enforce-gitops-kubectl` or using a generic shell as a workaround.

- [ ] **Step 3: Format and run focused verification**

```bash
gofmt -w internal/core/commandparse/tokens.go internal/core/commandparse/tokens_test.go internal/core/commandguard/lifecycle_command_kubectl.go internal/core/commandguard/kubectl_readonly_exec.go internal/core/commandguard/kubectl_readonly_exec_test.go internal/core/lifecycle/liveapproval/live_approval.go internal/core/lifecycle/liveapproval/live_approval_test.go internal/core/lifecycle/liveapproval/readonly_exec.go internal/core/lifecycle/liveapproval/readonly_exec_test.go internal/core/lifecycle/lifecycle_state.go internal/core/lifecycle/lifecycle_live_approval_test.go cmd/issueops/hookcli/hook_pre_tool_gitops_staged_test.go
go test ./internal/core/commandparse ./internal/core/commandguard ./internal/core/lifecycle/... ./cmd/issueops/hookcli ./internal/adapter/hook -count=1
go test -race ./internal/core/lifecycle/liveapproval ./internal/core/lifecycle ./cmd/issueops/hookcli -count=1
```

Expected: all PASS.

- [ ] **Step 4: Run repository-wide verification**

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/issueops ./cmd/issueops
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
python3 skills/atomic-commit-push/scripts/api_doc_gate.py .
```

Expected: all Go commands PASS and the API gate reports no API documentation candidates. Building `bin/issueops` must not leave a tracked diff.

- [ ] **Step 5: Run security/scope scans**

```bash
rg -n 'kubectl-readonly-exec|ReadOnlyExecGrantTTL|KubectlLiveAccessUnsafeExec' internal cmd .issueops
rg -n '다음 동일 명령 한 번|exec.*one-shot|kubectl exec.*10-minute grant' .issueops cmd internal
git diff --check
git status --short
```

Expected: new identifiers occur only in intended commandguard/lifecycle/docs paths; stale one-shot wording applies only to port-forward or historical design records; diff check is clean; only Task 4 docs are pending before commit.

- [ ] **Step 6: Commit Task 4 and verify clean history**

Commit `.issueops/CAUTIONS.md` and `.issueops/OPERATIONS.md` with subject `docs(kubectl): document scoped exec approval`. Lore lists focused/full/race/vet/build/golden/API evidence and states no push occurred.

Then run:

```bash
git status --short --branch
git log -6 --oneline
```

Expected: clean worktree, local implementation commits ahead of `origin/main`, and no remote update.

## Plan Completion Criteria

- Every spec allowlist grammar has a positive commandguard test.
- Every listed bypass family has a negative commandguard or commandparse test.
- Fake-clock tests prove 10-minute pending/activation and exact 30-minute sliding expiry.
- Same scope permits different target/container/safe argv; session/repo/context/namespace mismatches do not.
- Codex unsafe exec blocks without token or TTL refresh; Claude keeps native ask.
- Port-forward consumes one exact grant once and remains independent of exec state.
- State/error tests prove no raw command/cluster identifier leak and fail-closed behavior.
- Focused, full, race, vet, build, golden and API gates pass with a clean worktree.
- No push occurs without a new explicit user request.
