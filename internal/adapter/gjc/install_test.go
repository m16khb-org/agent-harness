package gjc

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestInstallerName(t *testing.T) {
	if got := NewInstaller().Name(); got != "gjc" {
		t.Fatalf("expected gjc, got %s", got)
	}
}

func TestGJCInstallerPlansSkillLinksHookShimAndPluginBundle(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	req := core.DefaultNativeInstallRequest(root, home, "", "", "harness")
	req.SkillNames = []string{"shared", "codex-only"}
	req.DryRun = true

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected installer to succeed, messages: %v", result.Messages)
	}
	if result.Host != "gjc" {
		t.Fatalf("expected host gjc, got %s", result.Host)
	}

	// Skill link for the unfiltered skill; the host-filtered skill is skipped.
	hasSkillLink := false
	for _, l := range result.Links {
		if l.Path == home+"/.gjc/agent/skills/shared" {
			hasSkillLink = true
		}
	}
	if !hasSkillLink {
		t.Error("expected gjc user skills link for 'shared'")
	}

	// Hook shim planned.
	hasHookShim := false
	for _, f := range result.Files {
		if f.Kind == "gjc_hook_shim" {
			hasHookShim = true
		}
	}
	if !hasHookShim {
		t.Error("expected gjc_hook_shim file plan")
	}
	hookSource, err := os.ReadFile(filepath.Join(root, "gjc-plugin", "hook.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"before_agent_start", "session_id", "tool_name", "tool_input", "return { block: true", "ctx.sessionManager.getSessionId()", "ctx.cwd", "\"--host\",\n\t\t\t\t\"gjc\""} {
		if !strings.Contains(string(hookSource), required) {
			t.Errorf("GJC hook shim missing %q", required)
		}
	}
	if strings.Contains(string(hookSource), `pi.on("context"`) {
		t.Error("GJC hook shim must not map every context event as a user prompt")
	}

	// Host-filtered skill skipped, plugin bundle install planned.
	hasPluginPlan := false
	for _, m := range result.Messages {
		if strings.Contains(m, "skip skill for gjc: codex-only") {
			// expected
		}
		if strings.Contains(m, "gjc plugin install") {
			hasPluginPlan = true
		}
	}
	if !hasPluginPlan {
		t.Error("expected gjc plugin install dry-run message")
	}
}

func TestGJCInstallerFailsWhenHookShimSourceMissing(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	// Remove the hook shim source to force the failure path.
	if err := os.Remove(filepath.Join(root, "gjc-plugin", "hook.ts")); err != nil {
		t.Fatal(err)
	}

	req := core.DefaultNativeInstallRequest(root, home, "", "", "harness")
	req.SkillNames = []string{"shared"}
	req.DryRun = true

	result, err := NewInstaller().Install(req)
	// Skill links still succeed, but the missing shim surfaces as a plan error,
	// so the host result reports ok=false.
	if err == nil && result.OK {
		t.Fatal("expected failure when hook shim source is missing")
	}
}

func TestGJCHookShimForwardsNativeIdentityAndEnforcesBlock(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(root, "gjc-plugin", "hook.ts")
	scriptPath := filepath.Join(root, "scripts", "smoke-gjc-native-hook.ts")
	out, err := exec.Command("bun", scriptPath, hookPath).CombinedOutput()
	if err != nil {
		t.Fatalf("GJC hook mock failed: %v\n%s", err, out)
	}
	var result struct {
		OK        bool   `json:"ok"`
		Host      string `json:"host"`
		SessionID string `json:"session_id"`
		CWD       string `json:"cwd"`
		Blocked   bool   `json:"blocked"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decode GJC hook smoke: %v\n%s", err, out)
	}
	if !result.OK || result.Host != "gjc" || result.SessionID != "gjc-session-1" || result.CWD != "/repo.worktrees/16-demo" || !result.Blocked {
		t.Fatalf("unexpected GJC hook smoke result: %+v", result)
	}
}

func TestCanWriteToCreatesFreshGJCAgentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".gjc", "agent")
	if !canWriteTo(dir) {
		t.Fatal("fresh GJC agent directory should be writable")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected fresh GJC agent directory to be created, info=%v err=%v", info, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".harness-write-test-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("write probe left temp files behind: %v", matches)
	}
}
