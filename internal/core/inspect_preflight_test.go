package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectHarnessIndexesSkillsAndDocs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	info := InspectHarness(root, root, t.TempDir(), "test-version", "atomic-commit-push")
	if !info.OK {
		t.Fatalf("InspectHarness ok=false: %+v", info)
	}
	if info.Version != "test-version" || info.HarnessRoot != root || info.TargetRepo != root {
		t.Fatalf("unexpected identity fields: %+v", info)
	}
	if !containsSkill(info.Skills, "atomic-commit-push") {
		t.Fatalf("unexpected skills: %+v", info.Skills)
	}
	if !containsDoc(info.Docs, ".agent-harness/OPERATIONS.md") {
		t.Fatalf("USAGE.md not indexed: %+v", info.Docs)
	}
	if !info.Integration.ProjectClaudeMCPConfig {
		t.Fatalf("project MCP config not detected: %+v", info.Integration)
	}
}

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
	root, err := filepath.Abs(filepath.Join("..", ".."))
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

func containsDoc(paths []string, suffix string) bool {
	for _, path := range paths {
		if filepath.ToSlash(path) == suffix || filepath.ToSlash(path) == "../../"+suffix || filepath.Base(path) == filepath.Base(suffix) {
			return true
		}
		if rel, err := filepath.Rel(filepath.Join("..", ".."), path); err == nil && filepath.ToSlash(rel) == suffix {
			return true
		}
	}
	return false
}

func containsSkill(skills []SkillInfo, name string) bool {
	for _, skill := range skills {
		if skill.Name == name && skill.HasSkillMD {
			return true
		}
	}
	return false
}
