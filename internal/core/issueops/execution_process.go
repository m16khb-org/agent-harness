package issueops

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/core/issueops/model"
)

const nativeProcessProbeTimeout = 3 * time.Second

const (
	NativeProcessStatusLive             = "live"
	NativeProcessStatusDead             = "dead"
	NativeProcessStatusIdentityMismatch = "identity-mismatch"
	NativeProcessStatusUnknown          = "unknown"
)

type workspaceProcess struct {
	PID     int
	Command string
	FD      string
	Access  string
	Path    string
}

type nativeProcessSnapshotEntry struct {
	ParentPID int
	Receipt   model.NativeProcessReceipt
}

// ObserveNativeProcessReceipt reads the operating-system identity used to
// fence PID reuse. Callers persist this exact receipt, not a self-reported time.
func ObserveNativeProcessReceipt(pid int) (model.NativeProcessReceipt, error) {
	if pid <= 0 {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	alive, err := nativePIDAlive(pid)
	if err != nil {
		return model.NativeProcessReceipt{}, err
	}
	if !alive {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process pid %d is not live", pid)
	}
	startedRaw, err := processField(pid, "lstart=")
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("read native process start identity: %w", err)
	}
	started, err := parseNativeProcessStart(startedRaw)
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("parse native process start identity: %w", err)
	}
	executable, err := processField(pid, "comm=")
	if err != nil {
		return model.NativeProcessReceipt{}, fmt.Errorf("read native process executable identity: %w", err)
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return model.NativeProcessReceipt{}, fmt.Errorf("native process executable identity is empty")
	}
	return model.NativeProcessReceipt{
		PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
	}, nil
}

// ObserveNativeProcessAncestry captures one operating-system process table
// snapshot and returns pid followed by each parent. A single snapshot keeps the
// PID/start/executable tuples internally consistent while the hook is deciding
// whether it is a descendant of the durable lease holder.
func ObserveNativeProcessAncestry(pid int) ([]model.NativeProcessReceipt, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("native process pid must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-ww", "-axo", "pid=,ppid=,lstart=,comm=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("read native process ancestry: %w", err)
	}
	snapshot, err := parseNativeProcessSnapshot(string(output))
	if err != nil {
		return nil, err
	}
	return nativeProcessAncestryFromSnapshot(snapshot, pid)
}

func parseNativeProcessSnapshot(output string) (map[int]nativeProcessSnapshotEntry, error) {
	snapshot := make(map[int]nativeProcessSnapshotEntry)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			if len(fields) == 0 {
				continue
			}
			return nil, fmt.Errorf("invalid native process snapshot row %q", scanner.Text())
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("invalid native process snapshot pid %q", fields[0])
		}
		parentPID, err := strconv.Atoi(fields[1])
		if err != nil || parentPID < 0 {
			return nil, fmt.Errorf("invalid native process snapshot parent pid %q", fields[1])
		}
		started, err := parseNativeProcessStart(strings.Join(fields[2:7], " "))
		if err != nil {
			return nil, fmt.Errorf("parse native process snapshot start identity for pid %d: %w", pid, err)
		}
		executable := strings.TrimSpace(strings.Join(fields[7:], " "))
		if executable == "" {
			return nil, fmt.Errorf("native process snapshot executable identity is empty for pid %d", pid)
		}
		snapshot[pid] = nativeProcessSnapshotEntry{
			ParentPID: parentPID,
			Receipt: model.NativeProcessReceipt{
				PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
			},
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func nativeProcessAncestryFromSnapshot(snapshot map[int]nativeProcessSnapshotEntry, pid int) ([]model.NativeProcessReceipt, error) {
	const maxNativeProcessAncestry = 128
	ancestry := make([]model.NativeProcessReceipt, 0, 8)
	seen := make(map[int]bool)
	for len(ancestry) < maxNativeProcessAncestry {
		if seen[pid] {
			return nil, fmt.Errorf("native process ancestry contains a cycle at pid %d", pid)
		}
		seen[pid] = true
		entry, ok := snapshot[pid]
		if !ok {
			return nil, fmt.Errorf("native process ancestry is missing pid %d", pid)
		}
		ancestry = append(ancestry, entry.Receipt)
		if entry.ParentPID == 0 {
			return ancestry, nil
		}
		pid = entry.ParentPID
	}
	return nil, fmt.Errorf("native process ancestry exceeds %d entries", maxNativeProcessAncestry)
}

func parseNativeProcessStart(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if started, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return started, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid process start identity %q", value)
}

func requireExactLiveNativeProcessReceipt(receipt model.NativeProcessReceipt) error {
	status, observed, err := inspectNativeProcessReceipt(receipt)
	if err != nil {
		return err
	}
	if status != "live" {
		return fmt.Errorf("native process identity is not live: pid=%d status=%s", receipt.PID, status)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return fmt.Errorf("native process identity does not match live PID %d", receipt.PID)
	}
	return nil
}

// InspectNativeProcessReceipt exposes the same PID-reuse-safe, read-only
// observation used by lease replacement to operational inventory collectors.
func InspectNativeProcessReceipt(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	return inspectNativeProcessReceipt(receipt)
}

func inspectNativeProcessReceipt(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	alive, err := nativePIDAlive(receipt.PID)
	if err != nil {
		return NativeProcessStatusUnknown, model.NativeProcessReceipt{}, fmt.Errorf("inspect native process identity: %w", err)
	}
	if !alive {
		return NativeProcessStatusDead, model.NativeProcessReceipt{}, nil
	}
	observed, err := ObserveNativeProcessReceipt(receipt.PID)
	if err != nil {
		alive, retryErr := nativePIDAlive(receipt.PID)
		if retryErr == nil && !alive {
			return NativeProcessStatusDead, model.NativeProcessReceipt{}, nil
		}
		return NativeProcessStatusUnknown, model.NativeProcessReceipt{}, fmt.Errorf("inspect live native process identity: %w", err)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return NativeProcessStatusIdentityMismatch, observed, nil
	}
	return NativeProcessStatusLive, observed, nil
}

func nativePIDAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	err = process.Signal(syscall.Signal(0))
	switch {
	case err == nil, errors.Is(err, os.ErrPermission):
		return true, nil
	case errors.Is(err, os.ErrProcessDone), errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

func processField(pid int, field string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", field)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", fmt.Errorf("ps returned an empty %s field", field)
	}
	return value, nil
}

func inspectWorkspaceProcesses(root string, excluded map[int]bool) ([]workspaceProcess, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-Fpcfna", "+D", root)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !(errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "") {
			return nil, fmt.Errorf("lsof workspace inventory: %s", strings.TrimSpace(stderr.String()))
		}
	}
	return parseWorkspaceProcesses(stdout.String(), root, excluded)
}

func parseWorkspaceProcesses(output, root string, excluded map[int]bool) ([]workspaceProcess, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	var pid int
	var command, fd, access string
	result := []workspaceProcess{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		tag, value := line[0], strings.TrimSpace(line[1:])
		switch tag {
		case 'p':
			pid, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid lsof process id %q", value)
			}
			command, fd, access = "", "", ""
		case 'c':
			command = value
		case 'f':
			fd, access = value, ""
		case 'a':
			access = value
		case 'n':
			path := strings.TrimSuffix(value, " (deleted)")
			if pid <= 0 || excluded[pid] || !pathWithinResolved(path, root) {
				continue
			}
			if fd == "cwd" || access == "w" || access == "u" {
				result = append(result, workspaceProcess{PID: pid, Command: command, FD: fd, Access: access, Path: path})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func pathWithinResolved(path, root string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
