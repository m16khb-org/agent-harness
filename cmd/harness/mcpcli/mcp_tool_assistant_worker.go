package mcpcli

import (
	"context"
	"time"

	"agent-harness/cmd/harness/mcpcli/argmap"
	commitsuggest "agent-harness/internal/adapter/commitsuggest"
	lintdiagnose "agent-harness/internal/adapter/lintdiagnose"
	webfetchoutbound "agent-harness/internal/adapter/outbound/webfetch"
	policy "agent-harness/internal/adapter/policy"
	worker "agent-harness/internal/adapter/worker"
	webfetchcontract "agent-harness/internal/contract/webfetch"
)

func handleAssistantWorkerMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "daemon_status":
		return mcpToolPayload(DaemonStatus())
	case "commit_suggest":
		result, err := commitsuggest.SuggestCommit(commitsuggest.CommitSuggestRequest{
			RepoRoot: ResolveTarget(argmap.String(call.Arguments, "repo")),
			Staged:   argmap.Bool(call.Arguments, "staged"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "commit_suggest failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "lint_diagnose":
		result, err := lintdiagnose.DiagnoseCommand(lintdiagnose.LintDiagnoseRequest{
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
		result, err := worker.EnqueueWorkerJob(argmap.String(call.Arguments, "kind"), argmap.String(call.Arguments, "payload"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_enqueue failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_run_read_only":
		result, err := worker.RunReadOnlyWorkerJob(
			argmap.String(call.Arguments, "kind"),
			argmap.String(call.Arguments, "payload"),
			policy.CommandPolicyRequest{
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
		result, err := worker.ReadWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_status failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_list":
		result, err := worker.ListWorkerJobs()
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_list failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "worker_cancel":
		result, err := worker.CancelWorkerJob(argmap.String(call.Arguments, "id"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "worker_cancel failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
