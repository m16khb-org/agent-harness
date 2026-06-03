package installutil

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func embeddedSkillSource() fstest.MapFS {
	return fstest.MapFS{
		"issueops/SKILL.md":        {Data: []byte("---\nname: issueops\n---\n")},
		"issueops/references/x.md": {Data: []byte("ref")},
		"workflows/SKILL.md":       {Data: []byte("---\nname: workflows\n---\n")},
		"workflows/install.json":   {Data: []byte(`{"hosts":["codex"]}`)},
	}
}

func TestPlanHostSkillsCopiesEmbeddedSkillsAndHonorsHostFilter(t *testing.T) {
	dest := t.TempDir()
	src := embeddedSkillSource()
	names := []string{"issueops", "workflows"}

	enabled, links, messages, errs := PlanHostSkills("", src, dest, names, "claude", false)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// workflows is codex-only -> skipped for claude.
	if len(enabled) != 1 || enabled[0] != "issueops" {
		t.Fatalf("expected only issueops enabled for claude, got %v", enabled)
	}
	foundSkip := false
	for _, m := range messages {
		if m == "skip skill for claude: workflows" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatalf("expected workflows skip message, got %v", messages)
	}
	// issueops tree copied (including nested file).
	for _, rel := range []string{"issueops/SKILL.md", "issueops/references/x.md"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Fatalf("expected copied %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "workflows")); !os.IsNotExist(err) {
		t.Fatalf("workflows must not be installed for claude, stat err=%v", err)
	}
	if len(links) != 1 || !links[0].Created || links[0].Target != "embedded:issueops" {
		t.Fatalf("unexpected links: %+v", links)
	}
}

func TestPlanHostSkillsCodexGetsWorkflows(t *testing.T) {
	dest := t.TempDir()
	enabled, _, _, errs := PlanHostSkills("", embeddedSkillSource(), dest, []string{"issueops", "workflows"}, "codex", false)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(enabled) != 2 {
		t.Fatalf("expected both skills for codex, got %v", enabled)
	}
	if _, err := os.Stat(filepath.Join(dest, "workflows", "SKILL.md")); err != nil {
		t.Fatalf("workflows should be installed for codex: %v", err)
	}
}

func TestPlanHostSkillsEmbeddedDryRunWritesNothing(t *testing.T) {
	dest := t.TempDir()
	_, links, _, errs := PlanHostSkills("", embeddedSkillSource(), dest, []string{"issueops"}, "codex", true)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(links) != 1 || !links[0].WouldCreate || links[0].Created {
		t.Fatalf("dry-run should plan without creating: %+v", links)
	}
	if _, err := os.Stat(filepath.Join(dest, "issueops")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not copy files, stat err=%v", err)
	}
}
