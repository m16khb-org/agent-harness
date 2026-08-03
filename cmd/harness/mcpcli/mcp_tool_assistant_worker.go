package mcpcli

import (
	"context"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	webfetchoutbound "agent-harness/internal/adapter/outbound/webfetch"
	webfetchcontract "agent-harness/internal/contract/webfetch"
	"agent-harness/internal/core"
)

func handleAssistantWorkerMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "daemon_status":
		return mcpToolPayload(DaemonStatus())
	case "commit_suggest":
		result, err := core.SuggestCommit(core.CommitSuggestRequest{
			RepoRoot: ResolveTarget(argmap.String(call.Arguments, "repo")),
			Staged:   argmap.Bool(call.Arguments, "staged"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "commit_suggest failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "lint_diagnose":
		result, err := core.DiagnoseCommand(core.LintDiagnoseRequest{
			RepoRoot:    ResolveTarget(argmap.String(call.Arguments, "repo")),
			CommandArgv: argmap.StringSlice(call.Arguments, "command_argv"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "lint_diagnose failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "web_fetch_resilient":
		timeout, err := time.ParseDuration(argmap.StringDefault(call.Arguments, "timeout", "30s"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "invalid timeout", err.Error()))
		}
		result, err := webfetchoutbound.Fetch(context.Background(), webfetchcontract.Request{
			URL:      argmap.String(call.Arguments, "url"),
			Timeout:  timeout,
			MaxChars: argmap.Int(call.Arguments, "max_chars", 0),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "web_fetch_resilient failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "contract_schema", "contract_check":
		return mcpToolPayload(CompatibilityContract())
	case "worker_enqueue":
		result, err := core.EnqueueWorkerJob(argmap.String(call.Arguments, "kind"), argmap.String(call.Arguments, "payload"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_enqueue failed", err.Error()))
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
			return mcpToolFailure(newProtocolError(-32000, "worker_run_read_only failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_status":
		result, err := core.ReadWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_status failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_list":
		result, err := core.ListWorkerJobs()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_list failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_cancel":
		result, err := core.CancelWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_cancel failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
