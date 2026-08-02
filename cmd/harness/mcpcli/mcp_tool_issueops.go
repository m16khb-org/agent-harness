package mcpcli

import "agent-harness/internal/core/issueops"

// IssueOps v1 intentionally exposes one MCP action tool. Its action field
// selects the same execution DTO used by the CLI subcommands.
var issueOpsMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"issueops_execution": handleMCPIssueOpsExecution,
}

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithDependencies(call, MCPDependencies{})
}

func handleIssueOpsMCPToolCallWithReleaseHandler(call MCPToolCall, release issueops.ExecutionReleaseHandler) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithDependencies(call, MCPDependencies{Release: release})
}

func handleIssueOpsMCPToolCallWithDependencies(call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	if call.Name == "issueops_execution" {
		return handleMCPIssueOpsExecutionWithDependencies(call.Arguments, deps)
	}
	handler, ok := issueOpsMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}
