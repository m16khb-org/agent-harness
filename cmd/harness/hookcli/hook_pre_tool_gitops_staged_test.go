package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesGitOpsKubectl(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-1",
		"tool_name":  "Bash",
		"tool_input": map[string]any{
			"command": `kubectl patch deployment/api -n prod -p '{"spec":{"replicas":1}}'`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected direct kubectl mutation to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "GitOps") || !strings.Contains(reason, "kubectl") {
		t.Fatalf("expected GitOps kubectl reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-1",
		"tool_name":  "Bash",
		"tool_input": map[string]any{
			"command": `kubectl port-forward svc/api 8080:80 -n prod`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl", "--json"})
	})
	if obj["decision"] != "ask" {
		t.Fatalf("expected kubectl live access to ask for confirmation, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	token := regexp.MustCompile(`AH-[A-HJ-NP-Z2-9]{6}`).FindString(reason)
	if token == "" {
		t.Fatalf("expected approval token, got %+v", obj)
	}
	repeated := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl", "--json"})
	})
	if repeatedReason, _ := repeated["reason"].(string); !strings.Contains(repeatedReason, token) {
		t.Fatalf("repeated pending request changed token: first=%q repeated=%+v", token, repeated)
	}
}

func TestRunHookPreToolUseAsksForBroadStagedChecks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check apps libs"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-1",
		"tool_name":  "Bash",
		"tool_input": map[string]any{
			"command": `npm run lint:check`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-staged-checks", "--json"})
	})
	if obj["decision"] != "ask" {
		t.Fatalf("expected broad staged check to ask for confirmation, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "staged") || !strings.Contains(reason, "apps/libs") {
		t.Fatalf("expected staged check reason, got %q", reason)
	}
}

func TestRunHookPreToolUseCodexHostBlocksKubectlLiveAccessAsk(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-1",
		"tool_name":  "Bash",
		"tool_input": map[string]any{
			"command": `kubectl port-forward svc/api 8080:80 -n prod`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("Codex host ask fallback must block, got %+v", obj)
	}
	if _, ok := obj["hookSpecificOutput"]; ok {
		t.Fatalf("Codex host ask fallback must not emit unsupported hookSpecificOutput, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "승인 AH-") {
		t.Fatalf("Codex block did not carry one-shot approval token: %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-1",
		"tool_name":  "Bash",
		"tool_input": map[string]any{
			"command": `kubectl exec -it pod/api-0 -- sh`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-gitops-kubectl"})
	})
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "ask" {
		t.Fatalf("expected Claude PreToolUse ask decision, got %+v", obj)
	}
}

func TestRunHookCodexKubectlLiveApprovalFlow(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	command := "kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user"
	preToolPayload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-approval",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	first := runHookCapture(t, string(preToolPayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	firstReason, _ := first["reason"].(string)
	token := regexp.MustCompile(`AH-[A-HJ-NP-Z2-9]{6}`).FindString(firstReason)
	if first["decision"] != "block" || token == "" {
		t.Fatalf("first request did not block with token: %+v", first)
	}

	promptPayload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-approval",
		"prompt":     "승인 " + token,
	})
	if err != nil {
		t.Fatal(err)
	}
	approved := runHookCapture(t, string(promptPayload), func() error {
		return runHookUserPrompt([]string{"--host", "codex"})
	})
	if ctx := hookAdditionalContext(approved); !strings.Contains(ctx, "10분") || !strings.Contains(ctx, "30분") ||
		strings.Contains(ctx, command) || strings.Contains(ctx, "karpathy-first") {
		t.Fatalf("unexpected approval context: %q", ctx)
	}

	allowed := runHookCapture(t, string(preToolPayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	if len(allowed) != 0 {
		t.Fatalf("approved exact request was not host no-op allow: %+v", allowed)
	}
	sameScopePayload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-approval",
		"tool_name":  "shell",
		"tool_input": map[string]any{"command": "kubectl --context bc-stgdev -n stg exec -c linkerd-proxy pod/gateway-2 -- curl -fsS http://localhost:4191/metrics"},
	})
	if err != nil {
		t.Fatal(err)
	}
	reused := runHookCapture(t, string(sameScopePayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	if len(reused) != 0 {
		t.Fatalf("same-scope request was not host no-op allow: %+v", reused)
	}

	changedScopePayload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-approval",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "kubectl --context bc-stgdev -n prod exec deploy/rest-api-gateway -- getent hosts grpc-user"},
	})
	if err != nil {
		t.Fatal(err)
	}
	changed := runHookCapture(t, string(changedScopePayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	changedReason, _ := changed["reason"].(string)
	changedToken := regexp.MustCompile(`AH-[A-HJ-NP-Z2-9]{6}`).FindString(changedReason)
	if changed["decision"] != "block" || changedToken == "" || changedToken == token {
		t.Fatalf("changed scope did not block with a new token: first=%q changed=%+v", token, changed)
	}
}

func TestRunHookCodexUnsafeKubectlExecBlocksWithoutToken(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-unsafe",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "kubectl --context bc-stgdev -n stg exec deploy/api -- env"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	reason, _ := got["reason"].(string)
	if got["decision"] != "block" || strings.Contains(reason, "AH-") || strings.Contains(reason, "bc-stgdev") || strings.Contains(reason, "stg") {
		t.Fatalf("unsafe exec did not block safely: %+v", got)
	}
}

func TestRunHookUserPromptHostConflictCannotGrantKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	preToolPayload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"session_id": "session-conflict",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "kubectl --context bc-stgdev -n stg port-forward svc/api 8080:80"},
	})
	if err != nil {
		t.Fatal(err)
	}
	first := runHookCapture(t, string(preToolPayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	reason, _ := first["reason"].(string)
	token := regexp.MustCompile(`AH-[A-HJ-NP-Z2-9]{6}`).FindString(reason)
	if token == "" {
		t.Fatalf("missing pending token: %+v", first)
	}

	conflictingPrompt, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"host":       "codex",
		"session_id": "session-conflict",
		"prompt":     "승인 " + token,
	})
	if err != nil {
		t.Fatal(err)
	}
	runHookCapture(t, string(conflictingPrompt), func() error {
		return runHookUserPrompt([]string{"--host", "claude"})
	})

	retry := runHookCapture(t, string(preToolPayload), func() error {
		return runHookPreToolUse([]string{"--host", "codex", "--enforce-gitops-kubectl"})
	})
	retryReason, _ := retry["reason"].(string)
	if retry["decision"] != "block" || !strings.Contains(retryReason, token) {
		t.Fatalf("host-conflicting prompt granted live access: %+v", retry)
	}
}
