package policy

import (
	"os"
	"strings"
	"time"

	policydomain "agent-harness/internal/domain/policy"

	"agent-harness/internal/domain/auditid"
)

func EvaluateCommandPolicy(req policydomain.CommandPolicyRequest) policydomain.CommandPolicyEvaluation {
	root := absOrOriginal(req.WorkspaceRoot)
	cwd := absOrOriginal(req.CWD)
	catalog := policyCatalogForWorkspace(root)
	canonicalRoot := canonicalPotentialPath(root)
	canonicalCWD := canonicalPotentialPath(cwd)
	argv := append([]string{}, req.Argv...)
	timeout, timeoutErr := time.ParseDuration(req.Timeout)
	if req.Timeout == "" {
		timeout = 30 * time.Second
	}
	auditID := req.AuditLogID
	if auditID == "" {
		auditID = auditid.Generate(req.WorkspaceRoot, req.CWD, req.Argv)
	}
	result := policydomain.CommandPolicyEvaluation{
		OK:             true,
		AuditLogID:     auditID,
		WorkspaceRoot:  root,
		CWD:            cwd,
		Argv:           policydomain.RedactArgv(argv),
		Timeout:        timeout.String(),
		EnvAllowlist:   policydomain.CleanEnvAllowlist(req.EnvAllowlist),
		NetworkAllowed: req.NetworkAllowed,
		WriteAllowed:   req.WriteAllowed,
		ShellAllowed:   req.ShellAllowed,
		ShellReason:    policydomain.RedactFreeform(req.ShellReason),
		Tier: policydomain.ResolveTier(policydomain.Request{
			WriteAllowed: req.WriteAllowed, NetworkAllowed: req.NetworkAllowed, ShellAllowed: req.ShellAllowed,
		}),
		DenyReasons: []string{},
		Warnings:    append([]string{}, catalog.warnings...),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	addDeny := func(reason string) {
		result.DenyReasons = append(result.DenyReasons, reason)
	}
	addWarn := func(warning string) {
		result.Warnings = append(result.Warnings, warning)
	}

	if req.WorkspaceRoot == "" {
		addDeny("workspace_root_required")
	} else if info, err := os.Stat(root); err != nil || !info.IsDir() {
		addDeny("workspace_root_not_directory")
	}
	if req.CWD == "" {
		addDeny("cwd_required")
	} else if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		addDeny("cwd_not_directory")
	}
	if root != "" && cwd != "" && !sameOrWithin(canonicalRoot, canonicalCWD) {
		addDeny("cwd_outside_workspace")
	}
	if len(argv) == 0 {
		addDeny("argv_required")
	}
	if timeoutErr != nil || timeout <= 0 {
		addDeny("invalid_timeout")
	} else if timeout > 15*time.Minute {
		addDeny("timeout_exceeds_15m")
	}
	for _, envName := range req.EnvAllowlist {
		if !policydomain.ValidEnvName(envName) {
			addDeny("invalid_env_allowlist_name")
			break
		}
	}
	for _, arg := range argv {
		if policydomain.SecretLikeArg(arg) {
			addDeny("secret_like_argument")
			break
		}
	}
	if len(argv) > 0 {
		if commandReferencesOutsideWorkspace(canonicalRoot, canonicalCWD, argv) {
			addDeny("path_outside_workspace")
		}
		if catalog.isShellCommand(argv[0]) {
			if !req.ShellAllowed {
				addDeny("shell_interpreter_not_allowed")
			} else if strings.TrimSpace(req.ShellReason) == "" {
				addDeny("shell_reason_required")
			} else {
				addWarn("shell_interpreter_exception")
			}
		}
		if catalog.commandUsesNetwork(argv) && !req.NetworkAllowed {
			addDeny("network_not_allowed")
		}
		if catalog.commandWrites(argv) && !req.WriteAllowed {
			addDeny("write_not_allowed")
		}
		if !req.WriteAllowed && !catalog.readOnlyAllowed(argv) {
			addDeny("command_not_in_read_only_allowlist")
		}
	}
	result.DenyReasons = uniqSorted(result.DenyReasons)
	result.Warnings = uniqSorted(result.Warnings)
	result.Allowed = len(result.DenyReasons) == 0
	return result
}
