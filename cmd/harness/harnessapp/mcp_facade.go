package harnessapp

import (
	"encoding/json"
	"fmt"
	"io"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/core"
)

type rpcRequest = mcpcli.RPCRequest
type rpcError = mcpcli.RPCError
type mcpToolCall = mcpcli.MCPToolCall
type mcpToolOutcome = mcpcli.MCPToolOutcome

func configureMCPCLI() {
	mcpcli.Version = version
	mcpcli.HarnessRoot = harnessRoot
	mcpcli.ResolveTarget = resolveTarget
	mcpcli.ReadHarnessFile = readHarnessFile
	mcpcli.InspectHarness = func(repo string) any {
		return inspectHarness(repo)
	}
	mcpcli.RunMCPProxy = runMCPProxy
	mcpcli.DaemonStatus = func() any {
		return daemonStatusForMCP()
	}
	mcpcli.CompatibilityContract = func() any {
		return compatibilityContract()
	}
	mcpcli.SelfVerify = func(iterations int, baseSeed int64, targetScore float64, verbose bool) (selfworkflow.SelfAugmentResult, error) {
		result, err := selfVerify(iterations, baseSeed, targetScore, verbose)
		if err != nil && isSelfVerificationGateError(err) {
			return result, fmt.Errorf("%w: %w", mcpcli.ErrSelfVerificationGateFailed, err)
		}
		return result, err
	}
	mcpcli.PrepareIssueOpsWorktreeTools = func(record core.IssueOpsRecord) (any, error) {
		return prepareIssueOpsWorktreeTools(record)
	}
	mcpcli.VerifyIssueOpsChildIssueBeforeLink = verifyIssueOpsChildIssueBeforeLink
	mcpcli.IssueOpsCleanupMerged = issueOpsCleanupMerged
	mcpcli.VerifyIssueOpsRemoteArtifactLive = verifyIssueOpsRemoteArtifactLive
}

func runMCP() error {
	configureMCPCLI()
	return mcpcli.RunMCP()
}

func serveMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	configureMCPCLI()
	return mcpcli.ServeMCPStream(input, output, diagnostics)
}

func mcpTools() []map[string]any {
	configureMCPCLI()
	return mcpcli.MCPTools()
}

func mcpResources() []map[string]any {
	configureMCPCLI()
	return mcpcli.MCPResources()
}

func handleToolCall(params json.RawMessage) (any, *rpcError) {
	configureMCPCLI()
	return mcpcli.HandleToolCall(params)
}

func handleResourceRead(params json.RawMessage) (any, *rpcError) {
	configureMCPCLI()
	return mcpcli.HandleResourceRead(params)
}

func handleRequest(req rpcRequest) (any, *rpcError) {
	configureMCPCLI()
	return mcpcli.HandleRequest(req)
}

func textResult(text string) map[string]any {
	return mcpcli.TextResult(text)
}

func writeRPCResult(id json.RawMessage, result any) {
	mcpcli.WriteRPCResult(id, result)
}

func writeRPCError(id json.RawMessage, code int, message string, data any) {
	mcpcli.WriteRPCError(id, code, message, data)
}
