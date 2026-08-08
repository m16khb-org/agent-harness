package issueops

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/contract/issueops"
	issueopsdomain "agent-harness/internal/domain/issueops"
	"agent-harness/internal/domain/issueopsremote"
	"agent-harness/internal/port"
)

// CleanupRemoteBranchDeps는 외부 표면 주입점이다.
//
// Git은 sync-base와 같은 ctx 서명을 쓴다 — 원격 관측과 삭제가 모두 네트워크
// 호출이므로 비대화 env와 timeout 계약이 필수다(기본 구현 재사용).
// VerifyMergedArtifact는 머지 여부와 head 정체를 한 번의 readback으로 함께
// 돌려준다. 게이트 ⑧·⑨·⑩이 서로 다른 시점의 관측을 섞지 않으려면 이 셋이
// 같은 응답에서 나와야 한다.
type CleanupRemoteBranchDeps struct {
	Git                  func(ctx context.Context, dir string, args ...string) (int, string)
	VerifyMergedArtifact func(artifact issueops.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error)
	// ReflectAudit는 삭제 성공 사실을 이슈 본문 completion 섹션에 멱등 병합한다
	// (finish ④'의 CleanupAudit 병합 선례). best-effort이며 실패해도 이미 끝난
	// 원격 삭제를 되돌리지 않는다.
	ReflectAudit func(record issueops.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string) error
	// ObserveArtifact는 replacement 증거를 provider에서 읽는다. 주입되지 않으면
	// 그 경로는 열리지 않는다 — 관측 없이 증거를 인정하지 않는다(#323).
	ObserveArtifact func(url string) (issueopsdomain.ArtifactObservation, error)
}

// cleanupRemoteBranchInventory는 fingerprint 입력이 되는 현재 관측 상태다.
// 관측 원격 OID가 들어가므로 preview 이후 원격 tip이 움직이면 fingerprint가
// 달라져 apply가 멈춘다.
type cleanupRemoteBranchInventory struct {
	ID          string `json:"id"`
	Repo        string `json:"repo"`
	Branch      string `json:"branch"`
	RemoteOID   string `json:"remote_oid"`
	ArtifactURL string `json:"artifact_url"`
	// SupersededBy는 replacement 증거를 fingerprint 입력에 포함시킨다.
	SupersededBy string `json:"superseded_by,omitempty"`
}

// CleanupRemoteBranch는 게이트 12종을 fail-closed로 평가하고, apply에서
// 관측 OID에 결속된 force-with-lease 삭제를 수행한다.
func CleanupRemoteBranch(ctx context.Context, stateRoot string, req CleanupRemoteBranchRequest, deps CleanupRemoteBranchDeps) (CleanupRemoteBranchResult, error) {
	if deps.Git == nil {
		deps.Git = defaultExecutionSyncBaseGit
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return CleanupRemoteBranchResult{OK: false, ID: req.ID}, err
	}
	result := CleanupRemoteBranchResult{OK: true, ID: record.ID, Preview: !req.Apply}
	inventory, missing := cleanupRemoteBranchGates(ctx, record, req, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("cleanup remote-branch is not ready: %s", strings.Join(missing, ", "))
	}
	// 평가 순서 ②: 원격 브랜치가 이미 없으면 fingerprint stale 검사 이전에
	// 즉시 멱등 성공이다. 성공한 삭제 뒤의 재실행이 stale로 막히면 멱등성과
	// TOCTOU 방어가 서로를 무효화한다(brooks B1).
	if !result.RemoteBranchPresent {
		result.AlreadyAbsent = true
		return result, nil
	}
	fingerprint, err := cleanupRemoteBranchFingerprint(inventory)
	if err != nil {
		return CleanupRemoteBranchResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"agent-harness issueops cleanup remote-branch --id %s --apply --confirm --fingerprint %s --json", record.ID, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("cleanup remote-branch --apply requires --confirm")
	}
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	// 파괴 이전에 보존 payload를 스냅샷한다(finish C2-F1 선례).
	completionSnapshot := gatherCompletionSection(record)
	// fully-qualified ref는 동명 태그를 배제하고, force-with-lease는 preview→push
	// 사이에 남은 TOCTOU를 서버측에서 원자적으로 봉쇄한다(brooks H7).
	ref := "refs/heads/" + inventory.Branch
	if code, out := deps.Git(ctx, record.Repo, "push", "origin", "--delete", ref,
		"--force-with-lease="+ref+":"+inventory.RemoteOID); code != 0 {
		result.OK = false
		result.FailedStep = "remote_branch_delete"
		result.NextCommand = fmt.Sprintf("agent-harness issueops cleanup remote-branch --id %s --preview --json", record.ID)
		return result, fmt.Errorf("git push origin --delete %s failed (remote unchanged; re-run preview then apply): %s",
			ref, strings.TrimSpace(out))
	}
	result.Deleted = true
	result.DeletedAt = time.Now().UTC().Format(time.RFC3339)
	if deps.ReflectAudit != nil {
		audit := fmt.Sprintf("원격 브랜치 삭제: branch=%s oid=%s at=%s", inventory.Branch, inventory.RemoteOID, result.DeletedAt)
		if err := deps.ReflectAudit(record, completionSnapshot, audit); err == nil {
			result.AuditReflected = true
		} else {
			// best-effort지만 무흔적 실패는 금지 — 결과에 표면화한다.
			result.AuditError = err.Error()
		}
	}
	return result, nil
}

