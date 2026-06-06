package main

import (
	"time"

	"agent-harness/internal/core"
)

func commandPolicyRequestFromArgs(args map[string]any) core.CommandPolicyRequest {
	return core.CommandPolicyRequest{
		WorkspaceRoot:  stringArg(args, "workspace_root"),
		CWD:            stringArg(args, "cwd"),
		Argv:           stringSliceArg(args, "argv"),
		Timeout:        stringArgWithDefault(args, "timeout", "30s"),
		EnvAllowlist:   stringSliceArg(args, "env_allowlist"),
		NetworkAllowed: boolArg(args, "network_allowed"),
		WriteAllowed:   boolArg(args, "write_allowed"),
		ShellAllowed:   boolArg(args, "shell_allowed"),
		ShellReason:    stringArg(args, "shell_reason"),
		AuditLogID:     stringArg(args, "audit_log_id"),
	}
}

func handlePolicyStateMCPToolCall(call mcpToolCall) mcpToolOutcome {
	switch call.Name {
	case "command_policy_check":
		return mcpToolPayload(core.EvaluateCommandPolicy(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_fake_run":
		return mcpToolPayload(core.FakeRunCommand(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_policy_audit":
		result, err := core.AuditCommandPolicy(commandPolicyRequestFromArgs(call.Arguments))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "command_policy_audit failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_write":
		result, err := core.StateWrite(stringArg(call.Arguments, "key"), stringArg(call.Arguments, "content"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "State write failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_read":
		result, err := core.StateRead(stringArg(call.Arguments, "key"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "State read failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_list":
		result, err := core.StateList()
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "State list failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_prune":
		maxAge, err := time.ParseDuration(stringArg(call.Arguments, "max_age"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "State prune failed", Data: "invalid max_age: " + err.Error()})
		}
		result, err := core.StatePrune(maxAge, boolArg(call.Arguments, "confirm"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32602, Message: "State prune failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_doctor":
		result, err := core.StateDoctor()
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "State doctor failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "state_migrate":
		result, err := core.StateMigrate(boolArg(call.Arguments, "confirm"))
		if err != nil {
			return mcpToolFailure(&rpcError{Code: -32000, Message: "State migrate failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	default:
		return mcpToolOutcome{}
	}
}
