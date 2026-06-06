package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForbiddenNameHitsSkipsRuntimeStateDirs(t *testing.T) {
	root := t.TempDir()
	runtimeFiles := []string{
		filepath.Join(".cache", "go-build", "log.txt"),
		filepath.Join(".claude", "hooks", ".logs", "hook-log.jsonl"),
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".codegraph", "daemon.log"),
		filepath.Join(".omc", "project-memory.json"),
		filepath.Join(".omx", "state.json"),
		filepath.Join("bin", "agent-harness"),
		filepath.Join("cache", "projects.json"),
	}
	for _, rel := range runtimeFiles {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("local m"+"16kh runtime state"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sourcePath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(sourcePath, []byte("source m"+"16kh leak"), 0o600); err != nil {
		t.Fatal(err)
	}

	hits := forbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != "AGENTS.md contains m"+"16kh" {
		t.Fatalf("expected only source hit, got %+v", hits)
	}
}

func TestCachedContractGoldenStepUsesFullGoTestEvidence(t *testing.T) {
	step := cachedContractGoldenStep(StepResult{Label: "go test", Command: "go test ./... -count=1", OK: true})
	if !step.OK || step.Label != "contract golden tests" {
		t.Fatalf("unexpected cached step: %+v", step)
	}
	if step.DurationMS != 0 {
		t.Fatalf("cached step should not report subprocess duration: %+v", step)
	}
	if !strings.Contains(step.Command, "covered by go test") || !strings.Contains(step.Stdout, "full go test suite") {
		t.Fatalf("cached step did not explain evidence source: %+v", step)
	}
}
