package hookcli

import (
	"strings"
	"testing"

	coreinstall "agent-harness/internal/core/install"
)

func TestRunHookPreToolUseBlocksCachedWorktreeRuntimeForBothHosts(t *testing.T) {
	previous := DiagnoseCurrentNativeRuntime
	DiagnoseCurrentNativeRuntime = func() (coreinstall.NativeRuntimeDiagnostic, error) {
		return coreinstall.NativeRuntimeDiagnostic{
			Stale: true, Observed: "/source.worktrees/completed/bin/agent-harness",
			Expected: "/source/bin/agent-harness", RestartRequired: true,
		}, nil
	}
	t.Cleanup(func() { DiagnoseCurrentNativeRuntime = previous })

	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			obj := runHookCapture(t, `{"cwd":"/source","tool_name":"Bash","tool_input":{"command":"pwd"}}`, func() error {
				return runHookPreToolUse([]string{"--host", host})
			})
			reason := cachedRuntimeTestReason(obj, host)
			for _, evidence := range []string{
				"observed=/source.worktrees/completed/bin/agent-harness",
				"expected=/source/bin/agent-harness",
				"restart the host session",
			} {
				if !strings.Contains(reason, evidence) {
					t.Fatalf("%s reason = %q, want %q", host, reason, evidence)
				}
			}
		})
	}
}

func cachedRuntimeTestReason(output map[string]any, host string) string {
	if host == "claude" {
		specific, _ := output["hookSpecificOutput"].(map[string]any)
		reason, _ := specific["permissionDecisionReason"].(string)
		return reason
	}
	reason, _ := output["reason"].(string)
	return reason
}
