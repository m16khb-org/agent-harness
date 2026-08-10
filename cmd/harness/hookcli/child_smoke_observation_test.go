package hookcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordChildSmokeHookEventAppendsOnlyBoundedLifecycleMarkers(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(root, "observation.json")
	t.Setenv("HARNESS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE", observationPath)

	for _, subcommand := range []string{"session-start", "pre-tool-use", "post-tool-use"} {
		if err := recordChildSmokeHookEvent(subcommand); err != nil {
			t.Fatalf("record %s: %v", subcommand, err)
		}
	}
	data, err := os.ReadFile(observationPath + ".hooks")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"event\":\"SessionStart\"}\n{\"event\":\"PreToolUse\"}\n"
	if string(data) != want {
		t.Fatalf("markers=%q want=%q", data, want)
	}
	if strings.Contains(string(data), observationPath) {
		t.Fatalf("marker leaked path: %s", data)
	}
	info, err := os.Stat(observationPath + ".hooks")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("marker mode=%v err=%v", info, err)
	}
}

func TestRecordChildSmokeHookEventRejectsNonPrivateParent(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARNESS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE", filepath.Join(root, "observation.json"))
	if err := recordChildSmokeHookEvent("session-start"); err == nil {
		t.Fatal("non-private marker parent was accepted")
	}
}

func TestContextHookBypassesChildSmokeObservation(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(root, "observation.json")
	t.Setenv("HARNESS_CHILD_SMOKE_HOOKS", "1")
	t.Setenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE", observationPath)

	if err := runHook([]string{"session-start", "--invalid-child-smoke-flag"}); err == nil {
		t.Fatal("invalid hook arguments unexpectedly passed")
	}
	if _, err := os.Stat(observationPath + ".hooks"); !os.IsNotExist(err) {
		t.Fatalf("context hook must not write a child-smoke marker: %v", err)
	}
}
