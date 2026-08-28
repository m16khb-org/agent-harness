package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestBranchRetargetCLIForwardsRequestAndPrintsRecord(t *testing.T) {
	prev := issueOpsCLIDeps.RetargetIssueOpsBranchWithActor
	t.Cleanup(func() { issueOpsCLIDeps.RetargetIssueOpsBranchWithActor = prev })
	var got issueopscontract.IssueOpsBranchRetargetRequest
	var gotID string
	issueOpsCLIDeps.RetargetIssueOpsBranchWithActor = func(_ string, id string, req issueopscontract.IssueOpsBranchRetargetRequest, _ issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
		gotID, got = id, req
		return issueopscontract.IssueOpsRecord{OK: true, ID: id, BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			BaseBranch: req.BaseBranch,
			Retargets:  []issueopscontract.IssueOpsBranchRetarget{{FromBase: "release/stg", ToBase: req.BaseBranch, Reason: req.Reason}},
		}}, nil
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "retarget", "--id", "io-1", "--base-branch", "2803-umbrella", "--reason", "child MR retargeted to the umbrella", "--json"})
	})
	if gotID != "io-1" || got.BaseBranch != "2803-umbrella" || !strings.Contains(got.Reason, "umbrella") {
		t.Fatalf("retarget request must reach the core unchanged: id=%q req=%+v", gotID, got)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("branch retarget should return JSON: %v\n%s", err, out)
	}
	prepare, _ := record["branch_prepare"].(map[string]any)
	retargets, _ := prepare["retargets"].([]any)
	if prepare["base_branch"] != "2803-umbrella" || len(retargets) != 1 {
		t.Fatalf("printed record must show the retargeted base and history: %#v", record)
	}
}

func TestBranchUsageListsRetarget(t *testing.T) {
	if !strings.Contains(issueOpsBranchPrepareUsage, "branch retarget --id ID --base-branch REF --reason TEXT") {
		t.Fatalf("branch usage must document retarget: %s", issueOpsBranchPrepareUsage)
	}
}
