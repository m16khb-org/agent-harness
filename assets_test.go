package assets

import (
	"io/fs"
	"testing"
)

func TestEmbeddedSkillsContainKnownSkill(t *testing.T) {
	skills, err := SkillsFS()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"issueops/SKILL.md", "atomic-commit-push/SKILL.md", "workflows/install.json"} {
		if _, err := fs.Stat(skills, path); err != nil {
			t.Fatalf("embedded skills missing %s: %v", path, err)
		}
	}
}

func TestEmbeddedConfigsContainHostTemplates(t *testing.T) {
	configs, err := ConfigsFS()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"claude/hooks.settings.json", "codex/hooks.json"} {
		if _, err := fs.Stat(configs, path); err != nil {
			t.Fatalf("embedded configs missing %s: %v", path, err)
		}
	}
}
