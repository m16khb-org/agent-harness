package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestSchemaChangeDetection(t *testing.T) {
	for _, path := range []string{
		"src/migrations/1730000000-add-index.ts",
		"db/migration/V3__add_column.sql",
		"internal/entity/user.entity.ts",
		"apps/api/src/entities/order.ts",
		"prisma/schema.prisma",
		"scripts/backfill.sql",
	} {
		if !pathIsSchemaChange(path) {
			t.Fatalf("schema-shaped path must be detected: %s", path)
		}
	}
	for _, path := range []string{
		"internal/adapter/issueops/readiness.go",
		"docs/migration-guide.md",
		"skills/issueops/SKILL.md",
		"web/src/components/entityCard.tsx",
	} {
		if pathIsSchemaChange(path) {
			t.Fatalf("non-schema path must not be detected: %s", path)
		}
	}
}

// 스키마 변경이 없는 사이클에서는 게이트 자체가 활성화되지 않는다.
func TestSchemaEvidenceGateInactiveWithoutSchemaChange(t *testing.T) {
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if got := schemaEvidenceMissingForPaths(record, []string{"main.go", "README.md"}, ""); got != "" {
		t.Fatalf("non-schema change set must not activate the gate: %q", got)
	}
}

func TestSchemaEvidenceGateActivatesOnSchemaChange(t *testing.T) {
	paths := []string{"main.go", "src/migrations/1730000000-add-index.ts"}
	record := issueops.IssueOpsRecord{Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect}}
	if got := schemaEvidenceMissingForPaths(record, paths, ""); got != "schema_evidence" {
		t.Fatalf("schema change must activate the gate: %q", got)
	}
	record.SchemaEvidence = &issueops.IssueOpsSchemaEvidence{
		Measurements: []string{"orders row count = 8,412,003"},
		Sources:      []string{"db-bc-prod execute_sql_bc_prod_market"},
	}
	if got := schemaEvidenceMissingForPaths(record, paths, ""); got != "" {
		t.Fatalf("recorded measurements must clear the gate: %q", got)
	}
	record.SchemaEvidence.ReviewedFingerprint = "old"
	if got := schemaEvidenceMissingForPaths(record, paths, "new"); got != "schema_evidence_stale" {
		t.Fatalf("drifted fingerprint must be stale: %q", got)
	}
	record.SchemaEvidence = &issueops.IssueOpsSchemaEvidence{Waived: true, WaiverRationale: "read-only view, no index impact"}
	if got := schemaEvidenceMissingForPaths(record, paths, ""); got != "" {
		t.Fatalf("waived evidence must clear the gate: %q", got)
	}
	record.SchemaEvidence = &issueops.IssueOpsSchemaEvidence{Waived: true}
	if got := schemaEvidenceMissingForPaths(record, paths, ""); got != "schema_evidence" {
		t.Fatalf("waiver without rationale must not clear the gate: %q", got)
	}
}

func TestRecordIssueOpsSchemaEvidenceValidation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := gitRepoWithProjectDocsForTest(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "92-schema"})
	if err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = IssueOpsPhaseImplement })

	if _, err := RecordIssueOpsSchemaEvidence(stateRoot, record.ID, IssueOpsSchemaEvidenceRequest{}); err == nil {
		t.Fatal("empty request must be rejected")
	}
	if _, err := RecordIssueOpsSchemaEvidence(stateRoot, record.ID, IssueOpsSchemaEvidenceRequest{
		Measurements: []string{"row count 12"},
	}); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("measurement without a source must be rejected: %v", err)
	}
	if _, err := RecordIssueOpsSchemaEvidence(stateRoot, record.ID, IssueOpsSchemaEvidenceRequest{Waive: true}); err == nil {
		t.Fatal("waiver without rationale must be rejected")
	}
	got, err := RecordIssueOpsSchemaEvidence(stateRoot, record.ID, IssueOpsSchemaEvidenceRequest{
		Measurements: []string{"idx_orders_user_id 미존재, orders 8.4M rows"},
		Sources:      []string{"mcp db-bc-prod execute_sql_bc_prod_market"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaEvidence == nil || got.SchemaEvidence.ReviewedFingerprint == "" {
		t.Fatalf("schema evidence must bind the reviewed fingerprint: %+v", got.SchemaEvidence)
	}
}

// 경량 readiness는 record만 읽는다 — 변경 집합 조회가 필요한 schema 게이트는
// strict가 소유한다. 이 경계가 무너지면 status ledger 파생이 git을 돌린다.
func TestPRReadinessSurfacesDocsGateButNotSchemaGate(t *testing.T) {
	repo := gitRepoWithSchemaChangeForTest(t)
	record := issueops.IssueOpsRecord{
		Repo:      repo,
		Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect},
	}
	ready := IssueOpsPRReadiness(record)
	if !containsString(ready.Missing, "project_docs_review") {
		t.Fatalf("non-strict readiness must still surface the docs gate: %+v", ready.Missing)
	}
	if containsString(ready.Missing, "schema_evidence") {
		t.Fatalf("non-strict readiness must not read the change set: %+v", ready.Missing)
	}
	if got := schemaEvidenceMissing(record, ""); got != "schema_evidence" {
		t.Fatalf("strict-side gate must activate on the schema change: %q", got)
	}
}

func gitRepoWithSchemaChangeForTest(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if code, _, stderr := preflightGitForReviewTest(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeRepoFileForTest(t, repo, "src/migrations/1730000000-add-index.ts", "export class AddIndex {}\n")
	return repo
}
