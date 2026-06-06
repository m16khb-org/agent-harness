package main

import "agent-harness/internal/core"

func handleAssistantWorkerMCPToolCall(call mcpToolCall) mcpToolOutcome {
	switch call.Name {
	case "daemon_status":
		return mcpToolPayload(daemonStatusForMCP())
	case "commit_suggest":
		result, err := core.SuggestCommit(core.CommitSuggestRequest{
			RepoRoot:   resolveTarget(stringArg(call.Arguments, "repo")),
			Staged:     boolArg(call.Arguments, "staged"),
			AgyCommand: stringArg(call.Arguments, "agy_command"),
			AgyModel:   stringArg(call.Arguments, "agy_model"),
		})
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "commit_suggest failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "lint_diagnose":
		result, err := core.DiagnoseCommand(core.LintDiagnoseRequest{
			RepoRoot:    resolveTarget(stringArg(call.Arguments, "repo")),
			CommandArgv: stringSliceArg(call.Arguments, "command_argv"),
			AgyCommand:  stringArg(call.Arguments, "agy_command"),
			AgyModel:    stringArg(call.Arguments, "agy_model"),
		})
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "lint_diagnose failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "contract_schema", "contract_check":
		return mcpToolPayload(compatibilityContract())
	case "worker_enqueue":
		result, err := core.EnqueueWorkerJob(stringArg(call.Arguments, "kind"), stringArg(call.Arguments, "payload"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "worker_enqueue failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_run_read_only":
		result, err := core.RunReadOnlyWorkerJob(
			stringArg(call.Arguments, "kind"),
			stringArg(call.Arguments, "payload"),
			core.CommandPolicyRequest{
				WorkspaceRoot: stringArg(call.Arguments, "workspace_root"),
				CWD:           stringArg(call.Arguments, "cwd"),
				Argv:          stringSliceArg(call.Arguments, "argv"),
				Timeout:       stringArgWithDefault(call.Arguments, "timeout", "30s"),
				EnvAllowlist:  stringSliceArg(call.Arguments, "env_allowlist"),
			},
		)
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "worker_run_read_only failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_status":
		result, err := core.ReadWorkerJob(stringArg(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "worker_status failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_list":
		result, err := core.ListWorkerJobs()
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "worker_list failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_cancel":
		result, err := core.CancelWorkerJob(stringArg(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "worker_cancel failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	default:
		return mcpToolOutcome{}
	}
}
