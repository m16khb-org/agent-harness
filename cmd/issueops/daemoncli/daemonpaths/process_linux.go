//go:build linux

package daemonpaths

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	linuxATClockTicks              = 17
	maxRepresentableTicksPerSecond = uint64(time.Second)
)

func InspectProcess(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("pid must be positive")
	}
	procDir := filepath.Join("/proc", strconv.Itoa(pid))
	executable, err := os.Readlink(filepath.Join(procDir, "exe"))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process executable: %w", err)
	}
	executable = strings.TrimSuffix(executable, " (deleted)")
	executable, err = canonicalExecutable(executable)
	if err != nil {
		return ProcessIdentity{}, err
	}
	stat, err := os.ReadFile(filepath.Join(procDir, "stat"))
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	systemStat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	auxv, err := os.ReadFile("/proc/self/auxv")
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	startTime, err := linuxProcessStartTime(stat, systemStat, auxv)
	if err != nil {
		return ProcessIdentity{}, fmt.Errorf("read process start time: %w", err)
	}
	return ProcessIdentity{StartTime: startTime, Executable: executable, ExecutablePathStable: true}, nil
}

func linuxProcessStartTime(stat, systemStat, auxv []byte) (string, error) {
	startTicks, err := linuxProcessStartTicks(stat)
	if err != nil {
		return "", err
	}
	bootSeconds, err := linuxBootTime(systemStat)
	if err != nil {
		return "", err
	}
	ticksPerSecond, err := linuxClockTicksPerSecond(auxv)
	if err != nil {
		return "", err
	}

	wholeSeconds := startTicks / ticksPerSecond
	remainingTicks := startTicks % ticksPerSecond
	nanoseconds := remainingTicks * uint64(time.Second) / ticksPerSecond
	if wholeSeconds > math.MaxInt64 {
		return "", fmt.Errorf("process start seconds overflow")
	}
	startSeconds := int64(wholeSeconds)
	if bootSeconds > math.MaxInt64-startSeconds {
		return "", fmt.Errorf("process start epoch overflow")
	}
	startedAt := time.Unix(bootSeconds+startSeconds, int64(nanoseconds)).UTC()
	if startedAt.Year() > 9999 {
		return "", fmt.Errorf("process start time is outside RFC3339 range")
	}
	return startedAt.Format(time.RFC3339Nano), nil
}

func linuxProcessStartTicks(stat []byte) (uint64, error) {
	closing := strings.LastIndexByte(string(stat), ')')
	if closing < 0 {
		return 0, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(string(stat)[closing+1:])
	const startTimeIndex = 19 // field 22 after removing pid and parenthesized comm
	if len(fields) <= startTimeIndex || fields[startTimeIndex] == "" {
		return 0, fmt.Errorf("process stat start time is missing")
	}
	startTicks, err := strconv.ParseUint(fields[startTimeIndex], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse process stat start time: %w", err)
	}
	return startTicks, nil
}

func linuxBootTime(systemStat []byte) (int64, error) {
	for _, line := range strings.Split(string(systemStat), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "btime" {
			continue
		}
		bootSeconds, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || bootSeconds < 0 {
			return 0, fmt.Errorf("invalid system boot time %q", fields[1])
		}
		return bootSeconds, nil
	}
	return 0, fmt.Errorf("system boot time is missing")
}

func linuxClockTicksPerSecond(auxv []byte) (uint64, error) {
	wordSize := strconv.IntSize / 8
	pairSize := wordSize * 2
	if len(auxv)%pairSize != 0 {
		return 0, fmt.Errorf("invalid auxiliary vector length")
	}
	for offset := 0; offset < len(auxv); offset += pairSize {
		tag := linuxNativeWord(auxv[offset : offset+wordSize])
		value := linuxNativeWord(auxv[offset+wordSize : offset+pairSize])
		if tag == 0 {
			break
		}
		if tag != linuxATClockTicks {
			continue
		}
		if value == 0 || value > maxRepresentableTicksPerSecond {
			return 0, fmt.Errorf("invalid clock ticks per second %d", value)
		}
		return value, nil
	}
	return 0, fmt.Errorf("clock ticks per second are missing")
}

func linuxNativeWord(data []byte) uint64 {
	if strconv.IntSize == 64 {
		return binary.NativeEndian.Uint64(data)
	}
	return uint64(binary.NativeEndian.Uint32(data))
}
