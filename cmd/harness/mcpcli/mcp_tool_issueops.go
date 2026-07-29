package mcpcli

import "agent-harness/internal/core/issueops"

// IssueOps v1 intentionally exposes one MCP action tool. Its action field
// selects the same execution DTO used by the CLI subcommands.
var issueOpsMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"issueops_execution": handleMCPIssueOpsExecution,
}

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithReleaseHandler(call, nil)
}

func handleIssueOpsMCPToolCallWithReleaseHandler(call MCPToolCall, release issueops.ExecutionReleaseHandler) MCPToolOutcome {
	if call.Name == "issueops_execution" {
		return handleMCPIssueOpsExecutionWithReleaseHandler(call.Arguments, release)
	}
	handler, ok := issueOpsMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}
