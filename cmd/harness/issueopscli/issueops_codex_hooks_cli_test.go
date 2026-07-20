package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestIssueOpsHandoffCodexHooksListOwnsExactCodexArgvAndJSONL(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	record := coordinatorPreparingCLIRecord(t)
	t.Chdir(record.Repo)
	argsLog, stdinLog, cwdLog, envLog := installCodexHooksListFake(t, codexHooksListHappyScript)
	t.Setenv("HARNESS_TEST_REVIEW_SECRET", "token=process-boundary-secret")

	response, err := json.Marshal(map[string]any{
		"id": 2,
		"result": map[string]any{"data": []any{map[string]any{
			"cwd": record.WorktreePath,
			"hooks": []any{map[string]any{
				"key": "pre-tool-use", "enabled": true, "trustStatus": "trusted", "currentHash": "sha256:abc",
				"sourcePath": filepath.Join(record.WorktreePath, "hooks.json"), "command": "agent-harness hook pre-tool-use [redacted]",
			}},
			"warnings": []any{},
			"errors":   []any{},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	setCodexHooksListFakeInput(t, "response", string(response))

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", record.ID, "--json"})
	})
	if !strings.Contains(out, `"cwd": "`+record.WorktreePath+`"`) || !strings.Contains(out, `"trustStatus": "trusted"`) {
		t.Fatalf("bounded helper output lost hook trust evidence: %s", out)
	}

	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := "-C\n" + record.WorktreePath + "\napp-server\n--stdio\n"
	if string(args) != wantArgs {
		t.Fatalf("codex argv = %q, want %q", string(args), wantArgs)
	}
	stdin, err := os.ReadFile(stdinLog)
	if err != nil {
		t.Fatal(err)
	}
	wantStdin := strings.Join([]string{
		`{"method":"initialize","id":1,"params":{"clientInfo":{"name":"agent_harness","title":"agent-harness","version":"1"}}}`,
		`{"method":"initialized","params":{}}`,
		`{"method":"hooks/list","id":2,"params":{"cwds":["` + record.WorktreePath + `"]}}`,
		"",
	}, "\n")
	if string(stdin) != wantStdin {
		t.Fatalf("codex JSONL transcript = %q, want %q", string(stdin), wantStdin)
	}
	cwd, err := os.ReadFile(cwdLog)
	if err != nil {
		t.Fatal(err)
	}
	if !sameCodexHooksListPath(strings.TrimSpace(string(cwd)), record.WorktreePath) {
		t.Fatalf("Codex app-server cwd = %q, want exact worker root %q", strings.TrimSpace(string(cwd)), record.WorktreePath)
	}
	env, err := os.ReadFile(envLog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(env), "process-boundary-secret") {
		t.Fatalf("Codex app-server inherited a non-allowlisted secret: %s", env)
	}
	auditPath := filepath.Join(stateDir, "audit", "process-execution.jsonl")
	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read Codex process audit: %v", err)
	}
	if !strings.Contains(string(audit), `"cwd":"`+record.WorktreePath+`"`) || !strings.Contains(string(audit), `"env_policy":"codex_hooks_list_v1"`) || !strings.Contains(string(audit), `"outcome":"success"`) || !strings.Contains(out, `"audit_log_id":`) {
		t.Fatalf("Codex process audit missing fixed boundary evidence: %s", audit)
	}
	if strings.Contains(string(audit), "process-boundary-secret") {
		t.Fatalf("Codex process audit leaked a secret: %s", audit)
	}
}

func TestIssueOpsHandoffCodexHooksListSupportsReadyWorkspaceBeforeHandoff(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := coordinatorPreparingCLIRecord(t)
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State:              "ready",
		WorkspaceEpoch:     "workspace-epoch-1",
		Driver:             "orca",
		Agent:              "codex",
		CoordinatorRoot:    record.Repo,
		WorkerRoot:         record.WorktreePath,
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator"},
		BaseHead:           strings.Repeat("a", 40),
	}
	record.ExecutionHandoff = nil
	written, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(written.Repo)
	installCodexHooksListFake(t, codexHooksListHappyScript)
	response, err := json.Marshal(map[string]any{
		"id": 2,
		"result": map[string]any{"data": []any{map[string]any{
			"cwd": written.WorktreePath, "hooks": []any{}, "warnings": []any{}, "errors": []any{},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	setCodexHooksListFakeInput(t, "response", string(response))

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", written.ID, "--json"})
	})
	if !strings.Contains(out, `"cwd": "`+written.WorktreePath+`"`) {
		t.Fatalf("ready workspace hook evidence used the wrong worker root: %s", out)
	}
}

func TestIssueOpsHandoffCodexHooksListFailsClosedOnMalformedOrUnboundedResponse(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := coordinatorPreparingCLIRecord(t)
	t.Chdir(record.Repo)

	validResult := func(data []any) string {
		t.Helper()
		payload, err := json.Marshal(map[string]any{"id": 2, "result": map[string]any{"data": data}})
		if err != nil {
			t.Fatal(err)
		}
		return string(payload)
	}
	entry := func(cwd string, hooks, warnings, errors []any) map[string]any {
		return map[string]any{"cwd": cwd, "hooks": hooks, "warnings": warnings, "errors": errors}
	}
	manyHooks := make([]any, 1025)
	for i := range manyHooks {
		manyHooks[i] = map[string]any{"key": "h", "enabled": true}
	}
	manyWarnings := make([]any, 257)
	for i := range manyWarnings {
		manyWarnings[i] = "warning"
	}
	manyErrors := make([]any, 257)
	for i := range manyErrors {
		manyErrors[i] = "error"
	}
	unknownFieldResult, err := json.Marshal(map[string]any{
		"id": 2,
		"result": map[string]any{
			"data":   []any{entry(record.WorktreePath, nil, nil, nil)},
			"future": strings.Repeat("x", 4097),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, scenario, response, want, forbid string
	}{
		{name: "malformed json", scenario: "before", response: `{`, want: "malformed"},
		{name: "trailing json", scenario: "before", response: `{"id":1,"result":{}} trailing`, want: "malformed"},
		{name: "json rpc error", scenario: "before", response: `{"id":1,"error":{"code":-1,"message":"token=rpc-review-secret"}}`, want: "json-rpc error", forbid: "rpc-review-secret"},
		{name: "id two before initialize", scenario: "before", response: validResult([]any{entry(record.WorktreePath, nil, nil, nil)}), want: "before initialize"},
		{name: "unexpected response id", scenario: "before", response: `{"id":"token=review-secret-value","result":{}}`, want: "unexpected response id", forbid: "review-secret-value"},
		{name: "duplicate initialize response", scenario: "duplicate-id-1", want: "duplicate response id"},
		{name: "duplicate hooks response", scenario: "duplicate-id-2", response: validResult([]any{entry(record.WorktreePath, nil, nil, nil)}), want: "duplicate response id"},
		{name: "notification object limit", scenario: "notification-limit", want: "object limit"},
		{name: "jsonl line limit", scenario: "line-limit", want: "line limit"},
		{name: "stdout byte limit", scenario: "stdout-limit", want: "stdout limit"},
		{name: "stderr diagnostic limit", scenario: "stderr-limit", response: `{`, want: "malformed"},
		{name: "stderr secret redaction", scenario: "secret-stderr", response: `{`, want: "malformed", forbid: "stderr-review-secret"},
		{name: "wrong cwd", scenario: "after", response: validResult([]any{entry(record.WorktreePath+"-other", nil, nil, nil)}), want: "exact worker cwd"},
		{name: "multiple cwd entries", scenario: "after", response: validResult([]any{entry(record.WorktreePath, nil, nil, nil), entry(record.WorktreePath, nil, nil, nil)}), want: "exactly one cwd"},
		{name: "hook limit", scenario: "after", response: validResult([]any{entry(record.WorktreePath, manyHooks, nil, nil)}), want: "hook limit"},
		{name: "warning limit", scenario: "after", response: validResult([]any{entry(record.WorktreePath, nil, manyWarnings, nil)}), want: "warning limit"},
		{name: "error limit", scenario: "after", response: validResult([]any{entry(record.WorktreePath, nil, nil, manyErrors)}), want: "error limit"},
		{name: "string limit", scenario: "after", response: validResult([]any{entry(record.WorktreePath, []any{map[string]any{"key": strings.Repeat("x", 4097)}}, nil, nil)}), want: "string limit"},
		{name: "unknown result string limit", scenario: "after", response: string(unknownFieldResult), want: "string limit"},
		{name: "secret-like map key", scenario: "after", response: validResult([]any{entry(record.WorktreePath, []any{map[string]any{"api_key=review-secret-value": true}}, nil, nil)}), want: "unsafe map key", forbid: "review-secret-value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installCodexHooksListFake(t, codexHooksListFailureScript)
			setCodexHooksListFakeInput(t, "scenario", tt.scenario)
			setCodexHooksListFakeInput(t, "response", tt.response)
			out, err := captureStdoutAndErrorForIssueOps(t, func() error {
				return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", record.ID, "--json"})
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("scenario %q error = %v, want %q; stdout=%s", tt.scenario, err, tt.want, out)
			}
			if len(err.Error()) > 40*1024 {
				t.Fatalf("scenario %q returned unbounded diagnostics: %d bytes", tt.scenario, len(err.Error()))
			}
			if tt.forbid != "" && strings.Contains(err.Error(), tt.forbid) {
				t.Fatalf("scenario %q reflected attacker-controlled secret: %v", tt.scenario, err)
			}
		})
	}
}

func TestIssueOpsHandoffCodexHooksListRejectsPositionalArguments(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := coordinatorPreparingCLIRecord(t)
	t.Chdir(record.Repo)
	installCodexHooksListFake(t, codexHooksListHappyScript)
	setCodexHooksListFakeInput(t, "response", `{"id":2,"result":{"data":[{"cwd":"`+record.WorktreePath+`","hooks":[],"warnings":[],"errors":[]}]}}`)

	_, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", record.ID, "--json", "extra"})
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "positional") {
		t.Fatalf("unexpected positional argument must fail closed, got %v", err)
	}
}

func TestIssueOpsHandoffCodexHooksListRejectsUnboundedOrReflectiveIDs(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	tests := []struct {
		name, id, secret string
	}{
		{name: "secret-bearing", id: "io-token=review-secret-value", secret: "review-secret-value"},
		{name: "oversized", id: "io-" + strings.Repeat("a", 8192)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdoutAndErrorForIssueOps(t, func() error {
				return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", tt.id, "--json"})
			})
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "canonical issueops id") {
				t.Fatalf("invalid helper id error = %v, want non-reflective canonical-id denial", err)
			}
			if len(out) > 1024 || len(err.Error()) > 1024 {
				t.Fatalf("invalid helper id produced unbounded output: stdout=%d error=%d", len(out), len(err.Error()))
			}
			if tt.secret != "" && (strings.Contains(out, tt.secret) || strings.Contains(err.Error(), tt.secret)) {
				t.Fatalf("invalid helper id reflected a secret: stdout=%s error=%v", out, err)
			}
		})
	}
}

func TestIssueOpsHandoffCodexHooksListRejectsEachNonPristineRecordField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*issueopsmodel.IssueOpsExecutionHandoff)
	}{
		{
			name: "pending operation",
			mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) {
				h.PendingOperation = &issueopsmodel.IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationDispatch, StartedAt: "2026-07-18T00:00:00Z", ExpectedAssigneeHandle: "term-1", DeliveryMode: "inject"}
			},
		},
		{
			name: "cleanup only",
			mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) {
				h.CleanupOnly = &issueopsmodel.IssueOpsOrcaCleanupArtifact{Kind: "worktree", ID: "wt-1", Reason: "test"}
			},
		},
		{
			name: "worker session",
			mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) {
				h.WorkerSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "worker-session"}
			},
		},
		{
			name: "result",
			mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) {
				h.Result = &issueopsmodel.IssueOpsExecutionHandoffResult{Outcome: handoff.OutcomeCompleted, FinalHead: strings.Repeat("a", 40)}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			record := coordinatorPreparingCLIRecord(t)
			t.Chdir(record.Repo)
			tt.mutate(record.ExecutionHandoff)
			if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
				if !strings.Contains(err.Error(), "state-incompatible fields") {
					t.Fatalf("non-pristine field was rejected for an unrelated reason: %v", err)
				}
				return
			}
			_, err := captureStdoutAndErrorForIssueOps(t, func() error {
				return runIssueOps([]string{"handoff", "codex-hooks-list", "--id", record.ID, "--json"})
			})
			if err == nil {
				t.Fatal("non-pristine coordinator record reached Codex process launch")
			}
		})
	}
}

func coordinatorPreparingCLIRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	record := handoffCLIRecord(t, handoff.StateDispatched)
	record.ExecutionHandoff.State = handoff.StateCoordinatorPreparing
	record.ExecutionHandoff.DeliveryMode = ""
	written, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func installCodexHooksListFake(t *testing.T, script string) (string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "args.log")
	stdinLog := filepath.Join(dir, "stdin.log")
	cwdLog := filepath.Join(dir, "cwd.log")
	envLog := filepath.Join(dir, "env.log")
	path := filepath.Join(dir, "codex")
	script = strings.ReplaceAll(script, "__HARNESS_TEST_CODEX_FIXTURE_DIR__", shellSingleQuote(dir))
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"response", "scenario"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HARNESS_TEST_CODEX_FIXTURE_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsLog, stdinLog, cwdLog, envLog
}

func setCodexHooksListFakeInput(t *testing.T, name, value string) {
	t.Helper()
	dir := os.Getenv("HARNESS_TEST_CODEX_FIXTURE_DIR")
	if dir == "" {
		t.Fatal("Codex hooks/list fake is not installed")
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

const codexHooksListHappyScript = `#!/bin/sh
set -eu
fixture_dir=__HARNESS_TEST_CODEX_FIXTURE_DIR__
printf '%s\n' "$@" > "$fixture_dir/args.log"
pwd > "$fixture_dir/cwd.log"
/usr/bin/env > "$fixture_dir/env.log"
IFS= read -r line
printf '%s\n' "$line" > "$fixture_dir/stdin.log"
printf '%s\n' '{"id":1,"result":{}}'
IFS= read -r line
printf '%s\n' "$line" >> "$fixture_dir/stdin.log"
IFS= read -r line
printf '%s\n' "$line" >> "$fixture_dir/stdin.log"
/bin/cat "$fixture_dir/response"
printf '\n'
`

const codexHooksListFailureScript = `#!/bin/sh
set -eu
fixture_dir=__HARNESS_TEST_CODEX_FIXTURE_DIR__
scenario=$(/bin/cat "$fixture_dir/scenario")
response=$(/bin/cat "$fixture_dir/response")
IFS= read -r first
case "$scenario" in
  before)
    printf '%s\n' "$response"
    ;;
  duplicate-id-1)
    printf '%s\n' '{"id":1,"result":{}}' '{"id":1,"result":{}}'
    IFS= read -r _
    IFS= read -r _
    ;;
  notification-limit)
    i=0
    while [ "$i" -lt 257 ]; do
      printf '%s\n' '{"method":"codex/event","params":{}}'
      i=$((i + 1))
    done
    ;;
  line-limit)
    printf '{"method":"'
    chunk='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
    i=0
    while [ "$i" -lt 513 ]; do
      printf '%s' "$chunk"
      i=$((i + 1))
    done
    printf '"}\n'
    ;;
  stdout-limit)
    chunk='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
    i=0
    while [ "$i" -lt 210 ]; do
      printf '{"method":"event","params":{"value":"'
      j=0
      while [ "$j" -lt 5 ]; do printf '%s' "$chunk"; j=$((j + 1)); done
      printf '"}}\n'
      i=$((i + 1))
    done
    ;;
  stderr-limit)
    chunk='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
    i=0
    while [ "$i" -lt 64 ]; do printf '%s' "$chunk" >&2; i=$((i + 1)); done
    printf '%s\n' "$response"
    ;;
  secret-stderr)
    printf '%s\n' 'token=stderr-review-secret' >&2
    printf '%s\n' "$response"
    ;;
  *)
    printf '%s\n' '{"id":1,"result":{}}'
    IFS= read -r _
    IFS= read -r _
    printf '%s\n' "$response"
    if [ "$scenario" = duplicate-id-2 ]; then
      printf '%s\n' "$response"
    fi
    ;;
esac
`
