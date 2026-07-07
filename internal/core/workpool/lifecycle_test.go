package workpool

import (
	"strings"
	"testing"
	"time"
)

func TestClaimLeasesOldestPendingAndSaturates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "claim", CreatePoolRequest{Size: 1})
	first := addWorkPoolTaskForLifecycleTest(t, pool.ID, "first")
	second := addWorkPoolTaskForLifecycleTest(t, pool.ID, "second")

	claimed, err := Claim(pool.ID, "worker-a")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Task.ID != first.ID || claimed.Task.Status != "leased" || claimed.Task.WorkerID != "worker-a" {
		t.Fatalf("claim should lease oldest pending to worker-a, got %+v", claimed.Task)
	}
	if claimed.Task.LeaseExpiresAt == "" || !strings.Contains(claimed.Prompt, "pool/"+pool.Name+"/"+first.ID) || !strings.Contains(claimed.Prompt, "heartbeat") || !strings.Contains(claimed.Prompt, "submit") {
		t.Fatalf("claim should return lease and delegation prompt, got %+v", claimed)
	}
	if _, err := Claim(pool.ID, "worker-b"); err == nil || !strings.Contains(err.Error(), "pool_saturated") {
		t.Fatalf("second claim should saturate size=1 pool, got %v", err)
	}
	readSecond, err := ReadTask(pool.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readSecond.Status != "pending" {
		t.Fatalf("saturated claim should leave second pending, got %+v", readSecond)
	}
}

func TestHeartbeatAfterReapRefusesStaleWorker(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "heartbeat-reap", CreatePoolRequest{LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "lease")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry, func() {
		if _, err := Reap(pool.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := Heartbeat(pool.ID, task.ID, "worker-a"); err == nil || !strings.Contains(err.Error(), "lease_not_held") {
			t.Fatalf("stale heartbeat after reap err=%v, want lease_not_held", err)
		}
	})
}

func TestSubmitAfterLeaseExpiryRefused(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "submit-expired", CreatePoolRequest{LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "submit")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry, func() {
		if _, err := Submit(pool.ID, task.ID, "worker-a", []string{"go test"}, "pool/branch", "/tmp/worktree"); err == nil || !strings.Contains(err.Error(), "lease_expired") {
			t.Fatalf("expired submit err=%v, want lease_expired", err)
		}
	})
}

func TestSubmitRefusedAfterTaskReclaimedByAnotherWorker(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "submit-reclaimed", CreatePoolRequest{LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "reclaimed")
	claimedA := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimedA.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry, func() {
		if _, err := Reap(pool.ID); err != nil {
			t.Fatal(err)
		}
	})
	claimedB := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-b")

	if _, err := Submit(pool.ID, task.ID, "worker-a", []string{"old evidence"}, "old", "/old"); err == nil || !strings.Contains(err.Error(), "worker_mismatch") {
		t.Fatalf("old worker submit err=%v, want worker_mismatch", err)
	}
	if _, err := Heartbeat(pool.ID, task.ID, "worker-a"); err == nil || !strings.Contains(err.Error(), "worker_mismatch") {
		t.Fatalf("old worker heartbeat err=%v, want worker_mismatch", err)
	}
	submitted, err := Submit(pool.ID, task.ID, "worker-b", []string{"new evidence"}, claimedB.Task.Branch, "/tmp/worktree")
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != "submitted" || submitted.WorkerID != "worker-b" || len(submitted.Evidence) != 1 {
		t.Fatalf("new worker submit should succeed, got %+v", submitted)
	}
}

func TestReapExactBoundaryIsExpired(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "reap-boundary", CreatePoolRequest{LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "boundary")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry, func() {
		reaped, err := Reap(pool.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(reaped) != 1 || reaped[0].ID != task.ID || reaped[0].Status != "pending" || reaped[0].Attempts != 1 {
			t.Fatalf("exact boundary should reap task, got %+v", reaped)
		}
	})
}

