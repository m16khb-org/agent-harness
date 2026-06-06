package preflight

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitPreflightDetectsCommitStyleAndSecretLikePath(t *testing.T) {
	repo := t.TempDir()
	if code, _, stderr := GitCmd(repo, "init", "-q"); code != 0 {
		t.Fatalf("git init: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := GitCmd(repo, "add", "file.txt"); code != 0 {
		t.Fatalf("git add: %s", stderr)
	}
	body := "Lore:\n- Intent: Validate core git preflight.\n- Why: Lock contract in unit tests.\n- Changes:\n  - Add fixture file.\n- Verify: go test ./internal/core\n- Risk: Low"
	if code, _, stderr := GitCmd(repo, "-c", "user.name=Core Test", "-c", "user.email=core-test@example.invalid", "commit", "-q", "-m", "docs(test): add preflight fixture", "-m", body); code != 0 {
		t.Fatalf("git commit: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("TOKEN=redacted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	result := GitPreflight(repo, root)
	if !result.OK {
		t.Fatalf("GitPreflight ok=false: %+v", result)
	}
	if len(result.SecretLikePaths) != 1 || result.SecretLikePaths[0] != ".env" {
		t.Fatalf("secret-like paths not detected: %+v", result.SecretLikePaths)
	}
	if result.CommitStyleHints["conventional_subjects"] != 1 {
		t.Fatalf("conventional subject hint wrong: %+v", result.CommitStyleHints)
	}
	if result.CommitStyleHints["lore_bodies"] != 1 {
		t.Fatalf("lore body hint wrong: %+v", result.CommitStyleHints)
	}
	if !containsString(result.Warnings, "secret_like_paths_present") {
		t.Fatalf("secret warning missing: %+v", result.Warnings)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
