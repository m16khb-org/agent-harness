package main

import "encoding/json"

type mcpToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type mcpToolOutcome struct {
	Handled bool
	Direct  bool
	Result  any
	Payload any
	Err     *rpcError
}

func mcpToolPayload(payload any) mcpToolOutcome {
	return mcpToolOutcome{Handled: true, Payload: payload}
}

func mcpToolDirect(result any) mcpToolOutcome {
	return mcpToolOutcome{Handled: true, Direct: true, Result: result}
}

func mcpToolFailure(err *rpcError) mcpToolOutcome {
	return mcpToolOutcome{Handled: true, Err: err}
}

func handleToolCall(params json.RawMessage) (any, *rpcError) {
	var call mcpToolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{Code: -32602, Message: "Invalid params", Data: err.Error()}
	}
	for _, handler := range []func(mcpToolCall) mcpToolOutcome{
		handleProjectMCPToolCall,
		handlePolicyStateMCPToolCall,
		handleIssueOpsMCPToolCall,
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
		return textResult(string(b)), nil
	}
	return nil, &rpcError{Code: -32602, Message: "Unknown tool", Data: call.Name}
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}
