package mcpcli

import "context"

// IssueOps v1 intentionally exposes one MCP action tool. Its action field
// selects the same execution DTO used by the CLI subcommands.
var issueOpsMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"issueops_execution": handleMCPIssueOpsExecution,
}

func handleIssueOpsMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithContext(context.Background(), call, MCPDependencies{})
}

func handleIssueOpsMCPToolCallWithDependencies(call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	return handleIssueOpsMCPToolCallWithContext(context.Background(), call, deps)
}

func handleIssueOpsMCPToolCallWithContext(
	ctx context.Context,
	call MCPToolCall,
	deps MCPDependencies,
) MCPToolOutcome {
	if call.Name == "issueops_execution" {
		return handleMCPIssueOpsExecutionWithContext(ctx, call.Arguments, deps)
	}
	handler, ok := issueOpsMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}
