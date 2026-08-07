package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"agent-harness/cmd/harness/mcpcli/resources"
	mcpadapter "agent-harness/internal/adapter/mcp"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func initSDKServer(deps MCPDependencies) *mcp.Server {
	return initSDKServerWithDiagnostics(deps, io.Discard)
}

func initSDKServerWithDiagnostics(deps MCPDependencies, diagnostics io.Writer) *mcp.Server {
	if diagnostics == nil {
		diagnostics = io.Discard
	}
	server := mcp.NewServer(
		&mcp.Implementation{Name: "agent_harness", Version: Version},
		sdkServerOptionsWithDiagnostics(diagnostics),
	)
	registerAllTools(server, deps)
	registerAllResources(server)
	return server
}

func sdkServerOptions() *mcp.ServerOptions {
	return sdkServerOptionsWithDiagnostics(io.Discard)
}

func sdkServerOptionsWithDiagnostics(diagnostics io.Writer) *mcp.ServerOptions {
	return &mcp.ServerOptions{
		Instructions: "This MCP endpoint is a proxy to the shared agent-harness daemon. Use harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context. External wiki or knowledge-base workflows belong to their own separately installed servers, not agent-harness.",
		Logger:       slog.New(slog.NewTextHandler(diagnostics, nil)),
		Capabilities: &mcp.ServerCapabilities{},
	}
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
			return nil, outcome.Err
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
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			IsError: outcome.IsError,
		}, nil
	}
}

func registerAllTools(server *mcp.Server, deps MCPDependencies) {
	for _, toolMap := range MCPTools() {
		name, _ := toolMap["name"].(string)
		desc, _ := toolMap["description"].(string)
		inputSchema := toolMap["inputSchema"]
		if name == "issueops_execution" {
			server.AddTool(
				&mcp.Tool{Name: name, Description: desc, InputSchema: inputSchema},
				issueOpsExecutionSDKToolHandler(deps),
			)
			continue
		}
		handler := resolveHandlerGroup(name)
		server.AddTool(
			&mcp.Tool{Name: name, Description: desc, InputSchema: inputSchema},
			sdkToolHandler(handler, name),
		)
	}
}

func issueOpsExecutionSDKToolHandler(deps MCPDependencies) mcp.ToolHandler {
	return sdkToolHandler(func(call MCPToolCall) MCPToolOutcome {
		return handleIssueOpsMCPToolCallWithDependencies(call, deps)
	}, "issueops_execution")
}

// handlerGroupLookup maps each dispatch group to its handler function.
// New tools only need to be added to the adapter catalog DispatchMap; this
// lookup stays stable as long as no new handler group is introduced.
var handlerGroupLookup = map[mcpadapter.DispatchGroup]func(MCPToolCall) MCPToolOutcome{
	mcpadapter.DispatchProject:         handleProjectMCPToolCall,
	mcpadapter.DispatchPolicyState:     handlePolicyStateMCPToolCall,
	mcpadapter.DispatchIssueOps:        handleIssueOpsMCPToolCall,
	mcpadapter.DispatchLoop:            handleLoopMCPToolCall,
	mcpadapter.DispatchAssistantWorker: handleAssistantWorkerMCPToolCall,
	mcpadapter.DispatchSelfLoop:        handleSelfLoopMCPToolCall,
}

func resolveHandlerGroup(name string) func(MCPToolCall) MCPToolOutcome {
	dm := mcpadapter.DispatchMap()
	group, ok := dm[name]
	if !ok {
		return func(call MCPToolCall) MCPToolOutcome {
			return MCPToolOutcome{Handled: true, Err: newProtocolError(-32602, "Unknown tool", call.Name)}
		}
	}
	if fn, ok := handlerGroupLookup[group]; ok {
		return fn
	}
	return func(call MCPToolCall) MCPToolOutcome {
		return MCPToolOutcome{Handled: true, Err: newProtocolError(-32602, "Unknown tool", call.Name)}
	}
}

func MCPResources() []map[string]any {
	return resources.MCPResources()
}

func HandleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	result, readErr := resources.HandleResourceRead(params, resources.Config{
		HarnessRoot:     HarnessRoot(),
		Version:         Version,
		SkillName:       skillName,
		ReadHarnessFile: ReadHarnessFile,
	})
	if readErr != nil {
		return nil, newProtocolError(int64(readErr.Code), readErr.Message, readErr.Data)
	}
	return result, nil
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
			return nil, readErr
		}
		if m, ok := result.(map[string]any); ok {
			if contents, ok := m["contents"].([]any); ok && len(contents) > 0 {
				if cm, ok := contents[0].(map[string]any); ok {
					return sdkReadResourceResult(cm), nil
				}
			}
			if contents, ok := m["contents"].([]map[string]any); ok && len(contents) > 0 {
				return sdkReadResourceResult(contents[0]), nil
			}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)}},
		}, nil
	}
}

func sdkReadResourceResult(content map[string]any) *mcp.ReadResourceResult {
	uri, _ := content["uri"].(string)
	mimeType, _ := content["mimeType"].(string)
	text, _ := content["text"].(string)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: text}},
	}
}

// serveMCPStreamSDK runs both split stdio and bidirectional daemon connections
// through the official go-sdk IOTransport.
func serveMCPStreamSDK(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	server := initSDKServerWithDiagnostics(deps, diagnostics)
	if rwc, ok := input.(io.ReadWriteCloser); ok && io.Writer(rwc) == output {
		return server.Run(ctx, &mcp.IOTransport{Reader: rwc, Writer: rwc})
	}
	reader := io.ReadCloser(io.NopCloser(input))
	if closer, ok := input.(io.ReadCloser); ok {
		reader = closer
	}
	return server.Run(ctx, &mcp.IOTransport{Reader: reader, Writer: writeCloser{output}})
}

type writeCloser struct{ io.Writer }

func (writeCloser) Close() error { return nil }
