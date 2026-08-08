package issueopspreparation

import (
	"context"
	"errors"
	"fmt"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

type OrcaDependencies struct {
	Provisioner   port.ExecutionOrcaProvisioner
	ValidateProbe func(context.Context, preparationcontract.ProbeRequest) (string, error)
	HydrateLaunch func(context.Context, preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error)
}

type OrcaGatewayAdapter struct {
	dependencies OrcaDependencies
}

func NewOrcaGateway(dependencies OrcaDependencies) *OrcaGatewayAdapter {
	return &OrcaGatewayAdapter{dependencies: dependencies}
}

func (adapter *OrcaGatewayAdapter) Probe(ctx context.Context, request preparationcontract.ProbeRequest) (preparationcontract.ProbeResult, error) {
	if adapter == nil || adapter.dependencies.Provisioner == nil {
		return preparationcontract.ProbeResult{Code: "orca_adapter_unavailable"}, fmt.Errorf("Orca provisioner is unavailable")
	}
	result, err := adapter.dependencies.Provisioner.Probe(ctx, port.ExecutionOrcaProbeRequest{
		Repo: request.Repo, Host: request.Host, Model: request.Model, Effort: request.Effort,
		Provider: request.Provider, Issue: request.Issue, Marker: request.Marker,
	})
	mapped := preparationcontract.ProbeResult{Available: result.Available, Ready: result.Ready, Code: result.Code}
	if err != nil {
		return mapped, fmt.Errorf("Orca probe failed: %w", err)
	}
	if !result.Available || !result.Ready {
		return mapped, err
	}
	if adapter.dependencies.ValidateProbe == nil {
		return preparationcontract.ProbeResult{Available: result.Available, Code: "orca_branch_precheck_unavailable"}, fmt.Errorf("Orca branch precheck is unavailable")
	}
	code, err := adapter.dependencies.ValidateProbe(ctx, request)
	if err != nil {
		mapped.Ready = false
		mapped.Code = code
		if mapped.Code == "" {
			mapped.Code = "orca_branch_precheck_failed"
		}
		return mapped, err
	}
	return mapped, nil
}

func (adapter *OrcaGatewayAdapter) Inspect(ctx context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentInventory, error) {
	if adapter == nil || adapter.dependencies.Provisioner == nil {
		return preparationcontract.IntentInventory{}, fmt.Errorf("Orca provisioner is unavailable")
	}
	inventory, err := adapter.dependencies.Provisioner.InspectIntent(ctx, toPortIntentRequest(request))
	if err != nil {
		return preparationcontract.IntentInventory{}, err
	}
	result := preparationcontract.IntentInventory{AuthoritativeZero: inventory.AuthoritativeZero}
	for _, candidate := range inventory.Candidates {
		result.Candidates = append(result.Candidates, fromPortIntentReceipt(candidate))
	}
	return result, nil
}

func (adapter *OrcaGatewayAdapter) Invoke(ctx context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentReceipt, error) {
	if adapter == nil || adapter.dependencies.Provisioner == nil {
		return preparationcontract.IntentReceipt{}, fmt.Errorf("Orca provisioner is unavailable")
	}
	if request.Launch != nil {
		if adapter.dependencies.HydrateLaunch == nil {
			return preparationcontract.IntentReceipt{}, fmt.Errorf("Orca sealed launch hydrator is unavailable")
		}
		var err error
		request, err = adapter.dependencies.HydrateLaunch(ctx, request)
		if err != nil {
			return preparationcontract.IntentReceipt{}, err
		}
	}
	receipt, err := adapter.dependencies.Provisioner.InvokeIntent(ctx, toPortIntentRequest(request))
	if err != nil {
		state := preparationcontract.InvocationUnknown
		if typed, ok := errors.AsType[*port.OrcaError](err); ok && !typed.Invoked {
			state = preparationcontract.InvocationNotInvoked
		}
		return preparationcontract.IntentReceipt{}, &preparationcontract.InvocationError{State: state, Cause: err}
	}
	return fromPortIntentReceipt(receipt), nil
}

func toPortIntentRequest(request preparationcontract.IntentRequest) port.ExecutionOrcaIntentRequest {
	result := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentStage(request.Stage), Marker: request.Marker,
		Workspace: toWorkspaceRequest(request.Workspace),
		Probe: port.ExecutionOrcaProbeRequest{
			Repo: request.Probe.Repo, Host: request.Probe.Host, Model: request.Probe.Model,
			Effort: request.Probe.Effort, Provider: request.Probe.Provider,
			Issue: request.Probe.Issue, Marker: request.Probe.Marker,
		},
		TerminalPTYID: request.TerminalPTYID, RunID: request.RunID,
		RunBound: request.RunBound, TaskID: request.TaskID,
	}
	if request.Prepared != nil {
		result.Prepared = &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{
				SourceRoot: request.Prepared.Workspace.SourceRoot, Root: request.Prepared.Workspace.Root,
				Branch: request.Prepared.Workspace.Branch, BaseHead: request.Prepared.Workspace.BaseHead,
				ParentWorktree: request.Prepared.Workspace.ParentWorktree,
				Driver:         request.Prepared.Workspace.Driver, Exists: request.Prepared.Workspace.Exists,
			},
			RuntimeID: request.Prepared.RuntimeID, RepoID: request.Prepared.RepoID,
			WorktreeID: request.Prepared.WorktreeID, WorktreeInstanceID: request.Prepared.WorktreeInstanceID,
		}
	}
	if request.Launch != nil {
		result.Launch = &port.ExecutionOrcaLaunchRequest{
			Prompt: request.Launch.Prompt, PromptPath: request.Launch.PromptPath,
			PromptSHA256:        request.Launch.PromptSHA256,
			ContextPacketPath:   request.Launch.ContextPacketPath,
			ContextPacketSHA256: request.Launch.ContextPacketSHA256,
		}
	}
	return result
}

func fromPortIntentReceipt(receipt port.ExecutionOrcaIntentReceipt) preparationcontract.IntentReceipt {
	result := preparationcontract.IntentReceipt{
		TerminalPTYID: receipt.TerminalPTYID, TerminalHandle: receipt.TerminalHandle,
		RunID: receipt.RunID, RunBound: receipt.RunBound, TaskID: receipt.TaskID,
		DispatchID: receipt.DispatchID,
	}
	if receipt.Workspace != nil {
		result.Workspace = &preparationcontract.OrcaWorkspaceReceipt{
			Workspace: preparationcontract.WorkspaceReceipt{
				SourceRoot: receipt.Workspace.Workspace.SourceRoot, Root: receipt.Workspace.Workspace.Root,
				Branch: receipt.Workspace.Workspace.Branch, BaseHead: receipt.Workspace.Workspace.BaseHead,
				ParentWorktree: receipt.Workspace.Workspace.ParentWorktree,
				Driver:         receipt.Workspace.Workspace.Driver, Exists: receipt.Workspace.Workspace.Exists,
			},
			RuntimeID: receipt.Workspace.RuntimeID, RepoID: receipt.Workspace.RepoID,
			WorktreeID: receipt.Workspace.WorktreeID, WorktreeInstanceID: receipt.Workspace.WorktreeInstanceID,
		}
	}
	return result
}
