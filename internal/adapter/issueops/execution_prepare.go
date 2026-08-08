package issueops

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/issueops/pathutil"
	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

const ExecutionModeAuto = "auto"

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
		code, output, _ := GitCmd(record.Repo, "rev-parse", "--verify", "--quiet", scope.ref)
		if code != 0 {
			continue
		}
		// branch prepare가 봉인된 base에 linked 원격 브랜치를 먼저 만드는 순서를
		// 보존한다. 로컬 브랜치나 다른 SHA의 원격 브랜치는 기존처럼 차단하고,
		// 이 정확한 원격 ref만 adapter의 안전한 정규화에 맡긴다.
		//
		// 안전 근거는 provider 이름이 아니라 **원격 tip이 봉인된 base 그대로여서
		// 잃을 작업이 없다**는 관측이다. 처음에는 GitLab만 예외로 두었는데,
		// GitHub도 documented flow가 같은 순서(branch prepare --link-verified →
		// linked branch → execution prepare --mode orca)를 지시하므로 그 순서를
		// 따르는 사용자가 GitHub에서만 막혔다(#319).
		if scope.where == "on origin" && exactPreparedRemoteAtSealedBase(record, branch, output) {
			continue
		}
		return fmt.Errorf(
			"branch %q already exists %s, so Orca cannot prepare this execution: Orca always creates a new branch, so it would take a different name (observed: a numeric suffix) and fail as worktree_branch_mismatch only after the worktree exists; "+
				"use --mode direct with an explicit --direct-reason, which adopts the existing branch, or %s",
			branch, scope.where, scope.remedy)
	}
	return nil
}

// exactPreparedRemoteAtSealedBase는 원격 브랜치가 이 lifecycle이 준비한 linked
// branch이고, tip이 봉인된 base SHA 그대로인지 보고한다.
//
// 네 조건을 모두 요구한다: link가 검증됐고, 기록된 branch 이름과 같고, base SHA가
// 봉인돼 있고, 관측된 원격 tip이 그 SHA와 정확히 같아야 한다. 커밋이 하나라도
// 올라가 있으면 OID가 달라져 채택되지 않으므로, 이 경로가 작업을 잃는 문을
// 열지 않는다.
func exactPreparedRemoteAtSealedBase(record issueops.IssueOpsRecord, branch, observedOID string) bool {
	prepared := record.BranchPrepare
	return prepared != nil &&
		prepared.LinkVerified &&
		strings.TrimSpace(prepared.Branch) == strings.TrimSpace(branch) &&
		strings.TrimSpace(prepared.BaseSHA) != "" &&
		strings.EqualFold(strings.TrimSpace(observedOID), strings.TrimSpace(prepared.BaseSHA))
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

func workspaceFromReceipt(receipt port.ExecutionWorkspaceReceipt, linkedAt string) issueops.Workspace {
	return issueops.Workspace{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
		BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree,
		Driver: receipt.Driver, LinkedAt: linkedAt,
	}
}

func executionNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}
