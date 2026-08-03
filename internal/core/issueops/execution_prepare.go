package issueops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const ExecutionModeAuto = "auto"

type ExecutionPrepareRequest struct {
	ID          string               `json:"id"`
	Mode        string               `json:"mode"`
	Actor       issueops.NativeActor `json:"actor"`
	CWD         string               `json:"cwd"`
	OwnerHost   string               `json:"owner_host,omitempty"`
	OwnerModel  string               `json:"owner_model,omitempty"`
	OwnerEffort string               `json:"owner_effort,omitempty"`
	Confirm     bool                 `json:"confirm,omitempty"`
}

type ExecutionPrepareResult struct {
	OK                  bool                `json:"ok"`
	ID                  string              `json:"id"`
	Preview             bool                `json:"preview,omitempty"`
	RequestedMode       string              `json:"requested_mode"`
	ResolvedMode        string              `json:"resolved_mode"`
	FallbackCode        string              `json:"fallback_code,omitempty"`
	Workspace           issueops.Workspace  `json:"workspace"`
	Execution           *issueops.Execution `json:"execution,omitempty"`
	ClaimTokenPath      string              `json:"claim_token_path,omitempty"`
	IssueBodySHA256     string              `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string              `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string              `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string              `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string              `json:"owner_prompt_sha256,omitempty"`
	IssueSnapshotSource string              `json:"issue_snapshot_source,omitempty"`
	NextCommand         string              `json:"next_command,omitempty"`
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
// 로컬과 원격을 모두 본다. #149는 로컬 refs만 봤는데, `gh issue develop`은 원격에만
// 브랜치를 만들므로 정식 순서에서는 그 검사가 **언제나** 통과했다 — 실환경 dogfood가
// 그 구멍으로 접미사 브랜치를 만들어냈다(#154). Orca가 원격 브랜치를 보고 이름을
// 정하므로 사전 확인의 시야도 거기까지여야 한다.
//
// 원격은 remote-tracking ref로 판정한다. `git ls-remote`는 prepare를 네트워크에 묶어
// 오프라인에서 정상 경로를 막는다. 대신 낡은 ref가 이미 삭제된 브랜치를 있다고
// 보고할 수 있어 메시지가 fetch를 안내한다.
func ensureOrcaBranchIsFree(record issueops.IssueOpsRecord, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("Orca prepare requires a branch name")
	}
	for _, scope := range []struct {
		ref    string
		where  string
		remedy string
	}{
		{ref: "refs/heads/" + branch, where: "locally", remedy: "delete the local branch if it holds no work"},
		{ref: "refs/remotes/origin/" + branch, where: "on origin",
			remedy: "delete the remote branch if it holds no work, or run `git fetch --prune` if it is already gone"},
	} {
		code, output, _ := preflight.GitCmd(record.Repo, "rev-parse", "--verify", "--quiet", scope.ref)
		if code != 0 {
			continue
		}
		// GitLab branch prepare가 봉인된 base에 빈 원격 브랜치를 먼저 만드는
		// 순서를 보존한다. 로컬 브랜치나 다른 SHA의 원격 브랜치는 기존처럼
		// 차단하고, 이 정확한 원격 ref만 adapter의 안전한 정규화에 맡긴다.
		if scope.where == "on origin" && exactGitLabPreparedRemote(record, branch, output) {
			continue
		}
		return fmt.Errorf(
			"branch %q already exists %s, so Orca cannot prepare this execution: Orca always creates a new branch, so it would take a different name (observed: a numeric suffix) and fail as worktree_branch_mismatch only after the worktree exists; "+
				"use --mode direct, which adopts the existing branch, or %s",
			branch, scope.where, scope.remedy)
	}
	return nil
}

