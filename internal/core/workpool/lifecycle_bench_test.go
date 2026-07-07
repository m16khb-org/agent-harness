package workpool

import (
	"fmt"
	"testing"
	"time"
)

func TestClaimAt1000TasksCompletesWithinBudget(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool := seedWorkPoolForClaimBenchmark(t, "claim-budget")

	started := time.Now()
	if _, err := Claim(pool.ID, "worker-budget"); err != nil {
		t.Fatal(err)
	}
	budget := 2 * time.Second
	if workPoolRaceDetectorEnabled() {
		budget = 5 * time.Second
	}
	if elapsed := time.Since(started); elapsed > budget {
		t.Fatalf("claim at 1000 tasks took %s, want <=%s", elapsed, budget)
	}
}

func BenchmarkClaimAt1000Tasks(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		b.Setenv("HARNESS_STATE_DIR", dir)
		pool := seedWorkPoolForClaimBenchmark(b, fmt.Sprintf("bench-%d", i))
		b.StartTimer()
		if _, err := Claim(pool.ID, fmt.Sprintf("worker-%d", i)); err != nil {
			b.Fatal(err)
		}
	}
}

func seedWorkPoolForClaimBenchmark(tb testing.TB, name string) WorkPool {
	tb.Helper()
	pool, err := CreatePool(CreatePoolRequest{Repo: tb.TempDir(), Name: name, Size: 16})
	if err != nil {
		tb.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := AddTask(pool.ID, AddTaskRequest{
			Title:        fmt.Sprintf("task %04d", i),
			Instructions: "mechanical work",
		}); err != nil {
			tb.Fatalf("AddTask %d: %v", i, err)
		}
	}
	return pool
}
