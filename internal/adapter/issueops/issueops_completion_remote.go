package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/port"
)

// completionArtifactNames는 워크트리 artifact 디렉토리에서 보존 대상으로
// 수집하는 파일 목록이다(설계 v5 WS2/WS4).
var completionArtifactNames = []string{"plan", "spec", "verified-execution-loop"}

// completionTuringSummaryLimit bounds the verified-execution summary excerpt; full texts of
// plan/spec travel as collapsible blocks with their own provider-side limit.
const completionTuringSummaryLimit = 4 * 1024

// ReflectIssueCompletion writes the completion section into the linked remote
// issue body. merged must carry caller-verified provider readback evidence
// (the same discipline as cleanup close-children); without it the write is
// rejected. On a confirmed successful update it stamps
// RemoteCompletion.ReflectedAt as a local cache of the remote state.
func ReflectIssueCompletion(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueops.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, port.IssueProviderUpdateIssueBodySectionResult{}, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("cannot reflect completion before a linked issue")
	}
	if record.RemoteArtifact == nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("cannot reflect completion before a verified remote artifact")
	}
	if !merged {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("cannot reflect completion without provider-verified merge evidence")
	}
	completion := gatherCompletionSection(record)
	result, err := prov.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:       record.Repo,
		IssueURL:   record.IssueURL,
		Section:    port.IssueBodySectionCompletion,
		Completion: &completion,
		Confirm:    confirm,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	if !confirm || !result.Updated {
		return record, result, nil
	}
	record, err = stampRemoteCompletion(stateRoot, id, func(rc *issueops.IssueOpsRemoteCompletion, now string) {
		rc.ReflectedAt = now
	})
	return record, result, err
}

// CloseIssueOpsRemoteIssue closes the linked parent issue after the caller
// verified merge evidence by provider readback. On a confirmed verified close
// it stamps RemoteCompletion.IssueClosedAt as a local cache.
func CloseIssueOpsRemoteIssue(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueops.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, port.IssueProviderCloseIssueResult{}, err
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("cannot close before a linked issue")
	}
	if !merged {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderCloseIssueResult{}, fmt.Errorf("cannot close the issue without provider-verified merge evidence")
	}
	result, err := prov.CloseIssue(port.IssueProviderCloseIssueRequest{
		Repo:     record.Repo,
		IssueURL: record.IssueURL,
		Confirm:  confirm,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	if !confirm || !result.Closed {
		return record, result, nil
	}
	record, err = stampRemoteCompletion(stateRoot, id, func(rc *issueops.IssueOpsRemoteCompletion, now string) {
		// AlreadyClosed 재실행이 최초 close 시각을 덮지 않게 멱등으로 둔다(C3-F8).
		if rc.IssueClosedAt == "" {
			rc.IssueClosedAt = now
		}
	})
	return record, result, err
}

func stampRemoteCompletion(stateRoot, id string, apply func(*issueops.IssueOpsRemoteCompletion, string)) (issueops.IssueOpsRecord, error) {
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if rec.RemoteCompletion == nil {
			rec.RemoteCompletion = &issueops.IssueOpsRemoteCompletion{}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		apply(rec.RemoteCompletion, now)
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

// gatherCompletionSection assembles the completion payload from durable record
// evidence and the on-disk artifact directory. 파일 부재는 에러가 아니라 빈
// 블록이다 — 섹션의 블록 헤딩 7종은 렌더러가 placeholder로 보존한다.
func gatherCompletionSection(record issueops.IssueOpsRecord) port.IssueProviderCompletionSection {
	completion := port.IssueProviderCompletionSection{}
	if record.RemoteArtifact != nil {
		completion.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	root := strings.TrimSpace(record.Repo)
	if record.Execution != nil {
		if record.Execution.Completion != nil {
			completion.FinalHead = record.Execution.Completion.FinalHead
			completion.VerificationSummary = record.Execution.Completion.Verification
			if completion.RemoteArtifactURL == "" {
				completion.RemoteArtifactURL = record.Execution.Completion.RemoteArtifactURL
			}
		}
		if workspaceRoot := strings.TrimSpace(record.Execution.Workspace.Root); workspaceRoot != "" {
			root = workspaceRoot
		}
	}
	if len(completion.VerificationSummary) == 0 {
		completion.VerificationSummary = record.AISlopCleanVerification
	}
	if root == "" {
		return completion
	}
	for _, name := range completionArtifactNames {
		body, ok := readCompletionArtifact(sealedArtifactPath(record, root, name))
		if !ok {
			// plan은 봉인 필수 아티팩트다. 부재는 조용히 넘기지 않고 섹션에 남긴다(#482).
			if name == "plan" {
				completion.MissingArtifacts = append(completion.MissingArtifacts, name)
			}
			continue
		}
		digest := sha256.Sum256([]byte(body))
		completion.ArtifactManifest = append(completion.ArtifactManifest, port.IssueProviderArtifactDigest{
			Name: name, SHA256: hex.EncodeToString(digest[:]),
		})
		switch name {
		case "plan":
			completion.PlanBody = body
		case "spec":
			completion.SpecBody = body
		case "verified-execution-loop":
			if len(body) > completionTuringSummaryLimit {
				body = body[:completionTuringSummaryLimit] + "\n… (절단)"
			}
			completion.TuringSummary = body
		}
	}
	return completion
}

func readCompletionArtifact(path string) (string, bool) {
	info, err := os.Lstat(path)
	// 형제 리더(execution_owner_context.go)와 동일한 봉인 계약: 0600 정규
	// 파일만 공개면 게시 대상이다. staging을 우회해 이 디렉토리에 놓인 임의
	// 파일이 이슈 본문으로 퍼블리시되는 경로를 차단한다(C3-F3).
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}
