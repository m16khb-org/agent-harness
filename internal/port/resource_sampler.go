package port

import (
	"context"
	"time"
)

type ResourceSample struct {
	SampledAt                   time.Time `json:"sampled_at"`
	LogicalCPUCount             int       `json:"logical_cpu_count"`
	Load1M                      float64   `json:"load_1m"`
	Load1MPerCPU                float64   `json:"load_1m_per_cpu"`
	TotalMemoryBytes            uint64    `json:"total_memory_bytes"`
	AvailableMemoryBytes        uint64    `json:"available_memory_bytes"`
	AvailableMemoryRatio        float64   `json:"available_memory_ratio"`
	SwapIOBytesPerSec           uint64    `json:"swap_io_bytes_per_sec"`
	WorkspaceDiskTotalBytes     uint64    `json:"workspace_disk_total_bytes"`
	WorkspaceDiskAvailableBytes uint64    `json:"workspace_disk_available_bytes"`
	WorkspaceDiskAvailableRatio float64   `json:"workspace_disk_available_ratio"`
	TempDiskTotalBytes          uint64    `json:"temp_disk_total_bytes"`
	TempDiskAvailableBytes      uint64    `json:"temp_disk_available_bytes"`
	TempDiskAvailableRatio      float64   `json:"temp_disk_available_ratio"`
	PipeCapacityBytes           int       `json:"pipe_capacity_bytes"`
}

type ResourceSampler interface {
	Sample(context.Context, string) (ResourceSample, error)
}
