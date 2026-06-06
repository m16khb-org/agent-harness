package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForbiddenNameHitsAllowsCurrentOwnerAndLicense(t *testing.T) {
	root := t.TempDir()
	// LICENSE copyright line is legitimate and the file is skipped entirely.
	if err := os.WriteFile(filepath.Join(root, "LICENSE"), []byte("Copyright (c) 2026 m"+"16khb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Clone URL with the current owner handle in docs must not flag.
	planPath := filepath.Join(root, "plan.md")
	if err := os.WriteFile(planPath, []byte("git clone git@github.com:m"+"16khb/agent-harness.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A genuine legacy needle (without the trailing b) must still be detected.
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("legacy m"+"16kh leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := forbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != "AGENTS.md contains m"+"16kh" {
		t.Fatalf("expected only the genuine legacy hit, got %+v", hits)
	}
}

func TestForbiddenNameHitsSkipsWorktreeGitPointer(t *testing.T) {
	root := t.TempDir()
	legacyPath := "gitdir: /Users/" + "m" + "16" + "kh" + "b/Workspace/agent-harness/.git/worktrees/example\n"
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte(legacyPath), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hits := forbiddenNameHits(root); len(hits) != 0 {
		t.Fatalf("worktree .git pointer should be skipped, got %+v", hits)
	}
}
