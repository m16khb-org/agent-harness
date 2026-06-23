package mcpcli

import (
	"context"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/core"
	"agent-harness/internal/core/webfetch"
)

func handleAssistantWorkerMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "daemon_status":
		return mcpToolPayload(DaemonStatus())
	case "commit_suggest":
		result, err := core.SuggestCommit(core.CommitSuggestRequest{
			RepoRoot:   ResolveTarget(argmap.String(call.Arguments, "repo")),
			Staged:     argmap.Bool(call.Arguments, "staged"),
			AgyCommand: argmap.String(call.Arguments, "agy_command"),
			AgyModel:   argmap.String(call.Arguments, "agy_model"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "commit_suggest failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "lint_diagnose":
		result, err := core.DiagnoseCommand(core.LintDiagnoseRequest{
			RepoRoot:    ResolveTarget(argmap.String(call.Arguments, "repo")),
			CommandArgv: argmap.StringSlice(call.Arguments, "command_argv"),
			AgyCommand:  argmap.String(call.Arguments, "agy_command"),
			AgyModel:    argmap.String(call.Arguments, "agy_model"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "lint_diagnose failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "web_fetch_resilient":
		timeout, err := time.ParseDuration(argmap.StringDefault(call.Arguments, "timeout", "30s"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "invalid timeout", Data: err.Error()})
		}
		result, err := webfetch.Fetch(context.Background(), webfetch.Request{
			URL:      argmap.String(call.Arguments, "url"),
			Timeout:  timeout,
			MaxChars: argmap.Int(call.Arguments, "max_chars", 0),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "web_fetch_resilient failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "contract_schema", "contract_check":
		return mcpToolPayload(CompatibilityContract())
	case "worker_enqueue":
		result, err := core.EnqueueWorkerJob(argmap.String(call.Arguments, "kind"), argmap.String(call.Arguments, "payload"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "worker_enqueue failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_run_read_only":
		result, err := core.RunReadOnlyWorkerJob(
			argmap.String(call.Arguments, "kind"),
			argmap.String(call.Arguments, "payload"),
			core.CommandPolicyRequest{
				WorkspaceRoot: argmap.String(call.Arguments, "workspace_root"),
				CWD:           argmap.String(call.Arguments, "cwd"),
				Argv:          argmap.StringSlice(call.Arguments, "argv"),
				Timeout:       argmap.StringDefault(call.Arguments, "timeout", "30s"),
				EnvAllowlist:  argmap.StringSlice(call.Arguments, "env_allowlist"),
			},
		)
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "worker_run_read_only failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_status":
		result, err := core.ReadWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "worker_status failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_list":
		result, err := core.ListWorkerJobs()
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "worker_list failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "worker_cancel":
		result, err := core.CancelWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "worker_cancel failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
