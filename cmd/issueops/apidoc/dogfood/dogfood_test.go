package dogfood

import (
	"path/filepath"
	"strings"
	"testing"

	apidoc "issueops/cmd/issueops/apidoc"
)

// fixtureAPIFiles are the API candidate files (controller/DTO) the gates see.
// Service files are intentionally absent: discovering them is the review
// evidence contract's job.
func fixtureAPIFiles() []string {
	return []string{
		"apps/api-gateway/src/users/users.controller.ts",
		"apps/api-gateway/src/users/dto/create-user.dto.ts",
		"apps/api-gateway/src/users/dto/update-user.dto.ts",
		"apps/api-gateway/src/users/dto/search-users.dto.ts",
		"apps/api-gateway/src/orders/orders.controller.ts",
		"apps/api-gateway/src/orders/dto/create-order.dto.ts",
		"apps/orders-service/src/orders.controller.ts",
	}
}

// TestDogfoodStaticRecall is the repeatable dogfooding standard: every seeded
// decorator-level omission must be flagged by the static gate.
func TestDogfoodStaticRecall(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir, DirtyFiles()); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	result, _ := apidoc.RunStaticCheckWithOptions(apidoc.StaticOptions{Repo: dir, Files: fixtureAPIFiles(), All: true, JSON: true})

	found := map[string]bool{}
	for _, violation := range result.Violations {
		found[violation.Code+"@"+filepath.Base(violation.File)] = true
	}
	var missed []string
	for _, expected := range GroundTruth() {
		if expected.Layer != "static" {
			continue
		}
		if !found[expected.Code+"@"+expected.File] {
			missed = append(missed, expected.ID+" ("+expected.Code+"@"+expected.File+")")
		}
	}
	if len(missed) > 0 {
		t.Errorf("static gate missed seeded omissions: %s\nall violations: %s", strings.Join(missed, ", "), codes(result))
	}
}

// TestDogfoodCleanFixtureNoFalsePositives locks precision: the fully documented
// control fixture must produce zero static violations.
func TestDogfoodCleanFixtureNoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir, CleanFiles()); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	result, _ := apidoc.RunStaticCheckWithOptions(apidoc.StaticOptions{Repo: dir, Files: fixtureAPIFiles(), All: true, JSON: true})
	if !result.OK || len(result.Violations) != 0 {
		t.Fatalf("clean fixture produced violations: %s", codes(result))
	}
}

// TestDogfoodReviewEvidenceContract locks the review anti-miss standard: the
// rendered review prompt must carry the business-logic error contract evidence
// (service throw sites, microservice RpcException patterns, ClientProxy hops)
// for every seeded error-contract finding. Without this evidence a host agent
// reviewing only the diff cannot see the omissions.
func TestDogfoodReviewEvidenceContract(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir, DirtyFiles()); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	result, err := apidoc.RunReviewWithOptions(apidoc.ReviewOptions{Repo: dir, Files: fixtureAPIFiles(), All: true, JSON: true})
	if err == nil {
		t.Fatalf("expected pending host-agent review result, got err=nil")
	}
	prompt := result.Prompt
	if strings.TrimSpace(prompt) == "" {
		t.Fatal("review prompt is empty")
	}
	var missed []string
	for _, expected := range GroundTruth() {
		if expected.Layer != "review" {
			continue
		}
		allPresent := true
		for _, detail := range expected.Details {
			if !strings.Contains(prompt, detail) {
				allPresent = false
				break
			}
		}
		if !allPresent {
			missed = append(missed, expected.ID+" ("+strings.Join(expected.Details, ", ")+")")
		}
	}
	if len(missed) > 0 {
		t.Errorf("review input is missing error-contract evidence: %s", strings.Join(missed, ", "))
	}
	if !strings.Contains(prompt, "Evidence") {
		t.Error("review prompt has no evidence section")
	}
}

func codes(result apidoc.StaticResult) string {
	var parts []string
	for _, violation := range result.Violations {
		parts = append(parts, violation.Code+"@"+filepath.Base(violation.File)+":"+itoa(violation.Line))
	}
	return strings.Join(parts, ", ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
