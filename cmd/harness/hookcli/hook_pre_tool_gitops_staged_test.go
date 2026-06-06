package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesGitOpsKubectl(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
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
		"cwd":       repo,
		"tool_name": "Bash",
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
}

func TestRunHookPreToolUseAsksForBroadStagedChecks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check apps libs"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
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

func TestRunHookPreToolUseCodexHostAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
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
	if obj["decision"] != nil {
		t.Fatalf("Codex host ask must not use legacy decision field, got %+v", obj)
	}
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "ask" {
		t.Fatalf("expected Codex PreToolUse ask decision, got %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
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
