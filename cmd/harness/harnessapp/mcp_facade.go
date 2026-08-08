package harnessapp

import (
	"agent-harness/internal/adapter/inspect"
	"agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/adapter/projectdocs"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/cmd/harness/selfworkflow"
	provenanceadapter "agent-harness/internal/adapter/outbound/issueopsprovenance"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func configureMCPCLI() {
	mcpcli.Version = version
	mcpcli.HarnessRoot = harnessRoot
	mcpcli.ResolveTarget = resolveTarget
	mcpcli.RouteProjectDocs = projectdocs.RouteProjectDocs
	mcpcli.ReadProjectDoc = projectdocs.ReadProjectDoc
	mcpcli.UpdateProjectDoc = projectdocs.UpdateProjectDoc
	mcpcli.AppendProjectDocsRecord = projectdocs.AppendProjectDocsRecord
	mcpcli.LoopStart = looprun.Start
	mcpcli.LoopRecordAttempt = looprun.RecordAttempt
	mcpcli.LoopStop = looprun.Stop
	mcpcli.LoopStatus = looprun.Status
	mcpcli.GitPreflight = preflight.GitPreflight
	mcpcli.ListSkills = inspect.ListSkills
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
}

func runMCP() error {
	configureMCPCLI()
	return mcpcli.RunMCPWithDependencies(issueOpsMCPDependencies())
}

func serveMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	configureMCPCLI()
	return mcpcli.ServeMCPStreamWithDependencies(input, output, diagnostics, issueOpsMCPDependencies())
}

func serveMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	configureMCPCLI()
	return mcpcli.ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, issueOpsMCPDependencies())
}

func mcpTools() []map[string]any {
	configureMCPCLI()
	return mcpcli.MCPTools()
}

func mcpResources() []map[string]any {
	configureMCPCLI()
	return mcpcli.MCPResources()
}

func handleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	configureMCPCLI()
	return mcpcli.HandleToolCallWithDependencies(params, issueOpsMCPDependencies())
}

func handleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	configureMCPCLI()
	return mcpcli.HandleResourceRead(params)
}

func issueOpsMCPDependencies() mcpcli.MCPDependencies {
	execution := productionIssueOpsExecutionDependencies()
	return mcpcli.MCPDependencies{
		Prepare: execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner, ReadIssue: execution.ReadIssue,
		Claim: issueOpsClaimHandler, Release: issueOpsReleaseHandler, Reseed: issueOpsReseedHandler,
		Resume: issueOpsResumeHandler, Reconcile: issueOpsReconcileHandler, Complete: issueOpsCompleteHandler,
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: mcpcli.PublicationHandlers{
			Create: issueOpsPublicationCreateHandler, Reconcile: issueOpsPublicationReconcileHandler,
		},
	}
}

func textResult(text string) map[string]any {
	return mcpcli.TextResult(text)
}
