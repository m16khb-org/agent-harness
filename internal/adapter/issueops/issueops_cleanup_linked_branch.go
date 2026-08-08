package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	linkedbranch "agent-harness/internal/domain/issueopslinkedbranch"
)

// CleanupLinkedBranchDeps는 외부 표면 주입점이다.
//
// 관측과 삭제를 갈라 둔 이유는 이 경로의 안전 근거가 "관측 없이는 아무것도
// 지우지 않는다"이기 때문이다. 관측 함수가 주입되지 않으면 preview조차 열리지
// 않는다 — 삭제 함수만 있고 관측이 없는 조합은 만들 수 없다.
type CleanupLinkedBranchDeps struct {
	Git func(ctx context.Context, dir string, args ...string) (int, string)
	// ObserveLinkedBranches는 이슈의 linked-branch 목록을 읽는다. TotalCount와
	// Nodes를 채워야 하며, 나머지 필드는 호출부가 record에서 채운다.
	ObserveLinkedBranches func(ctx context.Context, issueURL string) (linkedbranch.Observation, error)
	// DeleteLinkedBranch는 노드 id 하나만 지운다. 브랜치 이름을 받지 않는 것이
	// 의도다 — 이름으로 지우는 표면이 있으면 ref 있는 링크도 지울 수 있게 된다.
	DeleteLinkedBranch func(ctx context.Context, issueURL, nodeID string) error
}

// cleanupLinkedBranchInventory는 fingerprint 입력이다. 노드 id와 같은 시점의
// 원격 OID가 함께 들어가므로, preview 이후 링크가 수렴하거나 브랜치가 생기면
// fingerprint가 달라져 apply가 멈춘다(AC-05).
type cleanupLinkedBranchInventory struct {
	ID              string `json:"id"`
	IssueURL        string `json:"issue_url"`
	RequestedBranch string `json:"requested_branch"`
	SealedBase      string `json:"sealed_base"`
	LinkedBranchID  string `json:"linked_branch_id"`
	LinkedCount     int    `json:"linked_count"`
	RemoteRefOID    string `json:"remote_ref_oid"`
}

// CleanupLinkedBranch는 ref-null 고아 linked-branch를 preview → apply+confirm
// +fingerprint로만 정리한다(#306 AC-04·05·06).
func CleanupLinkedBranch(ctx context.Context, stateRoot string, req issueopscontract.CleanupLinkedBranchRequest, deps CleanupLinkedBranchDeps) (issueopscontract.CleanupLinkedBranchResult, error) {
	if deps.Git == nil {
		deps.Git = defaultExecutionSyncBaseGit
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return issueopscontract.CleanupLinkedBranchResult{OK: false, ID: req.ID}, err
	}
	result := issueopscontract.CleanupLinkedBranchResult{OK: true, ID: record.ID, Preview: !req.Apply}

	missing := cleanupLinkedBranchGates(record, deps)
	if len(missing) > 0 {
		result.OK, result.Missing = false, missing
		return result, fmt.Errorf("cleanup linked-branch is not ready: %s", strings.Join(missing, ", "))
	}
	prepare := record.BranchPrepare
	result.IssueURL, result.RequestedBranch, result.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA

	observation, err := deps.ObserveLinkedBranches(ctx, prepare.IssueURL)
	if err != nil {
		result.OK, result.FailedStep, result.ObserveError = false, "observe_linked_branches", err.Error()
		return result, err
	}
	observation.IssueURL, observation.RequestedBranch, observation.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA
	observation.RemoteOID = cleanupLinkedBranchRemoteOID(ctx, record.Repo, prepare.Branch, deps)
	result.LinkedCount, result.RemoteRefOID = observation.TotalCount, observation.RemoteOID

	state, target, reason := linkedbranch.Classify(observation)
	result.State, result.StateReason = string(state), reason

	// 이미 없으면 fingerprint 검사 이전에 멱등 성공이다. 성공한 삭제 뒤의
	// 재실행이 stale로 막히면 멱등성과 TOCTOU 방어가 서로를 무효화한다
	// (remote-branch 정리의 같은 판단을 따른다).
	if state == linkedbranch.StateAbsent {
		result.AlreadyAbsent = true
		result.AuditRecorded, result.AuditError = recordLinkedBranchCleanupAudit(stateRoot, record, result)
		return result, nil
	}
	if !linkedbranch.Deletable(state) {
		// 지울 수 없는 이유를 관측과 함께 남긴다. 이 진단이 없으면 사용자는
		// raw GraphQL 삭제로 우회하게 되고, 그것이 이 이슈가 막으려는 것이다.
		result.OK, result.FailedStep = false, "classify_linked_branch"
		result.AuditRecorded, result.AuditError = recordLinkedBranchCleanupAudit(stateRoot, record, result)
		return result, fmt.Errorf("linked branch cleanup refuses state %s: %s", state, reason)
	}
	result.LinkedBranchID = target.ID

	fingerprint, err := cleanupLinkedBranchFingerprint(cleanupLinkedBranchInventory{
		ID: record.ID, IssueURL: prepare.IssueURL, RequestedBranch: prepare.Branch, SealedBase: prepare.BaseSHA,
		LinkedBranchID: target.ID, LinkedCount: observation.TotalCount, RemoteRefOID: observation.RemoteOID,
	})
	if err != nil {
		result.OK, result.FailedStep = false, "fingerprint"
		return result, err
	}
	result.Fingerprint = fingerprint

	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"agent-harness issueops cleanup linked-branch --id %s --apply --confirm --fingerprint %s --json", record.ID, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK, result.FailedStep = false, "confirm"
		return result, fmt.Errorf("cleanup linked-branch --apply requires --confirm")
	}
	// fingerprint는 방금 다시 관측한 값으로 계산됐다. 사용자가 들고 온 값과
	// 다르면 preview 이후 외부 상태가 움직인 것이다(AC-05).
	if req.Fingerprint != fingerprint {
		result.OK, result.FailedStep = false, "stale_fingerprint"
		result.AuditRecorded, result.AuditError = recordLinkedBranchCleanupAudit(stateRoot, record, result)
		return result, fmt.Errorf("cleanup linked-branch fingerprint is stale: rerun the preview")
	}
	if err := deps.DeleteLinkedBranch(ctx, prepare.IssueURL, target.ID); err != nil {
		result.OK, result.FailedStep = false, "delete_linked_branch"
		result.AuditRecorded, result.AuditError = recordLinkedBranchCleanupAudit(stateRoot, record, result)
		return result, err
	}
	result.Deleted, result.DeletedAt = true, time.Now().UTC().Format(time.RFC3339)
	result.AuditRecorded, result.AuditError = recordLinkedBranchCleanupAudit(stateRoot, record, result)
	return result, nil
}

