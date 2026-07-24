package systemresource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"agent-harness/internal/core/resourcewait"
)

type Collector struct {
	mu          sync.Mutex
	readFile    func(string) ([]byte, error)
	run         func(context.Context, string, ...string) ([]byte, error)
	now         func() time.Time
	measurePipe func() (int, error)
	previous    *swapCounters
}

type swapCounters struct {
	in, out uint64
	at      time.Time
}

func NewCollector() *Collector {
	return &Collector{
		readFile:    os.ReadFile,
		run:         runFixedCommand,
		now:         time.Now,
		measurePipe: resourcewait.MeasurePipeCapacity,
	}
}

func (c *Collector) Sample(ctx context.Context, workspaceRoot string) (resourcewait.Sample, error) {
	if workspaceRoot == "" {
		return resourcewait.Sample{}, errors.New("workspace root is required")
	}
	info, err := os.Stat(workspaceRoot)
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("workspace root: %w", err)
	}
	if !info.IsDir() {
		return resourcewait.Sample{}, fmt.Errorf("workspace root %q is not a directory", workspaceRoot)
	}

	var sample resourcewait.Sample
	switch runtime.GOOS {
	case "darwin":
		sample, err = c.sampleDarwin(ctx)
	case "linux":
		sample, err = c.sampleLinux()
	default:
		return resourcewait.Sample{}, fmt.Errorf("unsupported_platform: %s", runtime.GOOS)
	}
	if err != nil {
		return resourcewait.Sample{}, err
	}
	workspaceTotal, workspaceAvailable, err := filesystemCapacity(workspaceRoot)
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("workspace filesystem: %w", err)
	}
	tempTotal, tempAvailable, err := filesystemCapacity(os.TempDir())
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("temporary filesystem: %w", err)
	}
	pipeCapacity, err := c.measurePipe()
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("pipe capacity: %w", err)
	}
	sample.SampledAt = c.now()
	sample.WorkspaceDiskTotalBytes = workspaceTotal
	sample.WorkspaceDiskAvailableBytes = workspaceAvailable
	sample.TempDiskTotalBytes = tempTotal
	sample.TempDiskAvailableBytes = tempAvailable
	sample.PipeCapacityBytes = pipeCapacity
	return sample, nil
}

