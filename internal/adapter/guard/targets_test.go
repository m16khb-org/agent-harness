package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuardTargetFilesUsesExplicitRelevantFiles(t *testing.T) {
	got := guardTargetFiles(t.TempDir(), GuardCheckRequest{
		Files: []string{
			" internal/core/foo.go ",
			"README.md",
			"../outside.go",
			filepath.Join(string(filepath.Separator), "tmp", "abs.go"),
			"internal/core/foo.go",
		},
	})

	want := []string{"README.md", "internal/core/foo.go"}
	if len(got) != len(want) {
		t.Fatalf("guardTargetFiles explicit length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("guardTargetFiles explicit[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGuardTargetFilesWalksRelevantFilesAndSkipsGeneratedDirs(t *testing.T) {
	root := t.TempDir()
	writeGuardTargetFile(t, root, "internal/core/foo.go")
	writeGuardTargetFile(t, root, "skills/demo/SKILL.md")
	writeGuardTargetFile(t, root, "bin/generated.go")
	writeGuardTargetFile(t, root, ".git/ignored.go")
	writeGuardTargetFile(t, root, "notes.txt")

	got := guardTargetFiles(root, GuardCheckRequest{All: true})
	want := []string{"internal/core/foo.go", "skills/demo/SKILL.md"}
	if len(got) != len(want) {
		t.Fatalf("guardTargetFiles all length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("guardTargetFiles all[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func writeGuardTargetFile(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
