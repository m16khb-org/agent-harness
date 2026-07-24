package resourcewait

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"agent-harness/internal/port"
)

const (
	ProfileE2E            = "e2e"
	requiredStableSamples = 3
	maxRecentSamples      = 5
)

type Status string

const (
	StatusReady     Status = "ready"
	StatusTimedOut  Status = "timed_out"
	StatusCancelled Status = "cancelled"
	StatusError     Status = "error"
)

type Thresholds struct {
	MaxLoad1MPerCPU                float64 `json:"max_load_1m_per_cpu"`
	MinAvailableMemoryBytes        uint64  `json:"min_available_memory_bytes"`
	MinAvailableMemoryRatio        float64 `json:"min_available_memory_ratio"`
	MaxSwapIOBytesPerSec           uint64  `json:"max_swap_io_bytes_per_sec"`
	MinWorkspaceDiskAvailableBytes uint64  `json:"min_workspace_disk_available_bytes"`
	MinWorkspaceDiskAvailableRatio float64 `json:"min_workspace_disk_available_ratio"`
	MinTempDiskAvailableBytes      uint64  `json:"min_temp_disk_available_bytes"`
	MinTempDiskAvailableRatio      float64 `json:"min_temp_disk_available_ratio"`
	MinPipeCapacityBytes           int     `json:"min_pipe_capacity_bytes"`
}

type Profile struct {
	Name       string     `json:"name"`
	Thresholds Thresholds `json:"thresholds"`
}

func E2EProfile() Profile {
	return Profile{Name: ProfileE2E, Thresholds: Thresholds{
		MaxLoad1MPerCPU:                0.75,
		MinAvailableMemoryBytes:        4 << 30,
		MinAvailableMemoryRatio:        0.20,
		MaxSwapIOBytesPerSec:           1 << 20,
		MinWorkspaceDiskAvailableBytes: 10 << 30,
		MinWorkspaceDiskAvailableRatio: 0.10,
		MinTempDiskAvailableBytes:      10 << 30,
		MinTempDiskAvailableRatio:      0.10,
		MinPipeCapacityBytes:           8192,
	}}
}

type Sample = port.ResourceSample

