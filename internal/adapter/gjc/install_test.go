package gjc

import (
	"os"
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
