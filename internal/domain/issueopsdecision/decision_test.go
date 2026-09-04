package issueopsdecision

import (
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestBuildRejectsSecretAndOversizedFields(t *testing.T) {
	now := time.Date(2026, 8, 11, 6, 0, 0, 0, time.UTC)
	base := issueopscontract.IssueOpsDecisionRecordRequest{
		Kind:  "architecture",
		Title: "Boundary",
		Body:  "Move the decision capability.",
	}
	secret := base
	secret.Body = "token=secret-value"
	if _, err := Build(secret, now); err == nil {
		t.Fatal("secret-like decision body must be rejected")
	}
	oversized := base
	oversized.Title = strings.Repeat("a", 513)
	if _, err := Build(oversized, now); err == nil {
		t.Fatal("oversized title must be rejected")
	}
}

func TestBuildValidatesAndProjectsAllFields(t *testing.T) {
	now := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	req := issueopscontract.IssueOpsDecisionRecordRequest{
		Kind:               " implementation ",
		Title:              "  Use sqlc  ",
		Body:               "Generate typed queries.",
		Rationale:          " type safety ",
		Alternatives:       []string{"gorm", "raw sql"},
		AffectedIssueLinks: []string{"https://example.com/i/1"},
		AffectedArtifacts:  []string{"plan", "review"},
	}
	decision, err := Build(req, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Kind != "implementation" || decision.Title != "Use sqlc" || decision.Body != "Generate typed queries." ||
		decision.Rationale != "type safety" || decision.CreatedAt != "2026-08-21T09:00:00Z" {
		t.Fatalf("projection wrong: %#v", decision)
	}
	if len(decision.Alternatives) != 2 || len(decision.AffectedArtifacts) != 2 || len(decision.AffectedIssueLinks) != 1 {
		t.Fatalf("stable slices wrong: %#v", decision)
	}
	// nil 슬라이스는 빈 슬라이스로 정규화된다(JSON에서 null 방지).
	minimal, err := Build(issueopscontract.IssueOpsDecisionRecordRequest{Kind: "scope", Title: "t", Body: "b"}, now)
	if err != nil || minimal.Alternatives == nil || minimal.AffectedArtifacts == nil || minimal.AffectedIssueLinks == nil {
		t.Fatalf("nil slices must normalize to empty: %#v err=%v", minimal, err)
	}
}

func TestBuildRejectsInvalidKindsArtifactsAndBodies(t *testing.T) {
	now := time.Now()
	base := issueopscontract.IssueOpsDecisionRecordRequest{Kind: "product", Title: "t", Body: "b"}
	cases := []struct {
		name string
		mut  func(*issueopscontract.IssueOpsDecisionRecordRequest)
		want string
	}{
		{"unknown kind", func(r *issueopscontract.IssueOpsDecisionRecordRequest) { r.Kind = "urgent" }, "invalid decision kind"},
		{"empty title", func(r *issueopscontract.IssueOpsDecisionRecordRequest) { r.Title = "  " }, "title is required"},
		{"oversized body", func(r *issueopscontract.IssueOpsDecisionRecordRequest) { r.Body = strings.Repeat("x", 65537) }, "64 KiB"},
		{"invalid artifact", func(r *issueopscontract.IssueOpsDecisionRecordRequest) { r.AffectedArtifacts = []string{"diagram"} }, "invalid affected artifact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.mut(&req)
			if _, err := Build(req, now); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error ~ %q, got %v", tc.want, err)
			}
		})
	}
}