func TestReapOneTickBeforeExpiryNotReaped(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "reap-before", CreatePoolRequest{LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "before")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry.Add(-time.Nanosecond), func() {
		reaped, err := Reap(pool.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(reaped) != 0 {
			t.Fatalf("one tick before expiry should not reap, got %+v", reaped)
		}
		read, err := ReadTask(pool.ID, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if read.Status != "leased" || read.WorkerID != "worker-a" {
			t.Fatalf("task should remain leased, got %+v", read)
		}
	})
}

func TestDrainingRefusesClaimAllowsSubmit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "draining", CreatePoolRequest{})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "draining task")
	claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	pool.Status = "draining"
	if _, err := writePool(pool); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(pool.ID, "worker-b"); err == nil || !strings.Contains(err.Error(), "pool_draining") {
		t.Fatalf("draining claim err=%v, want pool_draining", err)
	}
	if _, err := Submit(pool.ID, task.ID, "worker-a", []string{"evidence"}, "branch", "/tmp/worktree"); err != nil {
		t.Fatalf("draining pool should allow submit: %v", err)
	}
}

func TestClosedPoolRefusesAllMutations(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "closed", CreatePoolRequest{})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "closed task")
	claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	if _, err := Close(pool.ID, true, "closing test pool"); err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func() error{
		"claim": func() error {
			_, err := Claim(pool.ID, "worker-b")
			return err
		},
		"heartbeat": func() error {
			_, err := Heartbeat(pool.ID, task.ID, "worker-a")
			return err
		},
		"submit": func() error {
			_, err := Submit(pool.ID, task.ID, "worker-a", []string{"evidence"}, "branch", "/tmp/worktree")
			return err
		},
		"accept": func() error {
			_, err := Accept(pool.ID, task.ID, []string{"validated"})
			return err
		},
		"reject": func() error {
			_, err := Reject(pool.ID, task.ID, "rejected after close", true)
			return err
		},
	}
	for name, mutate := range mutations {
		if err := mutate(); err == nil || !strings.Contains(err.Error(), "pool_closed") {
			t.Fatalf("%s on closed pool err=%v, want pool_closed", name, err)
		}
	}
}

func TestCloseEmptyPoolSucceedsTrivially(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "empty-close", CreatePoolRequest{})
	closed, err := Close(pool.ID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != "closed" {
		t.Fatalf("empty pool should close, got %+v", closed)
	}
}

func TestAcceptRejectTransitions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "accept-reject", CreatePoolRequest{MaxAttempts: 1})
	acceptedTask := addWorkPoolTaskForLifecycleTest(t, pool.ID, "accepted")
	rejectedTask := addWorkPoolTaskForLifecycleTest(t, pool.ID, "rejected")
	claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	if _, err := Submit(pool.ID, acceptedTask.ID, "worker-a", []string{"worker evidence"}, "branch", "/tmp/worktree"); err != nil {
		t.Fatal(err)
	}
	accepted, err := Accept(pool.ID, acceptedTask.ID, []string{"main validated"})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != "accepted" || accepted.Evidence[0] != "main validated" {
		t.Fatalf("accept should terminally record validation evidence, got %+v", accepted)
	}

	claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-b")
	if _, err := Submit(pool.ID, rejectedTask.ID, "worker-b", []string{"bad evidence"}, "branch", "/tmp/worktree"); err != nil {
		t.Fatal(err)
	}
	dropped, err := Reject(pool.ID, rejectedTask.ID, "bad result needs redo", true)
	if err != nil {
		t.Fatal(err)
	}
	if dropped.Status != "dropped" || dropped.Attempts != 1 || dropped.RejectReason == "" {
		t.Fatalf("reject at max attempts should drop, got %+v", dropped)
	}
	if _, err := Reject(pool.ID, rejectedTask.ID, "short", true); err == nil || !strings.Contains(err.Error(), "reject_reason_too_short") {
		t.Fatalf("short reject reason err=%v, want reject_reason_too_short", err)
	}
}

func createWorkPoolForLifecycleTest(t *testing.T, name string, req CreatePoolRequest) WorkPool {
	t.Helper()
	req.Repo = t.TempDir()
	req.Name = name
	pool, err := CreatePool(req)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func addWorkPoolTaskForLifecycleTest(t *testing.T, poolID, title string) WorkTask {
	t.Helper()
	task, err := AddTask(poolID, AddTaskRequest{Title: title, Instructions: "do work"})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func claimWorkPoolTaskForLifecycleTest(t *testing.T, poolID, workerID string) ClaimResult {
	t.Helper()
	claimed, err := Claim(poolID, workerID)
	if err != nil {
		t.Fatal(err)
	}
	return claimed
}

func mustParseWorkPoolTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func withWorkPoolClockForTest(t *testing.T, now time.Time, fn func()) {
	t.Helper()
	old := workPoolNow
	workPoolNow = func() time.Time { return now }
	defer func() { workPoolNow = old }()
	fn()
}
