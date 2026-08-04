package mcpcli

import (
	"encoding/json"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/port"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

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
	Err     *jsonrpc.Error
}

// MCPDependencies는 server 생성 시 고정된다. 요청 간 package-global dependency
// cache를 두지 않아 서로 다른 MCP server의 handler가 섞이지 않는다.
type MCPDependencies struct {
	Prepare     issueops.ExecutionPrepareHandler
	Orca        port.ExecutionOrcaProvisioner
	OrcaOwner   port.ExecutionOrcaOwnerInspector
	ReadIssue   issueops.ExecutionIssueSnapshotReadFunc
	Claim       issueops.ExecutionClaimHandler
	Release     issueops.ExecutionReleaseHandler
	Reseed      issueops.ExecutionReseedHandler
	Resume      issueops.ExecutionResumeHandler
	Reconcile   issueops.ExecutionReconcileHandler
	Complete    issueops.ExecutionCompleteHandler
	Publication issueops.RemotePublicationHandlers
	Provenance  provenanceport.Observer
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

func mcpToolFailure(err *jsonrpc.Error) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Err: err}
}

func newProtocolError(code int64, message string, data any) *jsonrpc.Error {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	return &jsonrpc.Error{Code: code, Message: message, Data: raw}
}

func HandleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{})
}

// HandleToolCallWithReleaseHandler keeps the server dependency immutable per
// call instead of caching a composition-root handler in package state.
func HandleToolCallWithReleaseHandler(params json.RawMessage, release issueops.ExecutionReleaseHandler) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{Release: release})
}

func HandleToolCallWithDependencies(params json.RawMessage, deps MCPDependencies) (any, *jsonrpc.Error) {
	var call MCPToolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, newProtocolError(-32602, "Invalid params", err.Error())
	}
	for _, handler := range []func(MCPToolCall) MCPToolOutcome{
		handleProjectMCPToolCall,
		handlePolicyStateMCPToolCall,
		func(call MCPToolCall) MCPToolOutcome {
			return handleIssueOpsMCPToolCallWithDependencies(call, deps)
		},
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
	return nil, newProtocolError(-32602, "Unknown tool", call.Name)
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