type Blocker struct {
	Code       string  `json:"code"`
	Metric     string  `json:"metric"`
	Observed   float64 `json:"observed"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Unit       string  `json:"unit"`
	Summary    string  `json:"summary"`
}

func ResolveThresholds(sample Sample, thresholds Thresholds) Thresholds {
	thresholds.MinAvailableMemoryBytes = maxUint64(thresholds.MinAvailableMemoryBytes, ratioFloor(sample.TotalMemoryBytes, thresholds.MinAvailableMemoryRatio))
	thresholds.MinWorkspaceDiskAvailableBytes = maxUint64(thresholds.MinWorkspaceDiskAvailableBytes, ratioFloor(sample.WorkspaceDiskTotalBytes, thresholds.MinWorkspaceDiskAvailableRatio))
	thresholds.MinTempDiskAvailableBytes = maxUint64(thresholds.MinTempDiskAvailableBytes, ratioFloor(sample.TempDiskTotalBytes, thresholds.MinTempDiskAvailableRatio))
	return thresholds
}

func Evaluate(sample Sample, thresholds Thresholds) []Blocker {
	sample = normalizedSample(sample)
	thresholds = ResolveThresholds(sample, thresholds)
	blockers := make([]Blocker, 0, 6)
	if sample.Load1MPerCPU > thresholds.MaxLoad1MPerCPU {
		blockers = append(blockers, Blocker{"load_high", "load_1m_per_cpu", sample.Load1MPerCPU, "<=", thresholds.MaxLoad1MPerCPU, "ratio", "normalized one-minute load exceeds the e2e profile"})
	}
	if sample.AvailableMemoryBytes < thresholds.MinAvailableMemoryBytes {
		blockers = append(blockers, Blocker{"memory_low", "available_memory_bytes", float64(sample.AvailableMemoryBytes), ">=", float64(thresholds.MinAvailableMemoryBytes), "bytes", "available memory is below the e2e profile"})
	}
	if sample.SwapIOBytesPerSec > thresholds.MaxSwapIOBytesPerSec {
		blockers = append(blockers, Blocker{"swap_io_active", "swap_io_bytes_per_sec", float64(sample.SwapIOBytesPerSec), "<=", float64(thresholds.MaxSwapIOBytesPerSec), "bytes_per_sec", "active swap I/O exceeds the e2e profile"})
	}
	if sample.WorkspaceDiskAvailableBytes < thresholds.MinWorkspaceDiskAvailableBytes {
		blockers = append(blockers, Blocker{"workspace_disk_low", "workspace_disk_available_bytes", float64(sample.WorkspaceDiskAvailableBytes), ">=", float64(thresholds.MinWorkspaceDiskAvailableBytes), "bytes", "workspace disk availability is below the e2e profile"})
	}
	if sample.TempDiskAvailableBytes < thresholds.MinTempDiskAvailableBytes {
		blockers = append(blockers, Blocker{"temp_disk_low", "temp_disk_available_bytes", float64(sample.TempDiskAvailableBytes), ">=", float64(thresholds.MinTempDiskAvailableBytes), "bytes", "temporary disk availability is below the e2e profile"})
	}
	if sample.PipeCapacityBytes < thresholds.MinPipeCapacityBytes {
		blockers = append(blockers, Blocker{"pipe_capacity_degraded", "pipe_capacity_bytes", float64(sample.PipeCapacityBytes), ">=", float64(thresholds.MinPipeCapacityBytes), "bytes", "pipe capacity is below the e2e profile"})
	}
	return blockers
}

type Request struct {
	WorkspaceRoot string
	Profile       string
	Timeout       time.Duration
	Interval      time.Duration
}

type Dependencies struct {
	Sample   func(context.Context, string) (Sample, error)
	Now      func() time.Time
	Sleep    func(context.Context, time.Duration) error
	OnSample func(Sample, int, int, []Blocker)
}

type Result struct {
	OK                       bool       `json:"ok"`
	Kind                     string     `json:"kind"`
	Status                   Status     `json:"status"`
	Profile                  string     `json:"profile"`
	WorkspaceRoot            string     `json:"workspace_root"`
	StartedAt                time.Time  `json:"started_at"`
	FinishedAt               time.Time  `json:"finished_at"`
	WaitedMS                 int64      `json:"waited_ms"`
	SampleCount              int        `json:"sample_count"`
	RequiredStableSamples    int        `json:"required_stable_samples"`
	ConsecutiveStableSamples int        `json:"consecutive_stable_samples"`
	Thresholds               Thresholds `json:"thresholds"`
	LatestSample             *Sample    `json:"latest_sample,omitempty"`
	RecentSamples            []Sample   `json:"recent_samples"`
	Blockers                 []Blocker  `json:"blockers"`
	Warnings                 []string   `json:"warnings"`
}

type AdmissionError struct {
	Status Status
	Cause  error
}

func (e *AdmissionError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Status)
}

func (e *AdmissionError) Unwrap() error { return e.Cause }

func Wait(ctx context.Context, request Request, deps Dependencies) (Result, error) {
	if request.Profile == "" {
		request.Profile = ProfileE2E
	}
	if request.Profile != ProfileE2E {
		return errorResult(request, StatusError, fmt.Errorf("unsupported resource profile %q", request.Profile))
	}
	if request.Interval < time.Second || request.Interval > time.Minute {
		return errorResult(request, StatusError, errors.New("--interval must be between 1s and 60s"))
	}
	if request.Timeout < request.Interval*time.Duration(requiredStableSamples) || request.Timeout > time.Hour {
		return errorResult(request, StatusError, errors.New("--timeout must be at least interval * 3 and at most 60m"))
	}
	if request.WorkspaceRoot == "" {
		return errorResult(request, StatusError, errors.New("workspace root is required"))
	}
	if deps.Sample == nil {
		return errorResult(request, StatusError, errors.New("resource sampler is required"))
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = sleepContext
	}

	profile := E2EProfile()
	started := deps.Now()
	deadline := started.Add(request.Timeout)
	result := Result{
		Kind:                  "resource_wait",
		Profile:               profile.Name,
		WorkspaceRoot:         request.WorkspaceRoot,
		StartedAt:             started,
		RequiredStableSamples: requiredStableSamples,
		RecentSamples:         []Sample{},
		Blockers:              []Blocker{},
		Warnings:              []string{},
	}

	first := true
	for {
		if err := ctx.Err(); err != nil {
			return terminalResult(result, deps.Now(), StatusCancelled, err)
		}
		sampleStarted := deps.Now()
		if !first && sampleStarted.After(deadline) {
			return terminalResult(result, sampleStarted, StatusTimedOut, nil)
		}
		sample, err := deps.Sample(ctx, request.WorkspaceRoot)
		sampleFinished := deps.Now()
		if err != nil {
			status := StatusError
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				status = StatusCancelled
			}
			return terminalResult(result, sampleFinished, status, err)
		}
		if err := ctx.Err(); err != nil {
			return terminalResult(result, sampleFinished, StatusCancelled, err)
		}
		if sample.SampledAt.IsZero() {
			sample.SampledAt = sampleFinished
		}
		sample = normalizedSample(sample)
		thresholds := ResolveThresholds(sample, profile.Thresholds)
		blockers := Evaluate(sample, thresholds)
		result.SampleCount++
		result.Thresholds = thresholds
		result.LatestSample = &sample
		result.Blockers = blockers
		result.RecentSamples = appendRecent(result.RecentSamples, sample)
		if first {
			first = false
		} else if len(blockers) == 0 {
			result.ConsecutiveStableSamples++
		} else {
			result.ConsecutiveStableSamples = 0
		}
		if deps.OnSample != nil {
			deps.OnSample(sample, result.SampleCount, result.ConsecutiveStableSamples, blockers)
		}
		if result.ConsecutiveStableSamples == requiredStableSamples {
			return terminalResult(result, sampleFinished, StatusReady, nil)
		}
		if !sampleFinished.Before(deadline) {
			return terminalResult(result, sampleFinished, StatusTimedOut, nil)
		}
		wait := request.Interval
		if remaining := deadline.Sub(sampleFinished); remaining < wait {
			wait = remaining
		}
		if err := deps.Sleep(ctx, wait); err != nil {
			status := StatusError
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				status = StatusCancelled
			}
			return terminalResult(result, deps.Now(), status, err)
		}
	}
}

func terminalResult(result Result, finished time.Time, status Status, cause error) (Result, error) {
	result.Status = status
	result.OK = status == StatusReady
	result.FinishedAt = finished
	result.WaitedMS = finished.Sub(result.StartedAt).Milliseconds()
	if status == StatusReady {
		return result, nil
	}
	return result, &AdmissionError{Status: status, Cause: cause}
}

func errorResult(request Request, status Status, cause error) (Result, error) {
	result := Result{Kind: "resource_wait", Status: status, Profile: request.Profile, WorkspaceRoot: request.WorkspaceRoot, RecentSamples: []Sample{}, Blockers: []Blocker{}, Warnings: []string{}}
	return result, &AdmissionError{Status: status, Cause: cause}
}

func normalizedSample(sample Sample) Sample {
	if sample.LogicalCPUCount > 0 {
		sample.Load1MPerCPU = sample.Load1M / float64(sample.LogicalCPUCount)
	}
	if sample.TotalMemoryBytes > 0 {
		sample.AvailableMemoryRatio = float64(sample.AvailableMemoryBytes) / float64(sample.TotalMemoryBytes)
	}
	if sample.WorkspaceDiskTotalBytes > 0 {
		sample.WorkspaceDiskAvailableRatio = float64(sample.WorkspaceDiskAvailableBytes) / float64(sample.WorkspaceDiskTotalBytes)
	}
	if sample.TempDiskTotalBytes > 0 {
		sample.TempDiskAvailableRatio = float64(sample.TempDiskAvailableBytes) / float64(sample.TempDiskTotalBytes)
	}
	return sample
}

func ratioFloor(total uint64, ratio float64) uint64 {
	if ratio <= 0 || total == 0 {
		return 0
	}
	value := float64(total) * ratio
	if value >= float64(^uint64(0)) {
		return ^uint64(0)
	}
	return uint64(math.Ceil(value))
}

func maxUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}

func appendRecent(samples []Sample, sample Sample) []Sample {
	samples = append(samples, sample)
	if len(samples) > maxRecentSamples {
		return append([]Sample(nil), samples[len(samples)-maxRecentSamples:]...)
	}
	return samples
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
