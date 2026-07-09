package mcpcli

import "encoding/json"

type MCPToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type MCPToolOutcome struct {
	Handled bool
	Direct  bool
	IsError bool
	Result  any
	Payload any
	Err     *RPCError
}

func mcpToolPayload(payload any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Payload: payload}
}

// mcpToolErrorPayload reports a tool-level FAILURE (not-found, validation,
// disk/lock, live-verify) as a normalized error tool result rather than a
// JSON-RPC protocol error: the payload is serialized as text content and the
// result is flagged isError, mirroring the CLI's {ok:false,error:...} body.
// Genuine JSON-RPC schema/param violations still use mcpToolFailure(-32602).
func mcpToolErrorPayload(payload any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, IsError: true, Payload: payload}
}

func mcpToolDirect(result any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Direct: true, Result: result}
}

func mcpToolFailure(err *RPCError) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Err: err}
}

func HandleToolCall(params json.RawMessage) (any, *RPCError) {
	var call MCPToolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid params", Data: err.Error()}
	}
	for _, handler := range []func(MCPToolCall) MCPToolOutcome{
		handleProjectMCPToolCall,
		handlePolicyStateMCPToolCall,
		handleIssueOpsMCPToolCall,
		handleWorkpoolMCPToolCall,
		handleLoopMCPToolCall,
		handleAssistantWorkerMCPToolCall,
		handleSelfLoopMCPToolCall,
	} {
		outcome := handler(call)
		if !outcome.Handled {
			continue
		}
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		if outcome.Direct {
			return outcome.Result, nil
		}
		b, _ := json.MarshalIndent(outcome.Payload, "", "  ")
		if outcome.IsError {
			return ErrorTextResult(string(b)), nil
		}
		return TextResult(string(b)), nil
	}
	return nil, &RPCError{Code: -32602, Message: "Unknown tool", Data: call.Name}
}

func TextResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// ErrorTextResult is TextResult flagged as an MCP error result (isError:true),
// the tool-result form for tool-level failures that mirror the CLI body.
func ErrorTextResult(text string) map[string]any {
	result := TextResult(text)
	result["isError"] = true
	return result
}
