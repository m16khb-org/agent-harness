package orca

import (
	"context"
	"testing"

	"agent-harness/internal/port"
)

// IssueOps seals BaseHead before Orca preparation and requires the target
// branch name to be absent both locally and remotely. Passing the absent remote
// ref as an upstream makes Orca canonicalization require the very ref that the
// core just proved must not exist.
func TestExecutionWorktreeCreationUsesTheSealedBaseWithoutAnAbsentUpstream(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*ExecutionProvisioner, port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) error
	}{
		{
			name: "prepare workspace",
			run: func(provisioner *ExecutionProvisioner, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest) error {
				_, err := provisioner.PrepareWorkspace(context.Background(), workspace, probe)
				return err
			},
		},
		{
			name: "worktree intent",
			run: func(provisioner *ExecutionProvisioner, workspace port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest) error {
				_, err := provisioner.InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
					Stage: port.ExecutionOrcaIntentWorktree, Marker: probe.Marker,
					Workspace: workspace, Probe: probe,
				})
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace, probe := executionFixture(t)
			client := &executionFake{workspace: workspace, probeRequest: probe}

			if err := test.run(NewExecutionClient(client), workspace, probe); err != nil {
				t.Fatal(err)
			}
			if client.worktreeRequest.BaseBranch != workspace.BaseHead {
				t.Fatalf("create must stay pinned to the sealed base: %#v", client.worktreeRequest)
			}
			if client.worktreeRequest.UpstreamBranch != "" {
				t.Fatalf("the remote branch is required to be absent before Orca create, so it cannot be the upstream: %#v", client.worktreeRequest)
			}
		})
	}
}
