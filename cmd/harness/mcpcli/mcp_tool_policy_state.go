package mcpcli

import (
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	audit "agent-harness/internal/adapter/audit"
	statestore "agent-harness/internal/adapter/outbound/state"
	policy "agent-harness/internal/adapter/policy"
	policydomain "agent-harness/internal/domain/policy"
)

func commandPolicyRequestFromArgs(args map[string]any) policydomain.CommandPolicyRequest {
	return policydomain.CommandPolicyRequest{
		WorkspaceRoot:  argmap.String(args, "workspace_root"),
		CWD:            argmap.String(args, "cwd"),
		Argv:           argmap.StringSlice(args, "argv"),
		Timeout:        argmap.StringDefault(args, "timeout", "30s"),
		EnvAllowlist:   argmap.StringSlice(args, "env_allowlist"),
		NetworkAllowed: argmap.Bool(args, "network_allowed"),
		WriteAllowed:   argmap.Bool(args, "write_allowed"),
		ShellAllowed:   argmap.Bool(args, "shell_allowed"),
		ShellReason:    argmap.String(args, "shell_reason"),
		AuditLogID:     argmap.String(args, "audit_log_id"),
	}
}

func handlePolicyStateMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "command_policy_check":
		return mcpToolPayload(policy.EvaluateCommandPolicy(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_fake_run":
		return mcpToolPayload(policy.FakeRunCommand(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_policy_audit":
		result, err := audit.AuditCommandPolicy(commandPolicyRequestFromArgs(call.Arguments))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "command_policy_audit failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_write":
		result, err := statestore.StateWrite(argmap.String(call.Arguments, "key"), argmap.String(call.Arguments, "content"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State write failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_read":
		result, err := statestore.StateRead(argmap.String(call.Arguments, "key"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State read failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_list":
		result, err := statestore.StateList()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State list failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_prune":
		maxAge, err := time.ParseDuration(argmap.String(call.Arguments, "max_age"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State prune failed", "invalid max_age: "+err.Error()))
		}
		result, err := statestore.StatePrune(maxAge, argmap.Bool(call.Arguments, "confirm"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State prune failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_doctor":
		result, err := statestore.StateDoctor()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State doctor failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_maintain":
		result, err := statestore.StateMaintain()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State maintain failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