func (c *Collector) sampleLinux() (resourcewait.Sample, error) {
	loadRaw, err := c.readFile("/proc/loadavg")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("linux loadavg: %w", err)
	}
	load, err := parseLinuxLoad(string(loadRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	memRaw, err := c.readFile("/proc/meminfo")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("linux meminfo: %w", err)
	}
	totalMemory, availableMemory, err := parseLinuxMemory(string(memRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	vmstatRaw, err := c.readFile("/proc/vmstat")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("linux vmstat: %w", err)
	}
	swapIn, swapOut, err := parseLinuxSwap(string(vmstatRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	return resourcewait.Sample{
		LogicalCPUCount:      runtime.NumCPU(),
		Load1M:               load,
		TotalMemoryBytes:     totalMemory,
		AvailableMemoryBytes: availableMemory,
		SwapIOBytesPerSec:    c.swapRate(swapIn, swapOut, uint64(os.Getpagesize())),
	}, nil
}

func (c *Collector) sampleDarwin(ctx context.Context) (resourcewait.Sample, error) {
	loadRaw, err := c.run(ctx, "/usr/sbin/sysctl", "-n", "vm.loadavg")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("darwin loadavg: %w", err)
	}
	load, err := parseDarwinLoad(string(loadRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	cpuRaw, err := c.run(ctx, "/usr/sbin/sysctl", "-n", "hw.logicalcpu")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("darwin logical cpu: %w", err)
	}
	cpu, err := parsePositiveInt(string(cpuRaw), "darwin logical cpu")
	if err != nil {
		return resourcewait.Sample{}, err
	}
	memoryRaw, err := c.run(ctx, "/usr/sbin/sysctl", "-n", "hw.memsize")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("darwin memory: %w", err)
	}
	memory, err := parseUint(strings.TrimSpace(string(memoryRaw)), "darwin memory")
	if err != nil {
		return resourcewait.Sample{}, err
	}
	pressureRaw, err := c.run(ctx, "/usr/bin/memory_pressure", "-Q")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("darwin memory pressure: %w", err)
	}
	availablePercent, err := parseDarwinAvailablePercent(string(pressureRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	vmRaw, err := c.run(ctx, "/usr/bin/vm_stat")
	if err != nil {
		return resourcewait.Sample{}, fmt.Errorf("darwin vm_stat: %w", err)
	}
	pageSize, swapIn, swapOut, err := parseDarwinVMStat(string(vmRaw))
	if err != nil {
		return resourcewait.Sample{}, err
	}
	return resourcewait.Sample{
		LogicalCPUCount:      cpu,
		Load1M:               load,
		TotalMemoryBytes:     memory,
		AvailableMemoryBytes: uint64(float64(memory) * availablePercent / 100),
		SwapIOBytesPerSec:    c.swapRate(swapIn, swapOut, pageSize),
	}, nil
}

func (c *Collector) swapRate(in, out, pageSize uint64) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	previous := c.previous
	c.previous = &swapCounters{in: in, out: out, at: now}
	if previous == nil || !now.After(previous.at) || in < previous.in || out < previous.out {
		return 0
	}
	deltaPages := in - previous.in + out - previous.out
	if deltaPages > ^uint64(0)/pageSize {
		return ^uint64(0)
	}
	seconds := now.Sub(previous.at).Seconds()
	if seconds <= 0 {
		return 0
	}
	return uint64(float64(deltaPages*pageSize) / seconds)
}

func filesystemCapacity(path string) (uint64, uint64, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return 0, 0, err
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(resolved, &stat); err != nil {
		return 0, 0, err
	}
	if stat.Bsize <= 0 {
		return 0, 0, errors.New("invalid filesystem block size")
	}
	blockSize := uint64(stat.Bsize)
	return multiply(uint64(stat.Blocks), blockSize), multiply(uint64(stat.Bavail), blockSize), nil
}

func multiply(left, right uint64) uint64 {
	if left != 0 && right > ^uint64(0)/left {
		return ^uint64(0)
	}
	return left * right
}

func runFixedCommand(ctx context.Context, path string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	return command.Output()
}

func parseLinuxLoad(raw string) (float64, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, errors.New("linux loadavg is empty")
	}
	return parseFloat(fields[0], "linux loadavg")
}

func parseLinuxMemory(raw string) (uint64, uint64, error) {
	values := parseKeyValues(raw)
	total, ok := values["MemTotal"]
	if !ok {
		return 0, 0, errors.New("linux meminfo missing MemTotal")
	}
	available, ok := values["MemAvailable"]
	if !ok {
		return 0, 0, errors.New("linux meminfo missing MemAvailable")
	}
	return kibibytes(total), kibibytes(available), nil
}

func parseLinuxSwap(raw string) (uint64, uint64, error) {
	values := parseKeyValues(raw)
	in, ok := values["pswpin"]
	if !ok {
		return 0, 0, errors.New("linux vmstat missing pswpin")
	}
	out, ok := values["pswpout"]
	if !ok {
		return 0, 0, errors.New("linux vmstat missing pswpout")
	}
	return in, out, nil
}

func parseDarwinLoad(raw string) (float64, error) {
	fields := strings.Fields(strings.NewReplacer("{", " ", "}", " ").Replace(raw))
	if len(fields) == 0 {
		return 0, errors.New("darwin loadavg is empty")
	}
	return parseFloat(fields[0], "darwin loadavg")
}

func parseDarwinAvailablePercent(raw string) (float64, error) {
	for _, field := range strings.Fields(raw) {
		field = strings.TrimRight(field, ".,;")
		if !strings.HasSuffix(field, "%") {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSuffix(field, "%"), 64)
		if err == nil && value >= 0 && value <= 100 {
			return value, nil
		}
	}
	return 0, errors.New("darwin memory_pressure missing available percentage")
}

func parseDarwinVMStat(raw string) (uint64, uint64, uint64, error) {
	pageSize := uint64(0)
	values := map[string]uint64{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for index, field := range fields {
				if field == "of" && index+1 < len(fields) {
					pageSize, _ = parseUint(strings.Trim(fields[index+1], "()"), "darwin page size")
				}
			}
		}
		for _, key := range []string{"Pages swapped in", "Pages swapped out", "Swapins", "Swapouts"} {
			if strings.HasPrefix(strings.TrimSpace(line), key+":") {
				value := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), key+":")), ".")
				parsed, err := parseUint(value, key)
				if err != nil {
					return 0, 0, 0, err
				}
				values[key] = parsed
			}
		}
	}
	swapIn, hasSwapIn := values["Pages swapped in"]
	if !hasSwapIn {
		swapIn, hasSwapIn = values["Swapins"]
	}
	swapOut, hasSwapOut := values["Pages swapped out"]
	if !hasSwapOut {
		swapOut, hasSwapOut = values["Swapouts"]
	}
	if pageSize == 0 || !hasSwapIn || !hasSwapOut {
		return 0, 0, 0, errors.New("darwin vm_stat missing required fields")
	}
	return pageSize, swapIn, swapOut, nil
}

func parseKeyValues(raw string) map[string]uint64 {
	values := map[string]uint64{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(strings.Replace(line, ":", " ", 1))
		if len(fields) < 2 {
			continue
		}
		if value, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			values[fields[0]] = value
		}
	}
	return values
}

func parsePositiveInt(raw, field string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return value, nil
}

func parseUint(raw, field string) (uint64, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}
	return value, nil
}

func parseFloat(raw, field string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return value, nil
}

func kibibytes(value uint64) uint64 {
	return multiply(value, 1024)
}
