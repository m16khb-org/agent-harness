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
	"agent-harness/internal/core/issueops/pathutil"
	issueremote "agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/preflight"
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
	// 기본값은 요청 정규화 단계에서 한 번만 적용한다: 이후의 probe 조립과
	// intent 드리프트 검사(beginOrcaExecutionIntent)가 같은 값을 보게 된다.
	if defaultModel, defaultEffort, ok := port.IssueOpsImplementerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost))); ok {
		if strings.TrimSpace(req.OwnerModel) == "" {
			req.OwnerModel = defaultModel
		}
		if strings.TrimSpace(req.OwnerEffort) == "" {
			req.OwnerEffort = defaultEffort
		}
	}
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
	// preview·confirm 공통 조기 노출: 다른 lifecycle이 이미 이 canonical root를
	// 주장하고 있으면 provisioner를 만지기 전에 선점 사이클을 그대로 알려준다.
	if err := ensureExecutionRootUnclaimed(stateRoot, record.ID, workspaceReq.Root); err != nil {
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
	// direct 모드도 스테이징 artifact를 워크트리로 materialize한다 — packet
	// manifest 봉인은 orca 전용이지만, 스테이징이 조용한 no-op이 되어서는 안
	// 된다(C4a-F3). 홀더(호출 세션)가 같은 파일을 즉시 읽는다.
	if _, err := materializeStagedArtifacts(stateRoot, record); err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
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
		// withIssueOpsLock은 state-root 전역 span이므로 락 밖 preview 검사 이후에
		// 열린 TOCTOU 창을 임계구역에서 다시 닫는다.
		if err := ensureExecutionRootUnclaimed(stateRoot, current.ID, record.WorktreePath); err != nil {
			return err
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
	if err := ensureOrcaBranchIsFree(record, workspaceReq.Branch); err != nil {
		return ExecutionPrepareResult{OK: false, ID: record.ID}, err
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

// ensureOrcaBranchIsFree는 Orca가 워크트리를 만들기 전에 대상 브랜치 이름이
// 비어 있는지 확인한다.
//
// Orca `worktree create`는 언제나 새 브랜치를 만든다. 기존 브랜치를 체크아웃하는
// 옵션이 없다(`--base-branch`는 시작 ref, `--name`이 새 브랜치 이름). 그래서 이름이
// 이미 쓰이고 있으면 Orca가 `<branch>-2`처럼 접미사를 붙이고, 그 결과를
// CanonicalizeWorktreeBranch가 `worktree_branch_mismatch`로 거부한다. 그 거부는
// `Invoked: true`라 pending intent와 실제 Orca 워크트리를 남기며, 실측에서 그
// 잔여물이 abandon까지 막았다(#149).
//
// IssueOps는 linked branch를 먼저 만들도록 요구하므로 정식 순서를 따를수록 이
// 충돌이 확실해진다. mutation 이전에 막아 잔여물 자체를 없앤다.
//
// 로컬 refs만 본다. Orca가 원격 전용 브랜치에 어떻게 반응하는지는 확인하지
// 못했고, 추측으로 정상 경로를 막지 않는다.
func ensureOrcaBranchIsFree(record IssueOpsRecord, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("Orca prepare requires a branch name")
	}
	code, _, _ := preflight.GitCmd(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if code != 0 {
		return nil
	}
	return fmt.Errorf(
		"branch %q already exists, so Orca cannot prepare this execution: Orca always creates a new branch, so it would take a different name (observed: a numeric suffix) and fail as worktree_branch_mismatch only after the worktree exists; "+
			"use --mode direct, which adopts the existing branch, or delete the local branch first if it holds no work",
		branch)
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
	if probeReq.Provider == "gitlab" {
		const code = "gitlab_issue_metadata_unsupported"
		if requested == ExecutionModeAuto {
			return string(model.ExecutionModeDirect), code, probeReq, nil
		}
		return "", "", probeReq, fmt.Errorf("%s: installed Orca cannot seal GitLab issue metadata before handoff", code)
	}
	if probeReq.Host != "codex" && probeReq.Host != "claude" {
		return "", "", probeReq, fmt.Errorf("Orca owner_host must be codex or claude")
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

// ensureExecutionRootUnclaimed는 다른 lifecycle 레코드가 이미 주장한 canonical
// worktree root를 fail-closed로 거부한다. leaf 파생(strings.ReplaceAll(branch,
// "/", "-"))이 비단사라 "72/fix"와 "72-fix"가 같은 root로 접히지만 lifecycle ID는
// 브랜치로 해시되어 서로 다른 레코드가 된다 — 두 사이클이 같은 워크트리를
// 소유하는 불변식 위반을 prepare 입구에서 막는다.
//
// 스캔은 phase·lease 상태와 무관한 전 레코드다: cleanup finish가 워크트리를
// 제거한 뒤에야 레코드를 삭제하므로 레코드의 존재가 곧 root 소유권이다.
// WorktreePath와 Execution.Workspace.Root의 합집합을 보는 이유는 linking 경로가
// Execution 없이 WorktreePath만 채울 수 있고 그 경로에는 레코드 간 유일성 검증이
// 없기 때문이다. 읽기 오류는 통과가 아니라 거부다 — 손상된 레코드가 소유권
// 주장을 조용히 잃어서는 안 된다.
func ensureExecutionRootUnclaimed(stateRoot, selfID, root string) error {
	target := pathutil.CleanAbsPath(root)
	if target == "" {
		return fmt.Errorf("canonical worktree root is required")
	}
	self := strings.TrimSpace(selfID)
	ids, err := ListIssueOpsIDs(stateRoot)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == self {
			continue
		}
		record, err := readIssueOpsUnchecked(stateRoot, id)
		if err != nil {
			return fmt.Errorf("canonical worktree 소유권 스캔이 lifecycle %s 레코드를 읽지 못했다; 손상 레코드를 먼저 해소하라: %w", id, err)
		}
		for _, claimed := range []string{record.WorktreePath, executionRecordWorkspaceRoot(record)} {
			if strings.TrimSpace(claimed) == "" || pathutil.CleanAbsPath(claimed) != target {
				continue
			}
			return fmt.Errorf(
				"canonical worktree %s는 이미 lifecycle %s(브랜치 %s)가 선점했다; 먼저 그 사이클을 정리하라: agent-harness issueops cleanup finish --id %s --preview --json",
				target, id, strings.TrimSpace(record.Branch), id,
			)
		}
	}
	return nil
}

func executionRecordWorkspaceRoot(record IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	return record.Execution.Workspace.Root
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
