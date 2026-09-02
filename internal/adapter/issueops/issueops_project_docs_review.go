package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/issueops/implementation"
	"agent-harness/internal/contract/issueops"
)

// RecordIssueOpsProjectDocsReview는 publication 직전 project-doc 반영 판정을
// 기록한다. verdict가 updated면 적어 낸 문서가 실제 변경 집합 안에 있어야
// 하므로, 문서를 고치지 않고 "갱신했다"고 기록하는 경로가 막힌다.
func RecordIssueOpsProjectDocsReview(stateRoot, id string, req IssueOpsProjectDocsReviewRequest) (issueops.IssueOpsRecord, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if verdict != "updated" && verdict != "no-change" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("project docs review verdict must be updated|no-change")
	}
	docs := cleanReviewValues(req.Docs)
	evidence := cleanReviewValues(req.Evidence)
	if len(evidence) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("project docs review requires at least one evidence entry")
	}
	if verdict == "updated" && len(docs) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("project docs review verdict updated requires at least one --doc path")
	}
	if verdict == "no-change" && len(docs) > 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("project docs review verdict no-change must not list updated docs")
	}
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if issueOpsPhaseRank(rec.Phase) < issueOpsPhaseRank(issueops.IssueOpsPhaseImplement) {
			return fmt.Errorf("project docs review can only be recorded from the implement phase onward (current: %s)", rec.Phase)
		}
		// fingerprint를 계산할 수 없는 사이클(비-git worktree 등)도 판정 자체는
		// 기록할 수 있다 — ai_slop_clean과 같은 관용이다. 빈 채로 봉인하면
		// 나중에 fingerprint가 생겼을 때 stale로 잡혀 재기록을 요구한다.
		fingerprint := implementation.ChangeFingerprint(rec)
		normalized, e := normalizeProjectDocPaths(rec, docs)
		if e != nil {
			return e
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.ProjectDocsReview = &issueops.IssueOpsProjectDocsReview{
			Verdict: verdict, Docs: normalized, Evidence: evidence,
			ReviewedFingerprint: fingerprint,
			RecordedAt:          now,
		}
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

// normalizeProjectDocPaths는 입력 경로를 repo-상대 slash 경로로 정규화하고,
// 각 경로가 현재 변경 집합에 실제로 들어 있는지 확인한다.
func normalizeProjectDocPaths(record issueops.IssueOpsRecord, docs []string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	root := issueOpsStrictGitRoot(record)
	changed := map[string]bool{}
	for _, path := range implementation.ChangedPaths(record) {
		changed[path] = true
	}
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		rel := relativeChangePath(root, doc)
		if rel == "" {
			return nil, fmt.Errorf("project docs review path %q must be inside the worktree", doc)
		}
		if !changed[rel] {
			return nil, fmt.Errorf("project docs review lists %s but it is not in the current change set", rel)
		}
		out = append(out, rel)
	}
	return out, nil
}

func relativeChangePath(root, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if root == "" {
			return ""
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return ""
		}
		path = rel
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
}

// projectDocsReviewMissing은 publication 게이트 판정이다. implementation review와
// 달리 execution mode를 가리지 않는다 — direct 모드 사이클도 운영 문서에 남길
// 결정을 만들 수 있기 때문이다.
func projectDocsReviewMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	if record.Execution == nil {
		return ""
	}
	review := record.ProjectDocsReview
	if review == nil {
		return "project_docs_review"
	}
	if currentFingerprint != "" && review.ReviewedFingerprint != currentFingerprint {
		return "project_docs_review_stale"
	}
	return ""
}
