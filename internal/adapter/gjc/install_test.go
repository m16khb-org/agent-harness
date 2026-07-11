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
	quotedPath, _ := json.Marshal("file://" + hookPath)
	script := `
const mod = await import(` + string(quotedPath) + `);
const handlers = new Map();
const messages = [];
const calls = [];
const pi = {
  on: (event, handler) => handlers.set(event, handler),
  sendMessage: (message) => messages.push(message),
};
mod.registerAgentHarnessHooks(pi, async (subcommand, payload, enforce) => {
  calls.push({ subcommand, payload, enforce });
  if (subcommand === "session-start") return { hookSpecificOutput: { additionalContext: "claim-guidance" } };
  if (subcommand === "pre-tool-use") return { decision: "block", reason: "owned-by-other-session" };
  return {};
});
const ctx = { cwd: "/repo.worktrees/16-demo", sessionManager: { getSessionId: () => "gjc-session-1" } };
await handlers.get("session_start")({ type: "session_start" }, ctx);
const blocked = await handlers.get("tool_call")({ type: "tool_call", toolName: "edit", toolCallId: "call-1", input: { path: "x.go" } }, ctx);
await handlers.get("before_agent_start")({ type: "before_agent_start", prompt: "do work" }, ctx);
if (!blocked?.block || blocked.reason !== "owned-by-other-session") throw new Error("wrong block shape: " + JSON.stringify(blocked));
if (!messages.some((m) => String(m.content).includes("claim-guidance"))) throw new Error("session guidance not relayed");
if (handlers.has("context")) throw new Error("context must not be mapped as every user prompt");
const tool = calls.find((c) => c.subcommand === "pre-tool-use");
if (tool.payload.host !== "gjc" || tool.payload.session_id !== "gjc-session-1" || tool.payload.cwd !== ctx.cwd || tool.payload.tool_name !== "edit" || tool.payload.tool_input.path !== "x.go") throw new Error("native payload missing: " + JSON.stringify(tool));
`
	scriptPath := filepath.Join(t.TempDir(), "hook-smoke.ts")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bun", scriptPath).CombinedOutput()
	if err != nil {
		t.Fatalf("GJC hook mock failed: %v\n%s", err, out)
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
