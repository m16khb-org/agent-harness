package harnessapp

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestHarnessAppClaimWiring(t *testing.T) {
	_, err := issueOpsClaimHandler(context.Background(), t.TempDir(), issueops.ExecutionClaimRequest{ID: "io-claim-wiring"}, issueops.ExecutionClaimDependencies{})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-claim-wiring not found") {
		t.Fatalf("claim wiring error=%v", err)
	}
}

func TestIssueOpsClaimProviderNameUsesBranchPrepareAuthority(t *testing.T) {
	got, err := issueOpsClaimProviderName(issueops.IssueOpsRecord{
		IssueURL:      "https://code.company.example/group/agent-harness/-/issues/197",
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "gitlab"},
	}, "https://code.company.example/group/agent-harness/-/issues/197")
	if err != nil || got != "gitlab" {
		t.Fatalf("provider=%q err=%v", got, err)
	}
}
