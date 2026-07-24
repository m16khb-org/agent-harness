package resourcewait

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEvaluateOrdersEveryResourceBlocker(t *testing.T) {
	thresholds := E2EProfile().Thresholds
	sample := Sample{
		LogicalCPUCount:             1,
		Load1M:                      thresholds.MaxLoad1MPerCPU + 0.01,
		TotalMemoryBytes:            20 << 30,
		AvailableMemoryBytes:        1,
		SwapIOBytesPerSec:           thresholds.MaxSwapIOBytesPerSec + 1,
		WorkspaceDiskTotalBytes:     100 << 30,
		WorkspaceDiskAvailableBytes: 1,
		TempDiskTotalBytes:          100 << 30,
		TempDiskAvailableBytes:      1,
		PipeCapacityBytes:           thresholds.MinPipeCapacityBytes - 1,
	}

	blockers := Evaluate(sample, thresholds)
	want := []string{
		"load_high",
		"memory_low",
		"swap_io_active",
		"workspace_disk_low",
		"temp_disk_low",
		"pipe_capacity_degraded",
	}
	if len(blockers) != len(want) {
		t.Fatalf("blocker count = %d, want %d: %#v", len(blockers), len(want), blockers)
	}
	for i, code := range want {
		if blockers[i].Code != code {
			t.Errorf("blockers[%d].Code = %q, want %q", i, blockers[i].Code, code)
		}
	}
}

func TestWaitRequiresBaselineAndThreeStableIntervals(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	stable := healthySample()
	result, err := Wait(context.Background(), Request{
		WorkspaceRoot: "/workspace",
		Profile:       "e2e",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, Dependencies{
		Sample: func(context.Context, string) (Sample, error) { return stable, nil },
		Now:    func() time.Time { return now },
		Sleep: func(_ context.Context, delay time.Duration) error {
			now = now.Add(delay)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Status != StatusReady {
		t.Fatalf("status = %q, want %q", result.Status, StatusReady)
	}
	if result.SampleCount != 4 {
		t.Errorf("sample_count = %d, want 4", result.SampleCount)
	}
	if result.ConsecutiveStableSamples != 3 {
		t.Errorf("consecutive_stable_samples = %d, want 3", result.ConsecutiveStableSamples)
	}
}

func TestWaitResetsStabilityAndTimesOutAtScheduledSample(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	samples := []Sample{
		healthySample(),
		healthySample(),
		unhealthyLoadSample(),
		healthySample(),
	}
	progress := make([]int, 0, len(samples))
	result, err := Wait(context.Background(), Request{
		WorkspaceRoot: "/workspace",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, Dependencies{
		Sample: func(context.Context, string) (Sample, error) {
			sample := samples[0]
			samples = samples[1:]
			return sample, nil
		},
		Now: func() time.Time { return now },
		Sleep: func(_ context.Context, delay time.Duration) error {
			now = now.Add(delay)
			return nil
		},
		OnSample: func(_ Sample, _ int, stable int, _ []Blocker) {
			progress = append(progress, stable)
		},
	})
	if err == nil {
		t.Fatal("Wait() error = nil, want timeout")
	}
	if result.Status != StatusTimedOut {
		t.Fatalf("status = %q, want %q", result.Status, StatusTimedOut)
	}
	if result.SampleCount != 4 {
		t.Fatalf("sample_count = %d, want final deadline sample", result.SampleCount)
	}
	wantProgress := []int{0, 1, 0, 1}
	for index, want := range wantProgress {
		if progress[index] != want {
			t.Fatalf("progress[%d] = %d, want %d", index, progress[index], want)
		}
	}
}

func TestWaitReturnsCollectionError(t *testing.T) {
	want := errors.New("collector unavailable")
	result, err := Wait(context.Background(), Request{
		WorkspaceRoot: "/workspace",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, Dependencies{Sample: func(context.Context, string) (Sample, error) { return Sample{}, want }})
	if !errors.Is(err, want) {
		t.Fatalf("Wait() error = %v, want %v", err, want)
	}
	if result.Status != StatusError {
		t.Fatalf("status = %q, want %q", result.Status, StatusError)
	}
}

func TestWaitCancelsBeforeSamplerCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	result, err := Wait(ctx, Request{
		WorkspaceRoot: "/workspace",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, Dependencies{Sample: func(context.Context, string) (Sample, error) {
		cancel()
		return healthySample(), nil
	}})
	if err == nil || result.Status != StatusCancelled {
		t.Fatalf("Wait() = %#v, %v; want cancelled", result, err)
	}
	if result.SampleCount != 0 {
		t.Fatalf("sample_count = %d, want no committed sample", result.SampleCount)
	}
}

func TestWaitStopsAfterScheduledSampleFinishesPastDeadline(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	samples := 0
	sleeps := 0
	result, err := Wait(context.Background(), Request{
		WorkspaceRoot: "/workspace",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, Dependencies{
		Sample: func(context.Context, string) (Sample, error) {
			samples++
			if samples == 2 {
				now = now.Add(3 * time.Second)
			}
			return unhealthyLoadSample(), nil
		},
		Now: func() time.Time { return now },
		Sleep: func(_ context.Context, delay time.Duration) error {
			sleeps++
			now = now.Add(delay)
			return nil
		},
	})
	if err == nil || result.Status != StatusTimedOut {
		t.Fatalf("Wait() = %#v, %v; want timed_out", result, err)
	}
	if sleeps != 1 {
		t.Fatalf("sleep calls = %d, want no sleep after the overdue sample", sleeps)
	}
	if result.FinishedAt != now {
		t.Fatalf("finished_at = %s, want sample completion time %s", result.FinishedAt, now)
	}
}

func healthySample() Sample {
	return Sample{
		LogicalCPUCount:             8,
		Load1M:                      1,
		TotalMemoryBytes:            32 << 30,
		AvailableMemoryBytes:        16 << 30,
		WorkspaceDiskTotalBytes:     512 << 30,
		WorkspaceDiskAvailableBytes: 128 << 30,
		TempDiskTotalBytes:          512 << 30,
		TempDiskAvailableBytes:      128 << 30,
		PipeCapacityBytes:           16 << 10,
	}
}

func unhealthyLoadSample() Sample {
	sample := healthySample()
	sample.Load1M = 100
	return sample
}
