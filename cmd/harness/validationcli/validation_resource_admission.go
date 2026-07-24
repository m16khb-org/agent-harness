package validationcli

import (
	"context"
	"time"

	"agent-harness/internal/core/resourcewait"
)

func ValidateResourceAdmission() StepResult {
	started := time.Now()
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	sample := resourcewait.Sample{
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
	result, err := resourcewait.Wait(context.Background(), resourcewait.Request{
		WorkspaceRoot: "/self-verify",
		Timeout:       3 * time.Second,
		Interval:      time.Second,
	}, resourcewait.Dependencies{
		Sample: func(context.Context, string) (resourcewait.Sample, error) { return sample, nil },
		Now:    func() time.Time { return now },
		Sleep: func(_ context.Context, duration time.Duration) error {
			now = now.Add(duration)
			return nil
		},
	})
	errs := []string{}
	if err != nil {
		errs = append(errs, err.Error())
	}
	if result.Status != resourcewait.StatusReady || result.SampleCount != 4 {
		errs = append(errs, "resource wait did not admit three stable intervals")
	}
	return assertionStep("resource admission contract", started, errs)
}