// cleanupRemoteBranchGates는 게이트 12종을 전부 평가하고 missing을 나열한다
// (첫 실패에 멈추지 않는다 — 한 번의 preview로 모든 결격 사유를 본다).
func cleanupRemoteBranchGates(ctx context.Context, record issueops.IssueOpsRecord, req CleanupRemoteBranchRequest, deps CleanupRemoteBranchDeps, result *CleanupRemoteBranchResult) (cleanupRemoteBranchInventory, []string) {
	missing := []string{}
	inventory := cleanupRemoteBranchInventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(record.Branch)}
	if record.Execution != nil {
		if branch := strings.TrimSpace(record.Execution.Workspace.Branch); branch != "" {
			inventory.Branch = branch
		}
	}
	// ① branch_recorded / ② branch_name_revalidated — 삭제 직전 재검증은
	// 수동 편집된 레코드가 임의 ref를 지우는 경로를 막는다(brooks M11).
	if inventory.Branch == "" {
		missing = append(missing, "branch_recorded")
	} else if err := validateIssueOpsIssueBranch(inventory.Branch); err != nil {
		missing = append(missing, "branch_name_revalidated")
	}
	// ③ branch_not_base — base 브랜치 삭제 방어.
	if inventory.Branch != "" && record.BranchPrepare != nil &&
		inventory.Branch == strings.TrimSpace(record.BranchPrepare.BaseBranch) {
		missing = append(missing, "branch_not_base")
	}
	result.Branch = inventory.Branch
	// ④ phase_done — cleanup 명령군 순서 강제(brooks H5).
	if record.Phase != IssueOpsPhaseDone {
		missing = append(missing, "phase_done")
	}
	// ⑤ lease_released — active lease 사이클의 브랜치를 지우면 sync-base가
	// 영구히 막힌다.
	if record.Execution != nil && record.Execution.Lease.Status != issueops.LeaseStatusReleased {
		missing = append(missing, "lease_released")
	}
	// ⑥ child_tasks_closed.
	for _, link := range record.IssueLinks {
		if link.Type == "child" && strings.TrimSpace(link.CloseVerifiedAt) == "" {
			missing = append(missing, "child_tasks_closed")
			break
		}
	}
	// ⑦ remote_artifact_present.
	if record.RemoteArtifact == nil {
		missing = append(missing, "remote_artifact_present")
	} else {
		inventory.ArtifactURL = strings.TrimSpace(record.RemoteArtifact.URL)
		missing = append(missing, cleanupRemoteBranchArtifactGates(ctx, record, inventory, deps, result)...)
	}
	// ⑫ remote_branch_readable — 3분류: exit 0 + 비어있지 않음=present(OID 관측),
	// exit 0 + 빈 출력=absent, 그 외=unreadable(fail-closed).
	if inventory.Branch != "" {
		code, out := deps.Git(ctx, record.Repo, "ls-remote", "--heads", "origin", "refs/heads/"+inventory.Branch)
		if code != 0 {
			missing = append(missing, "remote_branch_readable")
		} else if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 {
			inventory.RemoteOID = fields[0]
			result.RemoteOID, result.RemoteBranchPresent = inventory.RemoteOID, true
		}
	}
	// ⑩ remote_tip_equals_merged_head — 관측 OID가 머지된 head와 다르면 머지
	// 이후 push된 커밋이 있다는 뜻이다. 부재 경로에는 비교 대상이 없다.
	//
	// 통과 경로가 둘이다. OID CAS가 첫 경로이고 squash 머지를 포함해 종전 그대로
	// 동작한다. 그것이 실패하면 원격 tip이 이미 base에 도달했는지 본다 — 한
	// 사이클이 PR을 두 개 낳으면 두 번째 PR의 커밋이 레코드의 단일 아티팩트에
	// 담기지 않아 OID CAS만으로는 영구히 막혔고, lease released 탓에 아티팩트
	// 갱신 경로도 닫혀 하네스 안에서 회복할 수 없었다(이슈 #153).
	//
	// ancestry를 OID CAS의 **대체**로 쓰지 않는다. squash 머지에서는 원본 커밋이
	// base의 조상이 아니므로 대체하면 squash된 브랜치를 영구히 못 지운다 — 원래
	// 주석의 brooks B3 기각이 지키려던 것이 그것이다. 추가 경로일 때만 성립한다.
	if result.RemoteBranchPresent {
		switch {
		case result.ArtifactHeadOID != "" && strings.EqualFold(result.ArtifactHeadOID, inventory.RemoteOID):
			// 머지된 그 커밋 그대로다.
		case cleanupRemoteTipReachedBase(ctx, record, inventory, deps):
			result.RemoteTipReachedBase = true
		default:
			// ancestry로도 판정할 수 없는 경우가 있다: 기록된 base 브랜치가 이미
			// 삭제됐거나, 전진분이 base가 아니라 다른 target으로 재통합된 경우다.
			// 그때는 후속 merged artifact의 provider readback이 유일한 근거다(#323).
			if err := verifyRemoteBranchSupersedingArtifact(record, req, deps); err != nil {
				result.SupersedeError = err.Error()
				missing = append(missing, "remote_tip_equals_merged_head")
			} else {
				inventory.SupersededBy = strings.TrimSpace(req.SupersededBy)
				result.SupersededBy = inventory.SupersededBy
			}
		}
	}
	return inventory, missing
}

