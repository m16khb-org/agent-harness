package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	issueremote "agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/port"
)

const ExecutionModeAuto = "auto"

type ExecutionPrepareRequest struct {
	ID          string            `json:"id"`
	Mode        string            `json:"mode"`
	Actor       model.NativeActor `json:"actor"`
	CWD         string            `json:"cwd"`
	OwnerHost   string            `json:"owner_host,omitempty"`
	OwnerModel  string            `json:"owner_model,omitempty"`
	OwnerEffort string            `json:"owner_effort,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
}

type ExecutionPrepareDependencies struct {
	Direct    port.ExecutionWorkspaceProvisioner
	Orca      port.ExecutionOrcaProvisioner
	ReadIssue ExecutionIssueSnapshotReadFunc
	Now       func() time.Time
}

type ExecutionPrepareResult struct {
	OK                  bool             `json:"ok"`
	ID                  string           `json:"id"`
	Preview             bool             `json:"preview,omitempty"`
	RequestedMode       string           `json:"requested_mode"`
	ResolvedMode        string           `json:"resolved_mode"`
	FallbackCode        string           `json:"fallback_code,omitempty"`
	Workspace           model.Workspace  `json:"workspace"`
	Execution           *model.Execution `json:"execution,omitempty"`
	ClaimTokenPath      string           `json:"claim_token_path,omitempty"`
	IssueBodySHA256     string           `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string           `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string           `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string           `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string           `json:"owner_prompt_sha256,omitempty"`
	NextCommand         string           `json:"next_command,omitempty"`
}

func PrepareExecution(ctx context.Context, stateRoot string, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies) (ExecutionPrepareResult, error) {
	if req.Confirm {
		if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
			return ExecutionPrepareResult{OK: false, ID: req.ID}, err
		}
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID}, err
	}
	requested, err := normalizeExecutionPrepareMode(req.Mode)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID}, err
	}
	if record.Execution != nil {
		if record.Execution.Pending != nil {
			result := preparedExecutionResult(record, requested)
			result.OK = false
			result.NextCommand = executionReconcileCommand(record.ID, true)
			return result, fmt.Errorf("IssueOps execution has a pending external intent; run %s", result.NextCommand)
		}
		return preparedExecutionResult(record, requested), nil
	}
	workspaceReq, err := executionWorkspaceRequest(record, req.Confirm)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID, RequestedMode: requested}, err
	}
	resolved, fallback, probe, err := resolveExecutionPrepareMode(ctx, record, req, requested, deps.Orca)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: req.ID, RequestedMode: requested}, err
	}
	result := ExecutionPrepareResult{
		OK: true, ID: record.ID, Preview: !req.Confirm,
		RequestedMode: requested, ResolvedMode: resolved, FallbackCode: fallback,
		Workspace: model.Workspace{
			SourceRoot: workspaceReq.SourceRoot, Root: workspaceReq.Root, Branch: workspaceReq.Branch,
			BaseHead: workspaceReq.BaseHead, Driver: map[string]string{"direct": "git", "orca": "orca"}[resolved],
		},
	}
	if resolved == string(model.ExecutionModeDirect) {
		return prepareDirectExecution(ctx, stateRoot, record, req, deps, workspaceReq, result)
	}
	return prepareOrcaExecution(ctx, stateRoot, record, req, deps, workspaceReq, probe, result)
}

func prepareDirectExecution(ctx context.Context, stateRoot string, record IssueOpsRecord, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies, workspaceReq port.ExecutionWorkspaceRequest, result ExecutionPrepareResult) (ExecutionPrepareResult, error) {
	if deps.Direct == nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	if !samePath(req.CWD, record.Repo) && !samePath(req.CWD, workspaceReq.Root) {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct prepare cwd must be source_root or the canonical worktree")
	}
	if req.Confirm {
		prober, ok := deps.Direct.(port.ExecutionWorkspaceAccessProber)
		if !ok {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("direct provisioner cannot verify canonical worktree base access")
		}
		access, err := prober.ProbeAccess(ctx, workspaceReq, actor.Host)
		if err != nil {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, err
		}
		if !access.Allowed {
			return ExecutionPrepareResult{
				OK: false, ID: record.ID, RequestedMode: result.RequestedMode, ResolvedMode: result.ResolvedMode,
				Workspace: result.Workspace, NextCommand: access.RelaunchCommand,
			}, fmt.Errorf("canonical worktree base is not accessible; relaunch with: %s", access.RelaunchCommand)
		}
	}
	receipt, err := deps.Direct.Prepare(ctx, workspaceReq)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	result.Workspace = workspaceFromReceipt(receipt, executionNow(deps.Now))
	if !req.Confirm {
		return result, nil
	}
	record.WorktreePath = receipt.Root
	record.Execution = &model.Execution{
		Mode:      model.ExecutionModeDirect,
		Workspace: result.Workspace,
		Lease: model.WriteLease{
			Generation: 1, Status: model.LeaseStatusActive, Holder: &actor,
			ClaimedAt: executionNow(deps.Now),
		},
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return err
		}
		if current.Execution != nil {
			persisted = current
			return nil
		}
		current.WorktreePath = record.WorktreePath
		current.Execution = record.Execution
		persisted, err = persistExecutionTransition(stateRoot, current, nil)
		return err
	})
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	return preparedExecutionResultWithModes(persisted, result.RequestedMode, result.FallbackCode), nil
}

func prepareOrcaExecution(ctx context.Context, stateRoot string, record IssueOpsRecord, req ExecutionPrepareRequest, deps ExecutionPrepareDependencies, workspaceReq port.ExecutionWorkspaceRequest, probe port.ExecutionOrcaProbeRequest, result ExecutionPrepareResult) (ExecutionPrepareResult, error) {
	if deps.Orca == nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca provisioner is unavailable")
	}
	if !req.Confirm {
		return result, nil
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	req.Actor = actor
	if !samePath(req.CWD, record.Repo) && !samePath(req.CWD, workspaceReq.Root) {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca prepare cwd must be source_root or the canonical worktree")
	}
	snapshot, err := readExecutionOwnerSnapshot(ctx, record, deps.ReadIssue)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	pending, payload, err := beginOrcaExecutionIntent(stateRoot, record, workspaceReq, probe, req, snapshot, deps.Now)
	if err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
	}
	for step := 0; pending.Execution != nil && pending.Execution.Pending != nil; step++ {
		if step >= 4 {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, fmt.Errorf("Orca prepare exceeded the fixed external intent stage count")
		}
		pending, payload, err = executeOrcaIntentStage(ctx, stateRoot, pending, payload, deps.Orca, deps.ReadIssue, deps.Now)
		if err != nil {
			return ExecutionPrepareResult{OK: false, ID: record.ID}, err
		}
	}
	result = preparedExecutionResultWithModes(pending, result.RequestedMode, result.FallbackCode)
	result.ClaimTokenPath = claimTokenPath(pending)
	result.IssueBodySHA256 = snapshot.issue.BodySHA256
	if payload.Launch != nil {
		result.ContextPacketPath, result.ContextPacketSHA256 = payload.Launch.ContextPacketPath, payload.Launch.ContextPacketSHA256
		result.OwnerPromptPath, result.OwnerPromptSHA256 = payload.Launch.PromptPath, payload.Launch.PromptSHA256
	}
	return result, nil
}

func validateExecutionOrcaReceipt(workspace port.ExecutionWorkspaceRequest, receipt port.ExecutionOrcaReceipt) error {
	got := receipt.Workspace
	if !samePath(got.SourceRoot, workspace.SourceRoot) || !samePath(got.Root, workspace.Root) || got.Branch != workspace.Branch || got.BaseHead != workspace.BaseHead || got.Driver != "orca" ||
		strings.TrimSpace(receipt.RuntimeID) == "" || strings.TrimSpace(receipt.RepoID) == "" || strings.TrimSpace(receipt.WorktreeID) == "" || strings.TrimSpace(receipt.TaskID) == "" || strings.TrimSpace(receipt.DispatchID) == "" {
		return fmt.Errorf("Orca receipt does not match the sealed execution workspace and launch identity")
	}
	return nil
}

func validateExecutionOrcaWorkspaceReceipt(workspace port.ExecutionWorkspaceRequest, receipt port.ExecutionOrcaWorkspaceReceipt) error {
	got := receipt.Workspace
	if !samePath(got.SourceRoot, workspace.SourceRoot) || !samePath(got.Root, workspace.Root) || got.Branch != workspace.Branch || got.BaseHead != workspace.BaseHead || got.Driver != "orca" ||
		strings.TrimSpace(receipt.RuntimeID) == "" || strings.TrimSpace(receipt.RepoID) == "" || strings.TrimSpace(receipt.WorktreeID) == "" {
		return fmt.Errorf("Orca workspace receipt does not match the sealed execution identity")
	}
	return nil
}

func newExecutionOperationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func resolveExecutionPrepareMode(ctx context.Context, record IssueOpsRecord, req ExecutionPrepareRequest, requested string, orca port.ExecutionOrcaProvisioner) (string, string, port.ExecutionOrcaProbeRequest, error) {
	probeReq := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: strings.ToLower(strings.TrimSpace(req.OwnerHost)),
		Model: strings.TrimSpace(req.OwnerModel), Effort: strings.TrimSpace(req.OwnerEffort),
		Marker: "agent-harness issueops-v1 lifecycle=" + record.ID,
	}
	if record.BranchPrepare != nil {
		probeReq.Provider = strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
		if value := issueremote.IssueNumber(record.BranchPrepare.IssueURL); value != "" {
			probeReq.Issue, _ = strconv.Atoi(value)
		}
	}
	if requested == string(model.ExecutionModeDirect) {
		return requested, "", probeReq, nil
	}
	if probeReq.Host != "codex" && probeReq.Host != "claude" {
		return "", "", probeReq, fmt.Errorf("Orca owner_host must be codex or claude")
	}
	if probeReq.Model == "" {
		return "", "", probeReq, fmt.Errorf("Orca owner_model is required")
	}
	if orca == nil {
		if requested == ExecutionModeAuto {
			return string(model.ExecutionModeDirect), "orca_adapter_unavailable", probeReq, nil
		}
		return "", "", probeReq, fmt.Errorf("Orca provisioner is unavailable")
	}
	probe, err := orca.Probe(ctx, probeReq)
	if err != nil || !probe.Available || !probe.Ready {
		code := strings.TrimSpace(probe.Code)
		if code == "" {
			code = "orca_probe_failed"
		}
		if requested == ExecutionModeAuto {
			return string(model.ExecutionModeDirect), code, probeReq, nil
		}
		if err != nil {
			return "", "", probeReq, fmt.Errorf("Orca probe failed: %w", err)
		}
		return "", "", probeReq, fmt.Errorf("Orca probe failed: %s", code)
	}
	return string(model.ExecutionModeOrca), "", probeReq, nil
}

func executionWorkspaceRequest(record IssueOpsRecord, confirm bool) (port.ExecutionWorkspaceRequest, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("execution branch is invalid")
	}
	return port.ExecutionWorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: filepath.Join(record.Repo+".worktrees", leaf),
		Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch),
		BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), Confirm: confirm,
	}, nil
}

var issueOpsOwnerReportLabels = []string{
	"Status",
	"Lifecycle",
	"Mode/host/model",
	"Worktree/branch/final HEAD",
	"Lease generation/completion",
	"Issue/packet digests",
	"Commits",
	"Changed files",
	"Acceptance evidence",
	"Verification",
	"AI-slop clean",
	"Draft PR/MR",
	"Deviations",
	"Blockers",
}

func renderExecutionOwnerReportContract(record IssueOpsRecord, req ExecutionPrepareRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if record.Execution != nil {
		mode = string(record.Execution.Mode)
	}
	values := []string{
		"completed | blocked",
		record.ID,
		mode + " / " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) + " / " + strings.TrimSpace(req.OwnerModel) + " (" + strings.TrimSpace(req.OwnerEffort) + ")",
		"<exact values>",
		"<generation + receipt or blocker>",
		"<verified | drift, 원문 secret 없음>",
		"<ordered SHA + subject>",
		"<exact paths>",
		"<AC-ID → test/command/result mapping>",
		"<every command + PASS/FAIL>",
		"<removed duplication/legacy/noise or none>",
		"<URL or none>",
		"<issue-vs-code mismatch with file:line evidence or none>",
		"<exact state/error/next command or none>",
	}
	lines := []string{"## IssueOps v1 Owner Report"}
	for index, label := range issueOpsOwnerReportLabels {
		lines = append(lines, "- "+label+": "+values[index])
	}
	return strings.Join(lines, "\n")
}

func executionReconcileCommand(id string, preview bool) string {
	action := "--confirm"
	if preview {
		action = "--preview"
	}
	return "agent-harness issueops execution reconcile --id " + id + " " + action + " ACTOR_FLAGS"
}

func normalizeExecutionPrepareMode(mode string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(mode)); normalized {
	case "", ExecutionModeAuto:
		return ExecutionModeAuto, nil
	case string(model.ExecutionModeDirect), string(model.ExecutionModeOrca):
		return normalized, nil
	default:
		return "", fmt.Errorf("execution mode must be auto, direct, or orca")
	}
}

func workspaceFromReceipt(receipt port.ExecutionWorkspaceReceipt, linkedAt string) model.Workspace {
	return model.Workspace{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
		BaseHead: receipt.BaseHead, Driver: receipt.Driver, LinkedAt: linkedAt,
	}
}

func preparedExecutionResult(record IssueOpsRecord, requested string) ExecutionPrepareResult {
	return preparedExecutionResultWithModes(record, requested, "")
}

func preparedExecutionResultWithModes(record IssueOpsRecord, requested, fallback string) ExecutionPrepareResult {
	return ExecutionPrepareResult{
		OK: true, ID: record.ID, RequestedMode: requested, ResolvedMode: string(record.Execution.Mode),
		FallbackCode: fallback, Workspace: record.Execution.Workspace, Execution: record.Execution,
	}
}

func executionNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}
