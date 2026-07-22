package issueopscli

import (
	"encoding/json"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/workpool"
)

func TestCLIIssueOpsPhaseAdvanceToPRBlockedByOpenPool(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIGitRepoForRemoteVerifyTest(t)
	parent, actor := startIssueOpsCLIReadyPRParentWithChild(t, repo, "123-parent-pr-pool-gate")
	pool, err := workpool.CreatePool(workpool.CreatePoolRequest{
		Repo:          repo,
		Name:          "cli pr gate open pool",
		ParentCycleID: parent.ID,
		Size:          2,
		LeaseTTL:      "1h",
	})
	if err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	task, err := workpool.AddTask(pool.ID, workpool.AddTaskRequest{
		Title:        "pool gate task",
		Instructions: "prove parent pr gate blocks incomplete pool",
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if _, err := core.AdvanceIssueOpsPhaseWithActor(core.IssueOpsStateRoot(), parent.ID, string(core.IssueOpsPhaseAISlopClean), actor); err != nil {
		t.Fatal(err)
	}

	blockedOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"}, actor))
	})
	assertIssueOpsJSONErrorContains(t, blockedOut, err, "pool_incomplete:"+pool.ID)

	if claimed, err := workpool.Claim(pool.ID, "worker-a"); err != nil || claimed.Task.ID != task.ID {
		t.Fatalf("Claim: result=%+v err=%v", claimed, err)
	}
	if _, err := workpool.Submit(pool.ID, task.ID, "worker-a", []string{"go test ./cmd/harness/issueopscli"}, "workpool/gate", repo); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := workpool.Accept(pool.ID, task.ID, []string{"reviewed pool gate evidence"}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if _, err := workpool.Close(pool.ID, false, ""); err != nil {
		t.Fatalf("Close: %v", err)
	}

	prOut := captureStdoutForContract(t, func() error {
		return runIssueOps(withIssueOpsCLIActor([]string{"phase", "--id", parent.ID, "--to", "pr", "--json"}, actor))
	})
	var prRecord core.IssueOpsRecord
	if err := json.Unmarshal([]byte(prOut), &prRecord); err != nil {
		t.Fatalf("phase pr should return JSON after pool close: %v\n%s", err, prOut)
	}
	if prRecord.Phase != core.IssueOpsPhasePR {
		t.Fatalf("closed pool should allow parent pr phase, got %s", prRecord.Phase)
	}
}
