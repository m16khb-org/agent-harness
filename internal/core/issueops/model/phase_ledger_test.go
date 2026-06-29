package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// Layer 1 of the phase-ledger design: the model carries an additive phase_ledger
// plus the net-new source-of-truth fields, all omitempty so legacy records and
// golden fixtures stay byte-compatible.

func TestIssueOpsPhaseLedgerEntryRoundTrip(t *testing.T) {
	ledger := IssueOpsPhaseLedger{
		IssueOpsPhaseProblem: IssueOpsPhaseLedgerEntry{
			Phase:       IssueOpsPhaseProblem,
			EnteredAt:   "2026-06-29T00:00:00Z",
			CompletedAt: "2026-06-29T00:01:00Z",
			Artifacts:   []string{"intent_contract"},
			Missing:     nil,
			Notes:       []string{"derived"},
		},
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	var got IssueOpsPhaseLedger
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal ledger: %v", err)
	}
	entry, ok := got[IssueOpsPhaseProblem]
	if !ok {
		t.Fatalf("expected problem entry, got %#v", got)
	}
	if entry.Phase != IssueOpsPhaseProblem {
		t.Fatalf("entry phase should equal its map key, got %q", entry.Phase)
	}
	if entry.CompletedAt != "2026-06-29T00:01:00Z" || len(entry.Artifacts) != 1 || entry.Artifacts[0] != "intent_contract" {
		t.Fatalf("unexpected round-tripped entry: %#v", entry)
	}
}

func TestIssueOpsPhaseLedgerEntryOmitsEmptyFields(t *testing.T) {
	entry := IssueOpsPhaseLedgerEntry{Phase: IssueOpsPhaseGrill, EnteredAt: "2026-06-29T00:00:00Z"}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	s := string(data)
	for _, key := range []string{"completed_at", "artifacts", "missing", "notes"} {
		if strings.Contains(s, key) {
			t.Fatalf("empty entry should omit %q, got %s", key, s)
		}
	}
	if !strings.Contains(s, "\"phase\":\"grill\"") || !strings.Contains(s, "entered_at") {
		t.Fatalf("entry should retain phase+entered_at, got %s", s)
	}
}

func TestIssueOpsRecordNewFieldsOmitEmpty(t *testing.T) {
	rec := IssueOpsRecord{ID: "1", Repo: "/repo", Phase: IssueOpsPhaseProblem}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	s := string(data)
	for _, key := range []string{"phase_ledger", "domain_review", "ai_slop_clean_categories", "ai_slop_clean_verification"} {
		if strings.Contains(s, key) {
			t.Fatalf("minimal record should omit %q, got %s", key, s)
		}
	}
}

func TestIssueOpsRecordNewFieldsRoundTrip(t *testing.T) {
	rec := IssueOpsRecord{
		ID:    "1",
		Repo:  "/repo",
		Phase: IssueOpsPhaseGrill,
		DomainReview: &IssueOpsDomainReview{
			Terminology:       []string{"ledger"},
			ModelFit:          "fits",
			Risks:             []string{"deadlock"},
			OpenUncertainties: []string{"none"},
			ReviewedAt:        "2026-06-29T00:00:00Z",
		},
		AISlopCleanCategories:   []string{"dead-code"},
		AISlopCleanVerification: []string{"go test ./..."},
		PhaseLedger: IssueOpsPhaseLedger{
			IssueOpsPhaseProblem: IssueOpsPhaseLedgerEntry{Phase: IssueOpsPhaseProblem, CompletedAt: "2026-06-29T00:01:00Z", Artifacts: []string{"intent_contract"}},
		},
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	var got IssueOpsRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if got.DomainReview == nil || got.DomainReview.ModelFit != "fits" {
		t.Fatalf("domain review did not round-trip: %#v", got.DomainReview)
	}
	if len(got.AISlopCleanCategories) != 1 || got.AISlopCleanCategories[0] != "dead-code" {
		t.Fatalf("cleanup categories did not round-trip: %#v", got.AISlopCleanCategories)
	}
	if len(got.AISlopCleanVerification) != 1 || got.AISlopCleanVerification[0] != "go test ./..." {
		t.Fatalf("verification evidence did not round-trip: %#v", got.AISlopCleanVerification)
	}
	if _, ok := got.PhaseLedger[IssueOpsPhaseProblem]; !ok {
		t.Fatalf("phase ledger did not round-trip: %#v", got.PhaseLedger)
	}
}

func TestIssueOpsFeedbackItemResolutionRoundTrip(t *testing.T) {
	item := IssueOpsFeedbackItem{Source: "review", Body: "fix", Classification: "defect", CreatedAt: "2026-06-29T00:00:00Z", Resolution: "valid-defect"}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal feedback: %v", err)
	}
	if !strings.Contains(string(data), "\"resolution\":\"valid-defect\"") {
		t.Fatalf("resolution should serialize, got %s", data)
	}
	empty := IssueOpsFeedbackItem{Source: "review", Body: "fix", Classification: "defect", CreatedAt: "2026-06-29T00:00:00Z"}
	emptyData, _ := json.Marshal(empty)
	if strings.Contains(string(emptyData), "resolution") {
		t.Fatalf("empty resolution should be omitted, got %s", emptyData)
	}
}

func TestIssueOpsRemoteArtifactTargetBranchRoundTrip(t *testing.T) {
	art := IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://x/1", TargetBranch: "main", VerifiedAt: "2026-06-29T00:00:00Z"}
	data, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	if !strings.Contains(string(data), "\"target_branch\":\"main\"") {
		t.Fatalf("target_branch should serialize, got %s", data)
	}
	empty := IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://x/1", VerifiedAt: "2026-06-29T00:00:00Z"}
	emptyData, _ := json.Marshal(empty)
	if strings.Contains(string(emptyData), "target_branch") {
		t.Fatalf("empty target_branch should be omitted, got %s", emptyData)
	}
}