// cleanupRemoteTipReachedBase는 원격 tip이 준비된 base의 remote-tracking ref의
// 조상인지 확인한다. 조상이면 그 커밋은 이미 base에 있으므로 브랜치를 지워도
// 잃을 것이 없다 — 게이트 ⑩이 막으려는 손해가 실재하지 않는 경우다.
//
// fail-closed다. base 이름을 모르거나 조회가 실패하면 false를 돌려주고 기존
// 판정이 남는다. 관측하지 못한 것을 통과 근거로 쓰면 커밋을 잃는다.
//
// remote-tracking ref와 비교한다. 로컬 브랜치는 fetch 상태에 따라 원격과 어긋날
// 수 있어 판정이 흔들린다. 낡은 ref는 ancestry를 성립시키지 못해 과잉 차단이
// 되는데, 그 방향은 안전하다.
func cleanupRemoteTipReachedBase(ctx context.Context, record issueops.IssueOpsRecord, inventory cleanupRemoteBranchInventory, deps CleanupRemoteBranchDeps) bool {
	if record.BranchPrepare == nil || strings.TrimSpace(inventory.RemoteOID) == "" {
		return false
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return false
	}
	code, _ := deps.Git(ctx, record.Repo, "merge-base", "--is-ancestor", inventory.RemoteOID, "refs/remotes/origin/"+base)
	return code == 0
}

