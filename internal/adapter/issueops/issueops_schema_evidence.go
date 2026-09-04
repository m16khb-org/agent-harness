package issueops

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
)

// RecordIssueOpsSchemaEvidence는 스키마·마이그레이션·엔티티 변경 사이클의
// 실측 근거를 기록한다. 인덱스 현황이나 row count처럼 실제 데이터베이스에서
// 관찰한 값과 그 출처를 함께 요구한다 — 출처 없는 수치는 추정과 구분되지
// 않기 때문이다. 관찰이 불가능하면 근거를 적어 waive한다.
func RecordIssueOpsSchemaEvidence(stateRoot, id string, req IssueOpsSchemaEvidenceRequest) (issueops.IssueOpsRecord, error) {
	measurements := cleanReviewValues(req.Measurements)
	sources := cleanReviewValues(req.Sources)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if req.Waive {
		if rationale == "" {
			return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("schema evidence waiver requires --waiver-rationale")
		}
	} else {
		if len(measurements) == 0 {
			return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("schema evidence requires at least one --measurement or an explicit --waive")
		}
		if len(sources) == 0 {
			return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("schema evidence requires at least one --source naming where the measurement was observed")
		}
	}
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if issueOpsPhaseRank(rec.Phase) < issueOpsPhaseRank(issueops.IssueOpsPhaseImplement) {
			return fmt.Errorf("schema evidence can only be recorded from the implement phase onward (current: %s)", rec.Phase)
		}
		fingerprint := implementation.ChangeFingerprint(rec)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.SchemaEvidence = &issueops.IssueOpsSchemaEvidence{
			Measurements: measurements, Sources: sources,
			Waived: req.Waive, WaiverRationale: strings.Join(cleanReviewValues([]string{rationale}), ""),
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

// schemaEvidenceMissing은 변경 집합에 스키마 파일이 있을 때만 활성화되는
// 조건부 게이트다. DB를 쓰지 않는 사이클에서는 아무것도 요구하지 않는다.
func schemaEvidenceMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	if record.Execution == nil {
		return ""
	}
	return schemaEvidenceMissingForPaths(record, implementation.ChangedPaths(record), currentFingerprint)
}

func schemaEvidenceMissingForPaths(record issueops.IssueOpsRecord, changed []string, currentFingerprint string) string {
	if !changeSetTouchesSchema(changed) {
		return ""
	}
	evidence := record.SchemaEvidence
	if evidence == nil {
		return "schema_evidence"
	}
	if evidence.Waived {
		if strings.TrimSpace(evidence.WaiverRationale) == "" {
			return "schema_evidence"
		}
	} else if len(evidence.Measurements) == 0 || len(evidence.Sources) == 0 {
		return "schema_evidence"
	}
	if currentFingerprint != "" && evidence.ReviewedFingerprint != currentFingerprint {
		return "schema_evidence_stale"
	}
	return ""
}

func changeSetTouchesSchema(changed []string) bool {
	for _, rel := range changed {
		if pathIsSchemaChange(rel) {
			return true
		}
	}
	return false
}

// pathIsSchemaChange는 확실한 스키마 신호만 인정한다. 오탐이 나면 DB 없는
// 사이클까지 게이트가 켜지므로, 판단이 갈리는 패턴은 일부러 뺀다.
func pathIsSchemaChange(rel string) bool {
	rel = strings.ToLower(strings.TrimSpace(rel))
	if rel == "" {
		return false
	}
	base := path.Base(rel)
	if strings.HasSuffix(base, ".sql") || base == "schema.prisma" {
		return true
	}
	if strings.HasSuffix(base, ".entity.ts") || strings.HasSuffix(base, ".entity.js") || strings.HasSuffix(base, ".entity.go") {
		return true
	}
	for _, segment := range strings.Split(path.Dir(rel), "/") {
		switch segment {
		case "migrations", "migration", "entities":
			return true
		}
	}
	return false
}
