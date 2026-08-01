package issueopspreparation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/port"
)

func TestOrcaAdapterMapsProbeInspectAndInvoke(t *testing.T) {
	provider := &orcaProviderFake{}
	validated := false
	hydrated := false
	adapter := NewOrcaGateway(OrcaDependencies{
		Provisioner: provider,
		ValidateProbe: func(_ context.Context, request preparationcontract.ProbeRequest) (string, error) {
			validated = request.Workspace.Branch == "199-orca"
			return "", nil
		},
		HydrateLaunch: func(_ context.Context, request preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
			hydrated = true
			request.Launch.Prompt = "sealed prompt"
			return request, nil
		},
	})
	probeRequest := preparationcontract.ProbeRequest{
		Repo: "/repo", Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		Provider: "github", Issue: 199, Marker: "marker",
		Workspace: preparationcontract.WorkspaceRequest{Branch: "199-orca"},
	}
	probe, err := adapter.Probe(context.Background(), probeRequest)
	if err != nil || !probe.Available || !probe.Ready || !validated {
		t.Fatalf("probe=%+v validated=%v err=%v", probe, validated, err)
	}
	wantProbe := port.ExecutionOrcaProbeRequest{Repo: "/repo", Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh", Provider: "github", Issue: 199, Marker: "marker"}
	if !reflect.DeepEqual(provider.probe, wantProbe) {
		t.Fatalf("probe request=%+v want=%+v", provider.probe, wantProbe)
	}

	request := adapterIntentRequest()
	inventory, err := adapter.Inspect(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 1 || inventory.Candidates[0].RunID != "run" {
		t.Fatalf("inventory=%+v err=%v", inventory, err)
	}
	receipt, err := adapter.Invoke(context.Background(), request)
	if err != nil || receipt.DispatchID != "dispatch" || !hydrated || provider.invoke.Launch == nil || provider.invoke.Launch.Prompt != "sealed prompt" {
		t.Fatalf("receipt=%+v hydrated=%v invoke=%+v err=%v", receipt, hydrated, provider.invoke, err)
	}
}

func TestOrcaAdapterPreservesInvocationCertainty(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		state string
	}{
		{name: "not invoked", err: &port.OrcaError{Code: "rejected", Invoked: false}, state: preparationcontract.InvocationNotInvoked},
		{name: "invoked", err: &port.OrcaError{Code: "timeout", Invoked: true}, state: preparationcontract.InvocationUnknown},
		{name: "untyped", err: errors.New("transport closed"), state: preparationcontract.InvocationUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &orcaProviderFake{invokeErr: test.err}
			adapter := NewOrcaGateway(OrcaDependencies{Provisioner: provider})
			_, err := adapter.Invoke(context.Background(), preparationcontract.IntentRequest{Stage: preparationcontract.IntentStageWorktree})
			var typed *preparationcontract.InvocationError
			if !errors.As(err, &typed) || typed.State != test.state || !errors.Is(err, test.err) {
				t.Fatalf("err=%v typed=%+v", err, typed)
			}
		})
	}
}

func TestOrcaAdapterFailsClosedOnMissingDependenciesAndBranchPrecheck(t *testing.T) {
	adapter := NewOrcaGateway(OrcaDependencies{})
	if _, err := adapter.Probe(context.Background(), preparationcontract.ProbeRequest{}); err == nil {
		t.Fatal("nil Orca provisioner accepted")
	}
	provider := &orcaProviderFake{}
	adapter = NewOrcaGateway(OrcaDependencies{Provisioner: provider})
	if _, err := adapter.Probe(context.Background(), preparationcontract.ProbeRequest{}); err == nil {
		t.Fatal("nil branch precheck accepted")
	}
	adapter = NewOrcaGateway(OrcaDependencies{
		Provisioner: provider,
		ValidateProbe: func(context.Context, preparationcontract.ProbeRequest) (string, error) {
			return "orca_branch_name_taken", errors.New("branch exists")
		},
	})
	result, err := adapter.Probe(context.Background(), preparationcontract.ProbeRequest{})
	if err == nil || result.Code != "orca_branch_name_taken" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func adapterIntentRequest() preparationcontract.IntentRequest {
	return preparationcontract.IntentRequest{
		Stage: preparationcontract.IntentStageDispatch, Marker: "marker",
		Workspace: preparationcontract.WorkspaceRequest{LifecycleID: "io-orca", SourceRoot: "/repo", Root: "/worktree", Branch: "199-orca", BaseBranch: "main", BaseHead: "base", Confirm: true},
		Probe:     preparationcontract.ProbeRequest{Repo: "/repo", Host: "codex", Model: "model", Provider: "github", Issue: 199, Marker: "marker"},
		Prepared: &preparationcontract.OrcaWorkspaceReceipt{
			Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/worktree", Branch: "199-orca", BaseHead: "base", Driver: "orca"},
			RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
		},
		Launch:        &preparationcontract.LaunchRequest{PromptPath: "/prompt", PromptSHA256: "prompt-sha", ContextPacketPath: "/packet", ContextPacketSHA256: "packet-sha"},
		TerminalPTYID: "terminal", RunID: "run", RunBound: true, TaskID: "task",
	}
}

type orcaProviderFake struct {
	probe     port.ExecutionOrcaProbeRequest
	inspect   port.ExecutionOrcaIntentRequest
	invoke    port.ExecutionOrcaIntentRequest
	invokeErr error
}

func (fake *orcaProviderFake) Probe(_ context.Context, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	fake.probe = request
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true, Code: "ready"}, nil
}
func (fake *orcaProviderFake) InspectIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	fake.inspect = request
	return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{RunID: "run"}}, AuthoritativeZero: false}, nil
}
func (fake *orcaProviderFake) InvokeIntent(_ context.Context, request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invoke = request
	if fake.invokeErr != nil {
		return port.ExecutionOrcaIntentReceipt{}, fake.invokeErr
	}
	return port.ExecutionOrcaIntentReceipt{DispatchID: "dispatch"}, nil
}
