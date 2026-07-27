package orca

import (
	"context"
	"testing"

	"agent-harness/internal/port"
)

// IssueOps는 Orca 준비 전에 BaseHead를 봉인하고 대상 브랜치가 로컬과 원격에
// 모두 없음을 요구한다. 존재하지 않는 원격 ref를 upstream으로 넘기면
// canonicalization이 core에서 부재를 증명한 바로 그 ref를 다시 요구하게 된다.
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
