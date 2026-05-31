package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/verifywork\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	result := buildVerifyWork(repo, false, []string{"git", "status", "--short"})
	if !result.OK {
		t.Fatalf("expected verify-work result to be ok, warnings=%v", result.Warnings)
	}

	assertEvidenceItem(t, result.EvidenceMatrix, "git_preflight", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "guard_check", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "passed")

	if len(result.Evidence) == 0 {
		t.Fatalf("expected legacy evidence strings to remain populated")
	}
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "test", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "build", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "vet", "./..."})
}

func TestBuildVerifyWorkSerializesEmptySuggestedCommands(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := buildVerifyWork(repo, false, nil)
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "skipped")
	if len(result.SuggestedCommands) != 0 {
		t.Fatalf("expected no suggested commands without project signals, got %#v", result.SuggestedCommands)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal verify-work result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal verify-work payload: %v", err)
	}
	commands, ok := decoded["suggested_commands"].([]any)
	if !ok {
		t.Fatalf("expected suggested_commands to serialize as an array, payload=%s", string(payload))
	}
	if len(commands) != 0 {
		t.Fatalf("expected empty suggested_commands array, got %#v", commands)
	}
}

func TestBuildVerifyWorkMarksDeniedCommandFailed(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := buildVerifyWork(repo, false, []string{"sh", "-c", "true"})
	if result.OK {
		t.Fatalf("expected denied command to fail verify-work")
	}
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "failed")
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func assertEvidenceItem(t *testing.T, items []VerifyWorkEvidenceItem, name string, status string) {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			if item.Status != status {
				t.Fatalf("evidence item %s status=%s, want %s", name, item.Status, status)
			}
			if item.Summary == "" {
				t.Fatalf("evidence item %s has empty summary", name)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %s in %#v", name, items)
}

func assertSuggestedCommand(t *testing.T, commands []VerifyWorkSuggestedCommand, want []string) {
	t.Helper()
	for _, command := range commands {
		if equalStringSlices(command.Command, want) {
			if command.Name == "" || command.Reason == "" {
				t.Fatalf("suggested command %#v must include name and reason", command)
			}
			return
		}
	}
	t.Fatalf("missing suggested command %v in %#v", want, commands)
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
