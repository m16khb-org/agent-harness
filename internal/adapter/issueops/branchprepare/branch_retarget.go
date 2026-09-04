package branchprepare

import (
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
)

// Retarget은 준비된 base를 provider가 실제로 보여주는 PR/MR target으로 바꾸고 그
// 이력을 남긴다. 자식 MR이 우산 브랜치로 재타깃되는 흐름처럼 base 변경은 정당한
// 결정이지만, cleanup finish는 레코드에 없는 결정을 인정하지 않는다
// (base_branch_drifted). 그래서 finish에 면제 플래그를 두는 대신 결정 자체를
// finish 전에 관측 증거와 함께 기록한다.
//
// 요청한 base는 두 관측을 모두 통과해야 한다: artifact readback의 target이
// 그 base이고, origin에 그 브랜치가 존재한다. 어느 관측이든 실패하면 통과가
// 아니라 거부다.
func Retarget(store Store, stateRoot, id string, req model.IssueOpsBranchRetargetRequest) (model.IssueOpsRecord, error) {
	baseBranch := strings.TrimSpace(req.BaseBranch)
	if baseBranch == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_branch is required")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("reason is required")
	}
	if store.ObserveArtifactTargetBranch == nil || store.RemoteBranchPresent == nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("retarget observation is unavailable")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.BranchPrepare == nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("branch must be prepared before retarget")
	}
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("a verified remote artifact is required before retarget")
	}
	fromBase := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if fromBase == baseBranch {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_branch %q is already the prepared base", baseBranch)
	}
	observed, err := store.ObserveArtifactTargetBranch(*record.RemoteArtifact)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact target observation failed: %w", err)
	}
	if strings.TrimSpace(observed) != baseBranch {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("remote artifact targets %q, not %q", observed, baseBranch)
	}
	present, err := store.RemoteBranchPresent(record.Repo, baseBranch)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("origin observation failed: %w", err)
	}
	if !present {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("base_branch %q is absent from origin", baseBranch)
	}
	record.BranchPrepare.Retargets = append(record.BranchPrepare.Retargets, model.IssueOpsBranchRetarget{
		FromBase: fromBase, ToBase: baseBranch, Reason: reason,
		ArtifactURL: strings.TrimSpace(record.RemoteArtifact.URL),
		ObservedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	})
	record.BranchPrepare.BaseBranch = baseBranch
	record.RemoteArtifact.TargetBranch = baseBranch
	return store.TouchWrite(stateRoot, record)
}
