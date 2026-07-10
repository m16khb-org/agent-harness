package gjc

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAdapterTestSkill(t *testing.T) (root, home string, cleanup func()) {
	t.Helper()
	root = t.TempDir()
	home = filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(root, "skills", "shared")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// host-filtered skill that GJC must skip
	codexOnlyDir := filepath.Join(root, "skills", "codex-only")
	if err := os.MkdirAll(codexOnlyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexOnlyDir, "SKILL.md"), []byte("# Codex Only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexOnlyDir, "install.json"), []byte(`{"hosts":["codex"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// hook shim source expected at <root>/gjc-plugin/hook.ts
	if err := os.MkdirAll(filepath.Join(root, "gjc-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	hookSource, err := os.ReadFile(filepath.Join("..", "..", "..", "gjc-plugin", "hook.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gjc-plugin", "hook.ts"), hookSource, 0o644); err != nil {
		t.Fatal(err)
	}
	// GJC plugin bundle manifest at the repo root so the adapter plans
	// `gjc plugin install` instead of skipping.
	if err := os.WriteFile(filepath.Join(root, pluginManifestName), []byte(`{"kind":"gajae-code-plugin","name":"agent-harness","version":"0.1.0"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, home, func() {}
}
