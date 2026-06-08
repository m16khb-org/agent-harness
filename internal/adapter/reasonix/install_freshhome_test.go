package reasonix

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

// Regression: a fresh install where ReasonixHome (~/.reasonix) does not yet
// exist must still create skill links and hook settings, not skip them with a
// misleading sandbox message. canWriteTo now creates the directory first.
func TestReasonixInstallerCreatesMissingHomeDir(t *testing.T) {
	root, home, cleanup := writeAdapterTestSkill(t)
	defer cleanup()

	reasonixHome := filepath.Join(home, ".reasonix-fresh-not-pre-created")
	req := core.DefaultNativeInstallRequest(root, home, "", reasonixHome, "harness")
	req.SkillNames = []string{"shared"}

	result, err := NewInstaller().Install(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected install to succeed, messages: %v", result.Messages)
	}

	hasUserSettings := false
	for _, f := range result.Files {
		if f.Kind == "reasonix_user_settings" {
			hasUserSettings = true
		}
	}
	hasSkillsLink := false
	for _, l := range result.Links {
		if l.Path == reasonixHome+"/skills/shared" {
			hasSkillsLink = true
		}
	}
	if !hasUserSettings {
		t.Error("expected reasonix_user_settings to be written when ~/.reasonix did not pre-exist")
	}
	if !hasSkillsLink {
		t.Error("expected reasonix skills link to be created when ~/.reasonix did not pre-exist")
	}
	for _, m := range result.Messages {
		if m == "reasonix home directory ~/.reasonix not writable under current sandbox; skipping skill links and hook settings" {
			t.Errorf("did not expect sandbox-skip message on a normal fresh install: %q", m)
		}
	}
}
