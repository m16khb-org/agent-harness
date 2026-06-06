package policy

func CommandPolicySummary() map[string]any {
	return map[string]any{
		"ok":                    true,
		"mode":                  "policy_check_fake_runner_read_only_runner",
		"executes_commands":     true,
		"executes_read_only":    true,
		"default_timeout":       "30s",
		"max_timeout":           "15m",
		"required_fields":       []string{"workspace_root", "cwd", "argv", "timeout", "env_allowlist", "network_allowed", "write_allowed", "audit_log_id"},
		"default_denials":       []string{"cwd_outside_workspace", "path_outside_workspace", "shell_interpreter_not_allowed", "network_not_allowed", "write_not_allowed", "command_not_in_read_only_allowlist", "secret_like_argument"},
		"read_only_examples":    [][]string{{"git", "status", "--short"}, {"git", "diff", "--stat"}, {"rg", "pattern", "."}},
		"catalog":               commandPolicyCatalog(),
		"write_requires_flag":   true,
		"network_requires_flag": true,
		"tiers":                 commandPolicyTiers(),
	}
}

// commandPolicyTiers documents the request→tier ladder so hosts can render the
// privilege envelope without re-deriving it. Order is ascending by privilege.

func commandPolicyTiers() []map[string]any {
	return []map[string]any{
		{"name": PolicyTierReadOnly, "requires": []string{}, "rationale": policyTierRationale(PolicyTierReadOnly)},
		{"name": PolicyTierWorkspaceWrite, "requires": []string{"write_allowed"}, "rationale": policyTierRationale(PolicyTierWorkspaceWrite)},
		{"name": PolicyTierNetworkAccess, "requires": []string{"network_allowed"}, "rationale": policyTierRationale(PolicyTierNetworkAccess)},
		{"name": PolicyTierShellException, "requires": []string{"shell_allowed", "shell_reason"}, "rationale": policyTierRationale(PolicyTierShellException)},
	}
}
