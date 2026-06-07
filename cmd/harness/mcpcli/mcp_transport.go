package mcpcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"agent-harness/cmd/harness/mcpcli/resources"
)

func RunMCP() error {
	if os.Getenv("HARNESS_MCP_DIRECT") == "1" {
		return ServeMCPStream(os.Stdin, os.Stdout, os.Stderr)
	}
	return RunMCPProxy()
}

func ServeMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req RPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeRPCErrorTo(output, nil, -32700, "Parse error", err.Error())
			continue
		}
		if len(req.ID) == 0 {
			handleNotificationTo(diagnostics, req)
			continue
		}
		result, rpcErr := HandleRequest(req)
		if rpcErr != nil {
			writeRPCErrorTo(output, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			continue
		}
		writeRPCResultTo(output, req.ID, result)
	}
	return scanner.Err()
}

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Code    int
	Message string
	Data    any
}

func HandleNotification(req RPCRequest) {
	handleNotificationTo(os.Stderr, req)
}

func handleNotification(req RPCRequest) {
	HandleNotification(req)
}

func handleNotificationTo(w io.Writer, req RPCRequest) {
	fmt.Fprintln(w, "agent-harness mcp notification:", req.Method)
}

func HandleRequest(req RPCRequest) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
			},
			"serverInfo":   map[string]any{"name": "agent_harness", "version": Version},
			"instructions": "This MCP endpoint is a proxy to the shared agent-harness daemon. Use harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context. For LLM Wiki workflows, install and use the upstream nvk/llm-wiki plugin instead of agent-harness.",
		}, nil
	case "tools/list":
		return map[string]any{"tools": MCPTools()}, nil
	case "tools/call":
		return HandleToolCall(req.Params)
	case "resources/list":
		return map[string]any{"resources": MCPResources()}, nil
	case "resources/read":
		return HandleResourceRead(req.Params)
	default:
		return nil, &RPCError{Code: -32601, Message: "Method not found", Data: req.Method}
	}
}

func MCPResources() []map[string]any {
	return resources.MCPResources()
}

func HandleResourceRead(params json.RawMessage) (any, *RPCError) {
	result, readErr := resources.HandleResourceRead(params, resources.Config{
		HarnessRoot:     HarnessRoot(),
		Version:         Version,
		SkillName:       skillName,
		ReadHarnessFile: ReadHarnessFile,
	})
	if readErr != nil {
		return nil, &RPCError{Code: readErr.Code, Message: readErr.Message, Data: readErr.Data}
	}
	return result, nil
}

func writeRPCResult(id json.RawMessage, result any) {
	writeRPCResultTo(os.Stdout, id, result)
}

func WriteRPCResult(id json.RawMessage, result any) {
	writeRPCResult(id, result)
}

func writeRPCError(id json.RawMessage, code int, message string, data any) {
	writeRPCErrorTo(os.Stdout, id, code, message, data)
}

func WriteRPCError(id json.RawMessage, code int, message string, data any) {
	writeRPCError(id, code, message, data)
}

func writeRPCResultTo(w io.Writer, id json.RawMessage, result any) {
	msg := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}
	b, _ := json.Marshal(msg)
	fmt.Fprintln(w, string(b))
}

func writeRPCErrorTo(w io.Writer, id json.RawMessage, code int, message string, data any) {
	msg := map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": code, "message": message, "data": data}}
	if id != nil {
		msg["id"] = json.RawMessage(id)
	} else {
		msg["id"] = nil
	}
	b, _ := json.Marshal(msg)
	fmt.Fprintln(w, string(b))
}