// cleanupRemoteBranchArtifactGates는 artifact 한 번의 readback에 의존하는
// 게이트 ⑧·⑨와 origin 정체 게이트 ⑪을 평가한다.
func cleanupRemoteBranchArtifactGates(ctx context.Context, record issueops.IssueOpsRecord, inventory cleanupRemoteBranchInventory,
	deps CleanupRemoteBranchDeps, result *CleanupRemoteBranchResult) []string {
	missing := []string{}
	// ⑧ remote_artifact_merged — 미머지와 readback 실패 모두 거부다(finish 동형).
	if deps.VerifyMergedArtifact == nil {
		missing = append(missing, "remote_artifact_merged")
		result.ArtifactError = "merge verification is not configured"
	} else if head, err := deps.VerifyMergedArtifact(*record.RemoteArtifact); err != nil {
		missing = append(missing, "remote_artifact_merged")
		result.ArtifactError = err.Error()
	} else {
		result.ArtifactHeadBranch = strings.TrimSpace(head.HeadRefName)
		result.ArtifactHeadOID = strings.TrimSpace(head.HeadRefOID)
		// ⑨ artifact_head_branch_match — 다른 브랜치의 PR을 근거로 이 브랜치를
		// 지우는 경로를 막는다(brooks B4).
		if result.ArtifactHeadBranch == "" || result.ArtifactHeadBranch != inventory.Branch {
			missing = append(missing, "artifact_head_branch_match")
		}
	}
	// ⑪ remote_identity_match — origin은 sync-base 선례로 고정하되, 그 origin이
	// artifact와 다른 프로젝트를 가리키면 거짓 성공이 된다(brooks H6).
	if err := cleanupRemoteBranchIdentityMatch(ctx, record, deps); err != nil {
		missing = append(missing, "remote_identity_match")
		result.RemoteIdentityError = err.Error()
	}
	return missing
}

func cleanupRemoteBranchIdentityMatch(ctx context.Context, record issueops.IssueOpsRecord, deps CleanupRemoteBranchDeps) error {
	artifact := record.RemoteArtifact
	provider := strings.ToLower(strings.TrimSpace(artifact.Provider))
	artifactKey := remote.ProjectKey(strings.TrimSpace(artifact.URL), provider, cleanupRemoteBranchArtifactKind(artifact.Kind))
	if artifactKey == "" {
		return fmt.Errorf("remote artifact URL does not identify one %s project", provider)
	}
	code, out := deps.Git(ctx, record.Repo, "remote", "get-url", "origin")
	if code != 0 {
		return fmt.Errorf("git remote get-url origin: %s", strings.TrimSpace(out))
	}
	originKey, err := remote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(out), provider)
	if err != nil {
		return err
	}
	if originKey != artifactKey {
		return fmt.Errorf("origin project %q does not match the remote artifact project %q", originKey, artifactKey)
	}
	return nil
}

func cleanupRemoteBranchArtifactKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pull_request":
		return "pr"
	case "merge_request":
		return "mr"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func cleanupRemoteBranchFingerprint(inventory cleanupRemoteBranchInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// verifyRemoteBranchSupersedingArtifact는 replacement 증거를 provider readback으로
// 검증한다. finish의 같은 이름 함수와 규칙을 공유하며(domain의
// ValidateSupersedingArtifact), 관측·증거가 없으면 사유를 돌려준다.
func verifyRemoteBranchSupersedingArtifact(record issueops.IssueOpsRecord, req CleanupRemoteBranchRequest, deps CleanupRemoteBranchDeps) error {
	candidate := strings.TrimSpace(req.SupersededBy)
	if candidate == "" {
		return fmt.Errorf("no superseding artifact was provided")
	}
	if deps.ObserveArtifact == nil {
		return fmt.Errorf("superseding artifact cannot be verified: provider observation is not configured")
	}
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		return fmt.Errorf("original artifact URL is unknown; cannot verify a supersede relation")
	}
	replacement, err := deps.ObserveArtifact(candidate)
	if err != nil {
		return fmt.Errorf("superseding artifact %s could not be observed: %w", candidate, err)
	}
	return issueopsdomain.ValidateSupersedingArtifact(issueopsdomain.ArtifactObservation{
		URL:      record.RemoteArtifact.URL,
		Provider: record.RemoteArtifact.Provider,
	}, replacement)
}