func exactGitLabPreparedRemote(record issueops.IssueOpsRecord, branch, observedOID string) bool {
	prepared := record.BranchPrepare
	return prepared != nil &&
		strings.EqualFold(strings.TrimSpace(prepared.Provider), "gitlab") &&
		prepared.LinkVerified &&
		strings.TrimSpace(prepared.Branch) == strings.TrimSpace(branch) &&
		strings.TrimSpace(prepared.BaseSHA) != "" &&
		strings.EqualFold(strings.TrimSpace(observedOID), strings.TrimSpace(prepared.BaseSHA))
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

func resolveExecutionPrepareMode(ctx context.Context, record issueops.IssueOpsRecord, req ExecutionPrepareRequest, requested string, orca port.ExecutionOrcaProvisioner) (string, string, port.ExecutionOrcaProbeRequest, error) {
	probeReq := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: strings.ToLower(strings.TrimSpace(req.OwnerHost)),
		Model: strings.TrimSpace(req.OwnerModel), Effort: strings.TrimSpace(req.OwnerEffort),
	}
	if requested == string(issueops.ExecutionModeDirect) {
		return requested, "", probeReq, nil
	}
	if probeReq.Host != "codex" && probeReq.Host != "claude" {
		return "", "", probeReq, fmt.Errorf("Orca owner_host must be codex or claude")
	}
	if orca == nil {
		if requested == ExecutionModeAuto {
			return string(issueops.ExecutionModeDirect), "orca_adapter_unavailable", probeReq, nil
		}
		return "", "", probeReq, fmt.Errorf("Orca provisioner is unavailable")
	}
	issueIdentity, err := orcaLaunchIssueIdentity(record)
	if err != nil {
		return "", "", probeReq, err
	}
	probeReq.Provider = issueIdentity.Provider
	probeReq.Issue = issueIdentity.Issue
	probeReq.Marker, err = renderOrcaReadinessMarker(record.ID, issueIdentity)
	if err != nil {
		return "", "", probeReq, err
	}
	probe, err := orca.Probe(ctx, probeReq)
	if err != nil || !probe.Available || !probe.Ready {
		code := strings.TrimSpace(probe.Code)
		if code == "" {
			code = "orca_probe_failed"
		}
		if requested == ExecutionModeAuto {
			return string(issueops.ExecutionModeDirect), code, probeReq, nil
		}
		if err != nil {
			return "", "", probeReq, fmt.Errorf("Orca probe failed: %w", err)
		}
		return "", "", probeReq, fmt.Errorf("Orca probe failed: %s", code)
	}
	// Orca는 준비됐지만 브랜치 이름이 이미 쓰이고 있으면 이 경로는 실행할 수 없다.
	// IssueOps 정식 순서(`gh issue develop` → `branch prepare`)를 따르면 항상 그렇게
	// 되므로, auto가 여기서 Orca를 확정하면 사전 확인에 막힌 뒤 사용자가 손으로
	// --mode direct를 다시 줘야 한다 — auto가 실행 가능한 모드를 고르지 못한 것이다
	// (이슈 #152).
	//
	// 같은 함수를 재사용해 폴백 판정과 실제 차단 기준을 하나로 유지한다. 두 곳에
	// 조건을 따로 쓰면 한쪽만 고쳐져 auto가 direct로 갔는데 direct도 막히거나 그
	// 반대가 된다.
	if err := ensureOrcaBranchIsFree(record, strings.TrimSpace(record.Branch)); err != nil {
		const code = "orca_branch_name_taken"
		if requested == ExecutionModeAuto {
			return string(issueops.ExecutionModeDirect), code, probeReq, nil
		}
		// 명시적으로 Orca를 고른 사용자의 의도는 대신 바꾸지 않는다. 원인과 다음
		// 행동을 담은 사전 확인 메시지를 그대로 전한다.
		return "", "", probeReq, err
	}
	return string(issueops.ExecutionModeOrca), "", probeReq, nil
}

