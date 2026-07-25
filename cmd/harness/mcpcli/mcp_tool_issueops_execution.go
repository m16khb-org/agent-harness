package mcpcli

import (
	"context"
	"os"

	"agent-harness/cmd/harness/issueopscli/remoteverify"
	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func handleMCPIssueOpsExecution(args map[string]any) MCPToolOutcome {
	orcaExecution := orca.NewExecution()
	result, err := issueops.ExecuteExecution(context.Background(), core.IssueOpsStateRoot(), executionActionRequestFromMCP(args), issueops.ExecutionActionDependencies{
		Direct: gitworktree.New(), Orca: orcaExecution, OrcaOwner: orcaExecution, ReadIssue: provider.ReadExecutionIssueSnapshot,
		// 완료가 orca task를 종결시킨다. CLI 경로와 같은 계약이다(#130).
		SettleOrcaTask: orca.New().SettleTask,
		RemotePR: issueops.RemotePullRequestDependencies{
			Create: func(providerName string, req core.IssueProviderCreatePullRequestRequest) (core.IssueProviderCreatePullRequestResult, error) {
				prov, err := provider.Resolve(providerName)
				if err != nil {
					return core.IssueProviderCreatePullRequestResult{}, err
				}
				return core.CreateRemotePullRequest(req, prov)
			},
			Reconcile: func(providerName string, req core.IssueProviderReconcilePullRequestRequest) (core.IssueProviderReconcilePullRequestResult, error) {
				prov, err := provider.Resolve(providerName)
				if err != nil {
					return core.IssueProviderReconcilePullRequestResult{}, err
				}
				return core.ReconcileRemotePullRequest(req, prov)
			},
			Verify: remoteverify.VerifyRemoteArtifactLive,
		},
	})
	if err != nil {
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	return mcpToolPayload(result)
}

func issueOpsMCPErrorPayload(err error) map[string]any {
	payload := map[string]any{"ok": false, "error": err.Error()}
	if structured, ok := err.(interface{ IssueOpsErrorFields() map[string]any }); ok {
		for key, value := range structured.IssueOpsErrorFields() {
			if value != nil && value != "" {
				payload[key] = value
			}
		}
	}
	return payload
}

func executionActionRequestFromMCP(args map[string]any) issueops.ExecutionActionRequest {
	ancestry, _ := issueops.ObserveNativeProcessAncestry(os.Getpid())
	// 관측이 실패하면 ancestry가 비어 core mutation validation이 호출자의
	// process receipt를 신뢰하는 대신 fail-closed로 동작한다.
	return executionActionRequestFromMCPWithAncestry(args, ancestry)
}

func executionActionRequestFromMCPWithAncestry(args map[string]any, ancestry []model.NativeProcessReceipt) issueops.ExecutionActionRequest {
	pid := argmap.Int(args, "session_pid", 0)
	return issueops.ExecutionActionRequest{
		Action: argmap.String(args, "action"), ID: argmap.String(args, "id"), Mode: argmap.String(args, "mode"),
		Actor: model.NativeActor{
			Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"),
			SessionProcess:  &model.NativeProcessReceipt{PID: pid, StartedAt: argmap.String(args, "session_started_at"), Executable: argmap.String(args, "session_executable")},
			ProcessAncestry: append([]model.NativeProcessReceipt(nil), ancestry...),
		},
		CWD: argmap.String(args, "cwd"), OwnerHost: argmap.String(args, "owner_host"), OwnerModel: argmap.String(args, "owner_model"), OwnerEffort: argmap.String(args, "owner_effort"),
		Generation: uint64(argmap.Int64(args, "generation", 0)), ExpectedGeneration: uint64(argmap.Int64(args, "expected_generation", 0)),
		TokenFile: argmap.String(args, "claim_token_file"), ReplaceAction: argmap.String(args, "replace_action"),
		IssueBodySHA256: argmap.String(args, "issue_body_sha256"), ContextPacketSHA256: argmap.String(args, "context_packet_sha256"),
		InventoryFingerprint: argmap.String(args, "inventory_fingerprint"), QuiescenceFingerprint: argmap.String(args, "quiescence_fingerprint"),
		Reason: argmap.String(args, "reason"), Preview: argmap.Bool(args, "preview"), Confirm: argmap.Bool(args, "confirm"),
		FinalHead: argmap.String(args, "final_head"), TuringReportPath: argmap.String(args, "turing_report_path"),
		Verification: argmap.StringSlice(args, "verification"), RemoteArtifactURL: argmap.String(args, "remote_artifact_url"),
	}
}
