package mcpcli

import (
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/adapter/core"
)

func commandPolicyRequestFromArgs(args map[string]any) core.CommandPolicyRequest {
	return core.CommandPolicyRequest{
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
		return mcpToolPayload(core.EvaluateCommandPolicy(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_fake_run":
		return mcpToolPayload(core.FakeRunCommand(commandPolicyRequestFromArgs(call.Arguments)))
	case "command_policy_audit":
		result, err := core.AuditCommandPolicy(commandPolicyRequestFromArgs(call.Arguments))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "command_policy_audit failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_write":
		result, err := core.StateWrite(argmap.String(call.Arguments, "key"), argmap.String(call.Arguments, "content"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State write failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_read":
		result, err := core.StateRead(argmap.String(call.Arguments, "key"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State read failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_list":
		result, err := core.StateList()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State list failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_prune":
		maxAge, err := time.ParseDuration(argmap.String(call.Arguments, "max_age"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State prune failed", "invalid max_age: "+err.Error()))
		}
		result, err := core.StatePrune(maxAge, argmap.Bool(call.Arguments, "confirm"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "State prune failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_doctor":
		result, err := core.StateDoctor()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State doctor failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "state_maintain":
		result, err := core.StateMaintain()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "State maintain failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
