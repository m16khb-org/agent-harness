package installutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyHookActivationRejectsWorktreeTargetWithExactDiagnostic(t *testing.T) {
	expectedTarget := "/source/bin/agent-harness"
	observedTarget := "/source.worktrees/completed/bin/agent-harness"
	expected := hookTargetTestConfig("'" + expectedTarget + "' hook pre-tool-use --host codex")
	actual := hookTargetTestConfig("'" + observedTarget + "' hook pre-tool-use --host codex")
	raw, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = VerifyHookActivation(path, expected)
	if err == nil {
		t.Fatal("worktree hook target was accepted")
	}
	for _, evidence := range []string{"observed=" + observedTarget, "expected=" + expectedTarget} {
		if !strings.Contains(err.Error(), evidence) {
			t.Fatalf("error = %q, want %q", err, evidence)
		}
	}
}
