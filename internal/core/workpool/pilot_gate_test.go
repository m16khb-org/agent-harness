package workpool

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestPilotGateLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "pilot-lifecycle", CreatePoolRequest{PilotRequired: true, Size: 2})
	if !pool.PilotRequired {
		t.Fatalf("CreatePool should persist PilotRequired, got %+v", pool)
	}

	if _, err := Claim(pool.ID, "worker-unassigned"); err == nil || !strings.Contains(err.Error(), "pool_pilot_unassigned") {
		t.Fatalf("claim before pilot assignment err=%v, want pool_pilot_unassigned", err)
	}

	mass := addWorkPoolTaskForLifecycleTest(t, pool.ID, "mass task")
	pilot, err := AddTask(pool.ID, AddTaskRequest{Title: "pilot task", Instructions: "prove approach", Pilot: true})
	if err != nil {
		t.Fatal(err)
	}
	readPool, err := ReadPool(pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readPool.PilotTaskID != pilot.ID {
		t.Fatalf("pilot task id not persisted: got %q want %q", readPool.PilotTaskID, pilot.ID)
	}
	if _, err := AddTask(pool.ID, AddTaskRequest{Title: "second pilot", Instructions: "duplicate", Pilot: true}); err == nil || !strings.Contains(err.Error(), "pool_pilot_already_set") {
		t.Fatalf("duplicate pilot err=%v, want pool_pilot_already_set", err)
	}

	claimedPilot, err := Claim(pool.ID, "worker-pilot")
	if err != nil {
		t.Fatal(err)
	}
	if claimedPilot.Task.ID != pilot.ID {
		t.Fatalf("pilot should be claimed before mass task: got %s want %s", claimedPilot.Task.ID, pilot.ID)
	}
	if _, err := Claim(pool.ID, "worker-mass-blocked"); err == nil || !strings.Contains(err.Error(), "pool_pilot_pending") {
		t.Fatalf("mass claim before pilot accept err=%v, want pool_pilot_pending", err)
	}

	if _, err := Submit(pool.ID, pilot.ID, "worker-pilot", []string{"pilot evidence"}, claimedPilot.Task.Branch, "/tmp/pilot"); err != nil {
		t.Fatal(err)
	}
	if _, err := Accept(pool.ID, pilot.ID, []string{"main accepted pilot"}); err != nil {
		t.Fatal(err)
	}
	claimedMass, err := Claim(pool.ID, "worker-mass")
	if err != nil {
		t.Fatal(err)
	}
	if claimedMass.Task.ID != mass.ID {
		t.Fatalf("accepted pilot should open mass claim, got %s want %s", claimedMass.Task.ID, mass.ID)
	}
}

func TestPilotDroppedFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "pilot-dropped", CreatePoolRequest{PilotRequired: true, MaxAttempts: 1})
	pilot, err := AddTask(pool.ID, AddTaskRequest{Title: "pilot task", Instructions: "try once", Pilot: true})
	if err != nil {
		t.Fatal(err)
	}
	addWorkPoolTaskForLifecycleTest(t, pool.ID, "mass task")
	claimed := claimWorkPoolTaskForLifecycleTest(t, pool.ID, "worker-pilot")
	if claimed.Task.ID != pilot.ID {
		t.Fatalf("expected pilot claim, got %+v", claimed.Task)
	}
	if _, err := Reject(pool.ID, pilot.ID, "pilot approach failed", true); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(pool.ID, "worker-mass"); err == nil || !strings.Contains(err.Error(), "pool_pilot_dropped") {
		t.Fatalf("claim after dropped pilot err=%v, want pool_pilot_dropped", err)
	}
}

func TestPilotFlagRequiresPilotPool(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "pilot-not-required", CreatePoolRequest{})
	if _, err := AddTask(pool.ID, AddTaskRequest{Title: "pilot task", Instructions: "not required", Pilot: true}); err == nil || !strings.Contains(err.Error(), "pool_pilot_not_required") {
		t.Fatalf("pilot task on non-pilot pool err=%v, want pool_pilot_not_required", err)
	}
	if _, err := AddTask(pool.ID, AddTaskRequest{Title: "normal task", Instructions: "plain"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(pool.ID, "worker"); err != nil {
		t.Fatalf("non-pilot pool should keep existing claim behavior: %v", err)
	}
}

func TestConcurrentClaimCannotBypassPilotGate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := createWorkPoolForLifecycleTest(t, "pilot-race", CreatePoolRequest{PilotRequired: true, Size: 8})
	pilot, err := AddTask(pool.ID, AddTaskRequest{Title: "pilot task", Instructions: "first", Pilot: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		addWorkPoolTaskForLifecycleTest(t, pool.ID, fmt.Sprintf("mass task %02d", i))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := []WorkTask{}
	errs := []error{}
	for i := 0; i < 20; i++ {
		workerID := fmt.Sprintf("worker-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Claim(pool.ID, workerID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			claimed = append(claimed, result.Task)
		}()
	}
	wg.Wait()
	if len(claimed) != 1 || claimed[0].ID != pilot.ID {
		t.Fatalf("only pilot should be claimable before acceptance, claimed=%+v", claimed)
	}
	for _, err := range errs {
		if !strings.Contains(err.Error(), "pool_pilot_pending") && !strings.Contains(err.Error(), "pool_saturated") {
			t.Fatalf("unexpected concurrent claim err=%v", err)
		}
	}
}
