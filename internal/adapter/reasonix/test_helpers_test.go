package reasonix

import (
	"os"
	"path/filepath"
	"runtime"
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
	reasonixHome := filepath.Join(home, ".reasonix")
	if err := os.MkdirAll(reasonixHome, 0o755); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, configSubDir(), "reasonix")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, home, func() {}
}

func configSubDir() string {
	if runtime.GOOS == "darwin" {
		return filepath.Join("Library", "Application Support")
	}
	return ".config"
}