// cleanupLinkedBranchGates는 관측 이전에 확정할 수 있는 전제만 본다. 외부
// 호출을 하기 전에 막을 수 있는 것은 여기서 막는다.
func cleanupLinkedBranchGates(record issueopscontract.IssueOpsRecord, deps CleanupLinkedBranchDeps) []string {
	var missing []string
	if deps.ObserveLinkedBranches == nil {
		missing = append(missing, "linked_branch_observation_unavailable")
	}
	if deps.DeleteLinkedBranch == nil {
		missing = append(missing, "linked_branch_deletion_unavailable")
	}
	prepare := record.BranchPrepare
	if prepare == nil {
		return append(missing, "branch_prepare_missing")
	}
	// LinkedBranch는 GitHub의 개념이다. GitLab에는 대응물이 없으므로 여기서
	// 멈춘다 — 다른 provider에서 이 경로가 열리면 무엇을 지울지 정의되지 않는다.
	if prepare.Provider != "github" {
		missing = append(missing, "linked_branch_cleanup_is_github_only")
	}
	if strings.TrimSpace(prepare.IssueURL) == "" {
		missing = append(missing, "issue_url_missing")
	}
	if strings.TrimSpace(prepare.Branch) == "" {
		missing = append(missing, "branch_missing")
	}
	if strings.TrimSpace(prepare.BaseSHA) == "" {
		missing = append(missing, "sealed_base_missing")
	}
	if strings.TrimSpace(record.Repo) == "" {
		missing = append(missing, "repo_missing")
	}
	return missing
}

// cleanupLinkedBranchRemoteOID는 요청 브랜치의 원격 ref를 같은 시점에 읽는다.
// 읽지 못하면 빈 값이고, 분류기는 빈 값을 "브랜치 없음"으로 다룬다.
func cleanupLinkedBranchRemoteOID(ctx context.Context, repo, branch string, deps CleanupLinkedBranchDeps) string {
	code, out := deps.Git(ctx, repo, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	if code != 0 {
		return ""
	}
	for line := range strings.Lines(strings.TrimSpace(out)) {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[1] == "refs/heads/"+branch {
			return fields[0]
		}
	}
	return ""
}

func cleanupLinkedBranchFingerprint(inventory cleanupLinkedBranchInventory) (string, error) {
	encoded, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// recordLinkedBranchCleanupAudit은 처분을 durable record에 남긴다(AC-06).
//
// 성공만이 아니라 이미 부재, 모호성, stale fingerprint도 남긴다. 다음 사람이
// "왜 아직 안 지워졌나"를 처음부터 다시 조사하지 않으려면 거절도 기록이어야
// 한다. 기록 실패는 이미 끝난 외부 삭제를 되돌리지 않으므로 best-effort다.
func recordLinkedBranchCleanupAudit(stateRoot string, record issueopscontract.IssueOpsRecord, result issueopscontract.CleanupLinkedBranchResult) (bool, string) {
	record.LinkedBranchCleanup = &issueopscontract.IssueOpsLinkedBranchCleanup{
		State:          result.State,
		StateReason:    result.StateReason,
		LinkedBranchID: result.LinkedBranchID,
		LinkedCount:    result.LinkedCount,
		RemoteRefOID:   result.RemoteRefOID,
		Fingerprint:    result.Fingerprint,
		Deleted:        result.Deleted,
		AlreadyAbsent:  result.AlreadyAbsent,
		FailedStep:     result.FailedStep,
		ObservedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		return false, err.Error()
	}
	return true, ""
}
