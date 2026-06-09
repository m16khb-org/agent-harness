package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var sdkServer *mcp.Server

func initSDKServer() *mcp.Server {
	if sdkServer != nil {
		return sdkServer
	}
	server := mcp.NewServer(
		&mcp.Implementation{Name: "agent_harness", Version: Version},
		&mcp.ServerOptions{
			Instructions: "This MCP endpoint is a proxy to the shared agent-harness daemon. Use harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context. For LLM Wiki workflows, install and use the upstream nvk/llm-wiki plugin instead of agent-harness.",
			Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	)
	registerAllTools(server)
	registerAllResources(server)
	sdkServer = server
	return server
}

func sdkToolHandler(groupHandler func(MCPToolCall) MCPToolOutcome, toolName string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		outcome := groupHandler(MCPToolCall{Name: toolName, Arguments: args})
		if outcome.Err != nil {
			return nil, fmt.Errorf("%s: %v", outcome.Err.Message, outcome.Err.Data)
		}
		if outcome.Direct {
			if dm, ok := outcome.Result.(map[string]any); ok {
				if contentArr, ok := dm["content"].([]any); ok {
					result := &mcp.CallToolResult{}
					for _, c := range contentArr {
						if cm, ok := c.(map[string]any); ok {
							if cm["type"] == "text" {
								if text, ok := cm["text"].(string); ok {
									result.Content = append(result.Content, &mcp.TextContent{Text: text})
								}
							}
						}
					}
					return result, nil
				}
			}
			b, _ := json.MarshalIndent(outcome.Result, "", "  ")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
		}
		b, _ := json.MarshalIndent(outcome.Payload, "", "  ")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
	}
}

func registerAllTools(server *mcp.Server) {
	for _, toolMap := range MCPTools() {
		name, _ := toolMap["name"].(string)
		desc, _ := toolMap["description"].(string)
		inputSchema := toolMap["inputSchema"]
		server.AddTool(
			&mcp.Tool{Name: name, Description: desc, InputSchema: inputSchema},
			sdkToolHandler(resolveHandlerGroup(name), name),
		)
	}
}

func resolveHandlerGroup(name string) func(MCPToolCall) MCPToolOutcome {
	for _, pair := range []struct {
		names []string
		fn    func(MCPToolCall) MCPToolOutcome
	}{
		{projectToolNames(), handleProjectMCPToolCall},
		{policyStateToolNames(), handlePolicyStateMCPToolCall},
		{issueOpsToolNames(), handleIssueOpsMCPToolCall},
		{assistantWorkerToolNames(), handleAssistantWorkerMCPToolCall},
		{selfLoopToolNames(), handleSelfLoopMCPToolCall},
	} {
		for _, n := range pair.names {
			if n == name {
				return pair.fn
			}
		}
	}
	return func(call MCPToolCall) MCPToolOutcome {
		return MCPToolOutcome{Handled: true, Err: &RPCError{Code: -32602, Message: "Unknown tool", Data: call.Name}}
	}
}

func projectToolNames() []string {
	return []string{"harness_inspect", "atomic_commit_preflight", "commit_policy", "skill_manifest", "docs_index", "project_docs_route", "project_docs_bootstrap_plan", "project_docs_read", "project_docs_update", "project_docs_record", "api_doc_review", "api_doc_static_check"}
}
func policyStateToolNames() []string {
	return []string{"command_policy_check", "command_fake_run", "command_policy_audit", "state_write", "state_read", "state_list", "state_prune", "state_doctor", "state_migrate"}
}
func issueOpsToolNames() []string {
	return []string{"issueops_start", "issueops_status", "issueops_record_intent", "issueops_review_design", "issueops_link_issue", "issueops_link_plan", "issueops_link_worktree", "issueops_prepare_worktree_tools", "issueops_link_child", "issueops_link_related", "issueops_prepare_branch", "issueops_add_feedback", "issueops_add_decision", "issueops_mark_issue_updated", "issueops_set_phase", "issueops_verify_remote_artifact", "issueops_remote_score", "issueops_pr_readiness", "issueops_cleanup_status", "issueops_force_release", "issueops_cleanup_stale", "issueops_remote_create_issue", "issueops_remote_create_pr", "issueops_remote_sync_graph"}
}
func assistantWorkerToolNames() []string {
	return []string{"daemon_status", "commit_suggest", "lint_diagnose", "contract_schema", "contract_check", "worker_enqueue", "worker_run_read_only", "worker_status", "worker_list", "worker_cancel"}
}
func selfLoopToolNames() []string {
	return []string{"self_augment", "self_augment_lesson", "self_verify", "self_verify_candidates", "self_verify_history", "self_verify_compare", "self_verify_promote", "self_augment_history", "self_augment_compare", "self_augment_promote"}
}

func registerAllResources(server *mcp.Server) {
	for _, r := range MCPResources() {
		uri, _ := r["uri"].(string)
		name, _ := r["name"].(string)
		desc, _ := r["description"].(string)
		mime, _ := r["mimeType"].(string)
		server.AddResource(
			&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: mime},
			sdkResourceHandler(),
		)
	}
}

func sdkResourceHandler() mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		params, err := json.Marshal(map[string]string{"uri": req.Params.URI})
		if err != nil {
			return nil, fmt.Errorf("marshal resource params: %w", err)
		}
		result, readErr := HandleResourceRead(params)
		if readErr != nil {
			return nil, fmt.Errorf("%s: %v", readErr.Message, readErr.Data)
		}
		if m, ok := result.(map[string]any); ok {
			if contents, ok := m["contents"].([]any); ok && len(contents) > 0 {
				if cm, ok := contents[0].(map[string]any); ok {
					uri, _ := cm["uri"].(string)
					mimeType, _ := cm["mimeType"].(string)
					text, _ := cm["text"].(string)
					return &mcp.ReadResourceResult{
						Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: text}},
					}, nil
				}
			}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)}},
		}, nil
	}
}

// serveMCPStreamSDK runs the MCP server using the official go-sdk IOTransport.
// Used for bidirectional connections (net.Conn from daemon accept loop).
func serveMCPStreamSDK(input io.Reader, output io.Writer) error {
	server := initSDKServer()
	rwc, ok := input.(io.ReadWriteCloser)
	if !ok {
		return server.Run(context.Background(), &mcp.IOTransport{
			Reader: io.NopCloser(input),
			Writer: writeCloser{output},
		})
	}
	return server.Run(context.Background(), &mcp.IOTransport{
		Reader: rwc,
		Writer: rwc,
	})
}

type writeCloser struct{ io.Writer }

func (writeCloser) Close() error { return nil }
