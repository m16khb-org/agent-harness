package issueops

import (
	"testing"

	"issueops/internal/contract/issueops"
)

func TestStrictPRReadinessFlagsTargetBranchMismatch(t *testing.T) {
	repo := initIssueOpsRepo(t)
	base := issueops.IssueOpsRecord{
		OK:            true,
		Repo:          repo,
		Branch:        "main",
		Phase:         IssueOpsPhasePR,
		IssueURL:      "https://github.com/example/repo/issues/1",
		PlanPath:      "plans/demo.md",
		WorktreePath:  repo,
		Intent:        issueOpsIntentContractForTest(),
		DesignReview:  issueOpsDesignReviewForTest(),
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/1", Branch: "main", BaseBranch: "main", LinkVerified: true},
	}

	mismatch := base
	mismatch.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/1", TargetBranch: "develop", VerifiedAt: "2026-06-29T00:00:00Z"}
	if ready := IssueOpsStrictPRReadiness(mismatch); !containsString(ready.Missing, "target_branch_match") {
		t.Fatalf("strict PR readiness must flag target_branch_match when target != base: %#v", ready.Missing)
	}

	match := base
	match.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/1", TargetBranch: "main", VerifiedAt: "2026-06-29T00:00:00Z"}
	if ready := IssueOpsStrictPRReadiness(match); containsString(ready.Missing, "target_branch_match") {
		t.Fatalf("matching target/base must not flag target_branch_match: %#v", ready.Missing)
	}
}
