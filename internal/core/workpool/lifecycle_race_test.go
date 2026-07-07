package workpool

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClaimConcurrentRespectsPoolSize(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "claim-race", CreatePoolRequest{Size: 3})
	for i := 0; i < 10; i++ {
		addWorkPoolTaskForLifecycleTest(t, pool.ID, fmt.Sprintf("task %02d", i))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedByWorker := map[string]WorkTask{}
	saturated := 0
	errs := []error{}
	for i := 0; i < 10; i++ {
		workerID := fmt.Sprintf("worker-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Claim(pool.ID, workerID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if strings.Contains(err.Error(), "pool_saturated") {
					saturated++
					return
				}
				errs = append(errs, err)
				return
			}
			claimedByWorker[workerID] = result.Task
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		t.Fatalf("unexpected claim errors: %v", errs)
	}
	if len(claimedByWorker) != 3 || saturated != 7 {
		t.Fatalf("claims=%d saturated=%d, want 3/7; claims=%#v", len(claimedByWorker), saturated, claimedByWorker)
	}
	seenTasks := map[string]string{}
	for workerID, task := range claimedByWorker {
		if task.Status != "leased" || task.WorkerID != workerID || task.LeaseExpiresAt == "" {
			t.Fatalf("claimed task should match claimant and carry lease: worker=%s task=%+v", workerID, task)
		}
		if previous, exists := seenTasks[task.ID]; exists {
			t.Fatalf("task %s leased twice by %s and %s", task.ID, previous, workerID)
		}
		seenTasks[task.ID] = workerID
	}
}

func TestClaimVsReapNoDoubleLease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "claim-reap-race", CreatePoolRequest{Size: 1, LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "racy")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)

	withWorkPoolClockForTest(t, expiry, func() {
		var wg sync.WaitGroup
		wg.Add(2)
		var reapErr error
		var claimResult ClaimResult
		var claimErr error
		go func() {
			defer wg.Done()
			_, reapErr = Reap(pool.ID)
		}()
		go func() {
			defer wg.Done()
			claimResult, claimErr = Claim(pool.ID, "worker-b")
		}()
		wg.Wait()
		if reapErr != nil {
			t.Fatalf("reap error: %v", reapErr)
		}
		if claimErr != nil && !strings.Contains(claimErr.Error(), "pool_saturated") && !strings.Contains(claimErr.Error(), "pool_no_pending_tasks") {
			t.Fatalf("unexpected claim error: %v", claimErr)
		}
		read, err := ReadTask(pool.ID, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if read.Status == "leased" {
			if read.WorkerID != "worker-a" && read.WorkerID != "worker-b" {
				t.Fatalf("leased task has unknown worker: %+v", read)
			}
			if claimErr == nil && claimResult.Task.ID != read.ID {
				t.Fatalf("claim result/task read mismatch: claim=%+v read=%+v", claimResult.Task, read)
			}
		}
		if read.Status == "pending" && read.WorkerID != "" {
			t.Fatalf("pending task must not retain worker lease: %+v", read)
		}
		if _, err := Submit(pool.ID, task.ID, "worker-a", []string{"stale"}, "branch-a", "/tmp/a"); err == nil || !strings.Contains(err.Error(), "worker_mismatch") && !strings.Contains(err.Error(), "lease_not_held") && !strings.Contains(err.Error(), "lease_expired") {
			t.Fatalf("stale worker submit err=%v, want fencing refusal", err)
		}
	})
}

func TestConcurrentSubmitAcceptNoLostUpdate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "submit-accept-race", CreatePoolRequest{Size: 16})
	claimed := map[string]WorkTask{}
	for i := 0; i < 50; i++ {
		workerID := fmt.Sprintf("worker-%02d", i)
		task := addWorkPoolTaskForLifecycleTest(t, pool.ID, fmt.Sprintf("task %02d", i))
		task.Status = "leased"
		task.WorkerID = workerID
		task.LeaseExpiresAt = workPoolNow().Add(time.Hour).UTC().Format(time.RFC3339Nano)
		task.LastHeartbeatAt = timestampNow()
		task.Branch = recommendedTaskBranch(pool, task)
		var err error
		task, err = writeTask(task)
		if err != nil {
			t.Fatal(err)
		}
		claimed[workerID] = task
	}

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for workerID, task := range claimed {
		workerID := workerID
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			evidence := []string{"submitted by " + workerID}
			submitted, err := Submit(pool.ID, task.ID, workerID, evidence, task.Branch, "/tmp/"+workerID)
			if err != nil {
				errs <- err
				return
			}
			if submitted.WorkerID != workerID || submitted.Evidence[0] != evidence[0] || submitted.Status != "submitted" {
				errs <- fmt.Errorf("submit integrity mismatch for %s: %+v", workerID, submitted)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	errs = make(chan error, 100)
	for workerID, task := range claimed {
		workerID := workerID
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			evidence := []string{"accepted " + workerID}
			accepted, err := Accept(pool.ID, task.ID, evidence)
			if err != nil {
				errs <- err
				return
			}
			if accepted.Status != "accepted" || accepted.WorkerID != workerID || accepted.Evidence[0] != evidence[0] {
				errs <- fmt.Errorf("accept integrity mismatch for %s: %+v", workerID, accepted)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := ListTasks(pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 50 {
		t.Fatalf("task count=%d, want 50", len(tasks))
	}
	for _, task := range tasks {
		if task.Status != "accepted" || !strings.HasPrefix(task.WorkerID, "worker-") || !strings.HasPrefix(task.Evidence[0], "accepted worker-") {
			t.Fatalf("final task integrity mismatch: %+v", task)
		}
	}
}

func TestClaimVsReapStaleWorkerFencedAfterReclaim(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "claim-reap-reclaim", CreatePoolRequest{Size: 1, LeaseTTL: "1m"})
	task := addWorkPoolTaskForLifecycleTest(t, pool.ID, "reclaim")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-a")
	expiry := mustParseWorkPoolTime(t, claimed.Task.LeaseExpiresAt)
	withWorkPoolClockForTest(t, expiry, func() {
		if _, err := Reap(pool.ID); err != nil {
			t.Fatal(err)
		}
	})
	claimedB := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-b")
	if claimedB.Task.ID != task.ID || claimedB.Task.WorkerID != "worker-b" {
		t.Fatalf("worker-b should reclaim task, got %+v", claimedB.Task)
	}
	if _, err := Heartbeat(pool.ID, task.ID, "worker-a"); err == nil || !strings.Contains(err.Error(), "worker_mismatch") {
		t.Fatalf("stale heartbeat err=%v, want worker_mismatch", err)
	}
	if _, err := Submit(pool.ID, task.ID, "worker-a", []string{"stale"}, "branch", "/tmp/a"); err == nil || !strings.Contains(err.Error(), "worker_mismatch") {
		t.Fatalf("stale submit err=%v, want worker_mismatch", err)
	}
	if _, err := Submit(pool.ID, task.ID, "worker-b", []string{"fresh"}, claimedB.Task.Branch, "/tmp/b"); err != nil {
		t.Fatalf("fresh worker submit: %v", err)
	}
}

func TestWorkPoolClockRaceTestsUseRealTime(t *testing.T) {
	// Guard against earlier boundary-test clock injection leaking into race tests.
	if delta := time.Since(workPoolNow()); delta < -time.Second || delta > time.Second {
		t.Fatalf("workPoolNow appears stale: delta=%s", delta)
	}
}
