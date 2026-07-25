package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestUmbrellaBranchGateReasonRequiresPreparedBranch(t *testing.T) {
	record := IssueOpsRecord{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
	}
	reason := UmbrellaBranchGateReason(record)
	if reason == "" {
		t.Fatal("umbrella cycle without branch_prepare must not be allowed to create child work items")
	}
	if !strings.Contains(reason, "branch prepare") {
		t.Fatalf("reason %q must name the command that resolves it", reason)
	}
}

func TestUmbrellaBranchGateReasonRequiresBranchIdentity(t *testing.T) {
	record := IssueOpsRecord{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider:   "github",
			IssueURL:   "https://github.com/acme/repo/issues/78",
			Branch:     "79-something-else",
			BaseBranch: "main",
		},
	}
	if reason := UmbrellaBranchGateReason(record); reason == "" {
		t.Fatal("branch_prepare recorded for a different branch must not satisfy the umbrella gate")
	}
}

func TestUmbrellaBranchGateReasonAcceptsPreparedUmbrella(t *testing.T) {
	record := IssueOpsRecord{
		ID:       "io-parent",
		Repo:     "/repo",
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/acme/repo/issues/78",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider:   "github",
			IssueURL:   "https://github.com/acme/repo/issues/78",
			Branch:     "78-umbrella",
			BaseBranch: "main",
		},
	}
	if reason := UmbrellaBranchGateReason(record); reason != "" {
		t.Fatalf("prepared umbrella branch must pass the gate, got %q", reason)
	}
}
