package mcpcli

import (
	"context"
	"fmt"
	"os"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/adapter/core"
	"agent-harness/internal/adapter/issueops"
	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func handleMCPIssueOpsExecution(args map[string]any) MCPToolOutcome {
	return handleMCPIssueOpsExecutionWithDependencies(args, MCPDependencies{})
}

func handleMCPIssueOpsExecutionWithReleaseHandler(args map[string]any, release issueops.ExecutionReleaseHandler) MCPToolOutcome {
	return handleMCPIssueOpsExecutionWithDependencies(args, MCPDependencies{Release: release})
}

func handleMCPIssueOpsExecutionWithDependencies(args map[string]any, deps MCPDependencies) MCPToolOutcome {
	req, err := executionActionRequestFromMCP(args)
	if err != nil {
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	result, err := issueops.ExecuteExecution(context.Background(), core.IssueOpsStateRoot(), req, issueOpsExecutionActionDependencies(deps))
	if err != nil {
		err = bindMCPIssueOpsExecutionErrorNextCommand(err, deps.Provenance)
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	result, err = bindMCPIssueOpsExecutionNextCommand(result, deps.Provenance)
	if err != nil {
		return mcpToolErrorPayload(issueOpsMCPErrorPayload(err))
	}
	return mcpToolPayload(result)
}

func issueOpsExecutionActionDependencies(deps MCPDependencies) issueops.ExecutionActionDependencies {
	return issueops.ExecutionActionDependencies{
		Prepare: deps.Prepare, Orca: deps.Orca, OrcaOwner: deps.OrcaOwner, ReadIssue: deps.ReadIssue,
		Claim: deps.Claim, Release: deps.Release, Reseed: deps.Reseed, Resume: deps.Resume, Reconcile: deps.Reconcile, Complete: deps.Complete,
		RemoteReconcile: deps.Publication.Reconcile,
	}
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

func executionActionRequestFromMCP(args map[string]any) (issueops.ExecutionActionRequest, error) {
	ancestry, _ := issueops.ObserveNativeProcessAncestry(os.Getpid())
	// 관측이 실패하면 ancestry가 비어 core mutation validation이 호출자의
	// process receipt를 신뢰하는 대신 fail-closed로 동작한다.
	return executionActionRequestFromMCPWithAncestry(args, ancestry)
}

func executionActionRequestFromMCPWithAncestry(args map[string]any, ancestry []model.NativeProcessReceipt) (issueops.ExecutionActionRequest, error) {
	pid := argmap.Int(args, "session_pid", 0)
	snapshot, err := executionIssueSnapshotFromMCP(args)
	if err != nil {
		return issueops.ExecutionActionRequest{}, err
	}
	return issueops.ExecutionActionRequest{
		Action: argmap.String(args, "action"), ID: argmap.String(args, "id"), Mode: argmap.String(args, "mode"),
		Actor: model.NativeActor{
			Host: argmap.String(args, "host"), SessionID: argmap.String(args, "session_id"), AgentID: argmap.String(args, "agent_id"),
			SessionProcess:  &model.NativeProcessReceipt{PID: pid, StartedAt: argmap.String(args, "session_started_at"), Executable: argmap.String(args, "session_executable")},
			ProcessAncestry: append([]model.NativeProcessReceipt(nil), ancestry...),
		},
		CWD: argmap.String(args, "cwd"), OwnerHost: argmap.String(args, "owner_host"), OwnerModel: argmap.String(args, "owner_model"), OwnerEffort: argmap.String(args, "owner_effort"),
		Generation: uint64(argmap.Int64(args, "generation", 0)), ExpectedGeneration: uint64(argmap.Int64(args, "expected_generation", 0)), CompletionGeneration: uint64(argmap.Int64(args, "completion_generation", 0)),
		TokenFile: argmap.String(args, "claim_token_file"), ReplaceAction: argmap.String(args, "replace_action"),
		IssueBodySHA256: argmap.String(args, "issue_body_sha256"), ContextPacketSHA256: argmap.String(args, "context_packet_sha256"),
		InventoryFingerprint: argmap.String(args, "inventory_fingerprint"), QuiescenceFingerprint: argmap.String(args, "quiescence_fingerprint"),
		Reason: argmap.String(args, "reason"), Preview: argmap.Bool(args, "preview"), Confirm: argmap.Bool(args, "confirm"),
		FinalHead: argmap.String(args, "final_head"), TuringReportPath: argmap.String(args, "turing_report_path"),
		Verification: argmap.StringSlice(args, "verification"), RemoteArtifactURL: argmap.String(args, "remote_artifact_url"),
		IssueSnapshot: snapshot,
	}, nil
}

func executionIssueSnapshotFromMCP(args map[string]any) (*port.ExecutionIssueSnapshotEvidence, error) {
	raw, exists := args["issue_snapshot"]
	if !exists {
		return nil, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("issue_snapshot must be an object")
	}
	allowed := map[string]bool{
		"provider": true,
		"source":   true,
		"web_url":  true,
		"body":     true,
		"state":    true,
	}
	for key := range object {
		if !allowed[key] {
			return nil, fmt.Errorf("issue_snapshot contains unsupported field %q", key)
		}
	}
	field := func(name string) (string, error) {
		value, exists := object[name]
		if !exists {
			return "", fmt.Errorf("issue_snapshot.%s is required", name)
		}
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("issue_snapshot.%s must be a string", name)
		}
		return text, nil
	}
	provider, err := field("provider")
	if err != nil {
		return nil, err
	}
	source, err := field("source")
	if err != nil {
		return nil, err
	}
	webURL, err := field("web_url")
	if err != nil {
		return nil, err
	}
	body, err := field("body")
	if err != nil {
		return nil, err
	}
	state, err := field("state")
	if err != nil {
		return nil, err
	}
	return &port.ExecutionIssueSnapshotEvidence{
		Provider: provider,
		Source:   source,
		WebURL:   webURL,
		Body:     body,
		State:    state,
	}, nil
}