// executionWriterAbsentNextCommand는 준비된 실행에 lease writer가 없을 때 그
// 사실과 해소 명령을 돌려준다. writer가 있으면 nil이다.
//
// 상태별로 다음 행동이 다르다. claimable은 토큰이 발급돼 있으므로 claim이
// 바로 되고, released는 토큰이 없어 reseed로 재봉인해야 하며, revoking은 이전
// 홀더가 죽어야 finalize할 수 있다. 하나의 문구로 뭉뚱그리면 사용자가 어느
// 명령을 써야 할지 모른다(이슈 #170).
func executionWriterAbsentNextCommand(record issueops.IssueOpsRecord, confirm bool) (string, error) {
	if !confirm {
		return "", nil
	}
	lease := record.Execution.Lease
	generation := lease.Generation
	switch lease.Status {
	case issueops.LeaseStatusClaimable:
		if record.Execution.Mode == issueops.ExecutionModeOrca &&
			(record.Execution.Orca == nil || record.Execution.Orca.LeaseGeneration != generation) {
			next := ExecutionResumeRecoveryCommand(record.ID, generation)
			return next, fmt.Errorf(
				"IssueOps execution is prepared but Orca generation %d has no current owner; run %s",
				generation, next)
		}
		next := executionWriterAbsentRecoveryCommand(record)
		return next, fmt.Errorf(
			"IssueOps execution is prepared but generation %d is claimable and has no writer; run %s",
			generation, next)
	case issueops.LeaseStatusReleased:
		next := executionWriterAbsentRecoveryCommand(record)
		return next, fmt.Errorf(
			"IssueOps execution is prepared but generation %d was released and has no writer; preview resealing with %s",
			generation, next)
	case issueops.LeaseStatusRevoking:
		next := executionWriterAbsentRecoveryCommand(record)
		return next, fmt.Errorf(
			"IssueOps execution generation %d is revoking and has no writer; finalize the revocation with %s",
			generation, next)
	default:
		return "", nil
	}
}

func executionWorkspaceRequest(record issueops.IssueOpsRecord, confirm bool) (port.ExecutionWorkspaceRequest, error) {
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("verified branch preparation with base_sha is required")
	}
	branch := strings.TrimSpace(record.Branch)
	leaf := strings.ReplaceAll(branch, "/", "-")
	if leaf == "" || leaf == "." || leaf == ".." {
		return port.ExecutionWorkspaceRequest{}, fmt.Errorf("execution branch is invalid")
	}
	parentWorktree := strings.TrimSpace(record.BranchPrepare.ParentWorktree)
	hasDelegatedParent := record.Delegation != nil && strings.TrimSpace(record.Delegation.ParentCycleID) != ""
	if parentWorktree != "" || hasDelegatedParent {
		parentLeaf := strings.ReplaceAll(strings.TrimSpace(record.BranchPrepare.BaseBranch), "/", "-")
		if parentLeaf == "" || parentLeaf == "." || parentLeaf == ".." {
			return port.ExecutionWorkspaceRequest{}, fmt.Errorf("parent execution base branch is invalid")
		}
		expectedParent := filepath.Join(record.Repo+".worktrees", parentLeaf)
		if parentWorktree == "" {
			parentWorktree = expectedParent
		} else {
			parentWorktree = filepath.Clean(parentWorktree)
			if !samePath(parentWorktree, expectedParent) {
				return port.ExecutionWorkspaceRequest{}, fmt.Errorf(
					"parent_worktree %q does not match canonical parent worktree %q",
					parentWorktree, expectedParent,
				)
			}
		}
	}
	return port.ExecutionWorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: filepath.Join(record.Repo+".worktrees", leaf),
		Branch: branch, BaseBranch: strings.TrimSpace(record.BranchPrepare.BaseBranch),
		BaseHead: strings.TrimSpace(record.BranchPrepare.BaseSHA), ParentWorktree: parentWorktree, Confirm: confirm,
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

func executionRecordWorkspaceRoot(record issueops.IssueOpsRecord) string {
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

func renderExecutionOwnerReportContract(record issueops.IssueOpsRecord, req ExecutionPrepareRequest) string {
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
	case string(issueops.ExecutionModeDirect), string(issueops.ExecutionModeOrca):
		return normalized, nil
	default:
		return "", fmt.Errorf("execution mode must be auto, direct, or orca")
	}
}

func workspaceFromReceipt(receipt port.ExecutionWorkspaceReceipt, linkedAt string) issueops.Workspace {
	return issueops.Workspace{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
		BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree,
		Driver: receipt.Driver, LinkedAt: linkedAt,
	}
}

func preparedExecutionResult(record issueops.IssueOpsRecord, requested string) ExecutionPrepareResult {
	return preparedExecutionResultWithModes(record, requested, "")
}

func preparedExecutionResultWithModes(record issueops.IssueOpsRecord, requested, fallback string) ExecutionPrepareResult {
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
