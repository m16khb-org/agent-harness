package reasonix

import (
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/port"
)

func TestInstallerName(t *testing.T) {
	if got := NewInstaller().Name(); got != "reasonix" {
		t.Fatalf("expected reasonix, got %s", got)
	}
}

func TestReasonixInstallerDefaultsToUserScopeOnly(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	reasonixHome := home
	req := core.DefaultNativeInstallRequest(root, home, "", reasonixHome, "harness")
	req.SkillNames = []string{"shared"}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected installer to succeed")
	}
	if result.Host != "reasonix" {
		t.Fatalf("expected host reasonix, got %s", result.Host)
	}

	hasUserSettings := false
	hasUserMCP := false
	hasSkillsLink := false
	hasHooksTemplate := false
	hasMCPTemplate := false

	for _, f := range result.Files {
		switch f.Kind {
		case "reasonix_user_settings":
			hasUserSettings = true
		case "reasonix_user_mcp_config":
			hasUserMCP = true
		case "reasonix_hooks_template":
			hasHooksTemplate = true
		case "reasonix_project_mcp_template":
			hasMCPTemplate = true
		}
	}
	for _, l := range result.Links {
		if l.Path == reasonixHome+"/skills/shared" {
			hasSkillsLink = true
		}
	}

	if !hasUserSettings {
		t.Error("expected reasonix_user_settings file")
	}
	if !hasUserMCP {
		t.Error("expected reasonix_user_mcp_config file")
	}
	if !hasSkillsLink {
		t.Error("expected reasonix user skills link")
	}
	if !hasHooksTemplate {
		t.Error("expected reasonix_hooks_template file")
	}
	if !hasMCPTemplate {
		t.Error("expected reasonix_project_mcp_template file")
	}
}

func TestReasonixInstallerProjectLocalIsExplicit(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	reasonixHome := home
	req := core.DefaultNativeInstallRequest(root, home, "", reasonixHome, "harness")
	req.SkillNames = []string{"shared"}
	req.ProjectLocal = true
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatal("expected installer to succeed")
	}

	hasProjectSkills := false
	hasProjectSettings := false
	for _, l := range result.Links {
		if l.Path == root+"/.reasonix/skills/shared" {
			hasProjectSkills = true
		}
	}
	for _, f := range result.Files {
		if f.Path == root+"/.reasonix/settings.json" {
			hasProjectSettings = true
		}
	}
	if !hasProjectSkills {
		t.Error("expected project-local .reasonix/skills/shared link")
	}
	if !hasProjectSettings {
		t.Error("expected project-local .reasonix/settings.json file")
	}
}

func TestReasonixInstallerMergesLifecycleHooksIdempotently(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	reasonixHome := home
	req := core.DefaultNativeInstallRequest(root, home, "", reasonixHome, "harness")
	req.SkillNames = []string{"shared"}

	_, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}
	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	if !result.OK {
		t.Fatal("expected second install to succeed")
	}

	var settingsFile port.InstallFile
	for _, f := range result.Files {
		if f.Kind == "reasonix_user_settings" {
			settingsFile = f
			break
		}
	}
	if settingsFile.Written {
		t.Error("expected idempotent settings merge (no rewrite)")
	}
}

func TestReasonixInstallerSkipsCodexOnlySkills(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	reasonixHome := home
	req := core.DefaultNativeInstallRequest(root, home, "", reasonixHome, "harness")
	req.SkillNames = []string{"shared", "codex-only"}

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasCodexSkill := false
	for _, l := range result.Links {
		if l.Path == reasonixHome+"/skills/codex-only" {
			hasCodexSkill = true
		}
	}
	if hasCodexSkill {
		t.Error("expected codex-only skill to be skipped for reasonix")
	}

	hasSkipMessage := false
	for _, m := range result.Messages {
		if m == "skip skill for reasonix: codex-only" {
			hasSkipMessage = true
		}
	}
	if !hasSkipMessage {
		t.Error("expected skip message for codex-only skill")
	}
}