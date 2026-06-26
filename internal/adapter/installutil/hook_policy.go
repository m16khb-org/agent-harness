package installutil

import "strings"

// PreToolUseEnforcementFlags lists the harness enforcement gates attached to the
// PreToolUse lifecycle hook. Both host installers append this identical set;
// host-specific differences (such as Claude's --host flag) are applied
// separately by each command builder. Adding or removing a gate happens here
// once instead of in every host command builder.
func PreToolUseEnforcementFlags() string {
	return strings.Join([]string{
		"--enforce-worktree",
		"--enforce-korean-remote-artifacts",
		"--enforce-vcs-issue-linking",
		"--enforce-staged-checks",
		"--enforce-gitops-kubectl",
	}, " ")
}

// StopEnforcementFlags lists the harness enforcement gates attached to the Stop
// lifecycle hook, shared identically across hosts.
func StopEnforcementFlags() string {
	return strings.Join([]string{
		"--enforce-numbered-next-actions",
		"--enforce-engelbart-canvas-sections",
		"--relay-next-action-judgement",
	}, " ")
}

// HookGroupContainsAgentHarness reports whether a host hook group already holds
// an agent-harness lifecycle hook command, so installers replace their own prior
// hook rather than duplicating it while leaving third-party hooks untouched.
func HookGroupContainsAgentHarness(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "harness") && strings.Contains(cmd, " hook ") {
			return true
		}
	}
	return false
}
