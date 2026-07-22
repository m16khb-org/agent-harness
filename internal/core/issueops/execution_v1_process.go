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

const nativeProcessProbeTimeoutV1 = 3 * time.Second

const (
	NativeProcessStatusLive             = "live"
	NativeProcessStatusDead             = "dead"
	NativeProcessStatusIdentityMismatch = "identity-mismatch"
	NativeProcessStatusUnknown          = "unknown"
)

type workspaceProcessV1 struct {
	PID     int
	Command string
	FD      string
	Access  string
	Path    string
}

type nativeProcessSnapshotEntryV1 struct {
	ParentPID int
	Receipt   model.NativeProcessReceiptV1
}

// ObserveNativeProcessReceiptV1 reads the operating-system identity used to
// fence PID reuse. Callers persist this exact receipt, not a self-reported time.
func ObserveNativeProcessReceiptV1(pid int) (model.NativeProcessReceiptV1, error) {
	if pid <= 0 {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("native process pid must be positive")
	}
	alive, err := nativePIDAliveV1(pid)
	if err != nil {
		return model.NativeProcessReceiptV1{}, err
	}
	if !alive {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("native process pid %d is not live", pid)
	}
	startedRaw, err := processFieldV1(pid, "lstart=")
	if err != nil {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("read native process start identity: %w", err)
	}
	started, err := parseNativeProcessStartV1(startedRaw)
	if err != nil {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("parse native process start identity: %w", err)
	}
	executable, err := processFieldV1(pid, "comm=")
	if err != nil {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("read native process executable identity: %w", err)
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return model.NativeProcessReceiptV1{}, fmt.Errorf("native process executable identity is empty")
	}
	return model.NativeProcessReceiptV1{
		PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
	}, nil
}

// ObserveNativeProcessAncestryV1 captures one operating-system process table
// snapshot and returns pid followed by each parent. A single snapshot keeps the
// PID/start/executable tuples internally consistent while the hook is deciding
// whether it is a descendant of the durable lease holder.
func ObserveNativeProcessAncestryV1(pid int) ([]model.NativeProcessReceiptV1, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("native process pid must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeoutV1)
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
	snapshot, err := parseNativeProcessSnapshotV1(string(output))
	if err != nil {
		return nil, err
	}
	return nativeProcessAncestryFromSnapshotV1(snapshot, pid)
}

func parseNativeProcessSnapshotV1(output string) (map[int]nativeProcessSnapshotEntryV1, error) {
	snapshot := make(map[int]nativeProcessSnapshotEntryV1)
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
		started, err := parseNativeProcessStartV1(strings.Join(fields[2:7], " "))
		if err != nil {
			return nil, fmt.Errorf("parse native process snapshot start identity for pid %d: %w", pid, err)
		}
		executable := strings.TrimSpace(strings.Join(fields[7:], " "))
		if executable == "" {
			return nil, fmt.Errorf("native process snapshot executable identity is empty for pid %d", pid)
		}
		snapshot[pid] = nativeProcessSnapshotEntryV1{
			ParentPID: parentPID,
			Receipt: model.NativeProcessReceiptV1{
				PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable,
			},
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func nativeProcessAncestryFromSnapshotV1(snapshot map[int]nativeProcessSnapshotEntryV1, pid int) ([]model.NativeProcessReceiptV1, error) {
	const maxNativeProcessAncestryV1 = 128
	ancestry := make([]model.NativeProcessReceiptV1, 0, 8)
	seen := make(map[int]bool)
	for len(ancestry) < maxNativeProcessAncestryV1 {
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
	return nil, fmt.Errorf("native process ancestry exceeds %d entries", maxNativeProcessAncestryV1)
}

func parseNativeProcessStartV1(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if started, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return started, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid process start identity %q", value)
}

func requireExactLiveNativeProcessReceiptV1(receipt model.NativeProcessReceiptV1) error {
	status, observed, err := inspectNativeProcessReceiptV1(receipt)
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

// InspectNativeProcessReceiptV1 exposes the same PID-reuse-safe, read-only
// observation used by lease replacement to operational inventory collectors.
func InspectNativeProcessReceiptV1(receipt model.NativeProcessReceiptV1) (string, model.NativeProcessReceiptV1, error) {
	return inspectNativeProcessReceiptV1(receipt)
}

func inspectNativeProcessReceiptV1(receipt model.NativeProcessReceiptV1) (string, model.NativeProcessReceiptV1, error) {
	alive, err := nativePIDAliveV1(receipt.PID)
	if err != nil {
		return NativeProcessStatusUnknown, model.NativeProcessReceiptV1{}, fmt.Errorf("inspect native process identity: %w", err)
	}
	if !alive {
		return NativeProcessStatusDead, model.NativeProcessReceiptV1{}, nil
	}
	observed, err := ObserveNativeProcessReceiptV1(receipt.PID)
	if err != nil {
		alive, retryErr := nativePIDAliveV1(receipt.PID)
		if retryErr == nil && !alive {
			return NativeProcessStatusDead, model.NativeProcessReceiptV1{}, nil
		}
		return NativeProcessStatusUnknown, model.NativeProcessReceiptV1{}, fmt.Errorf("inspect live native process identity: %w", err)
	}
	if observed.StartedAt != receipt.StartedAt || observed.Executable != receipt.Executable {
		return NativeProcessStatusIdentityMismatch, observed, nil
	}
	return NativeProcessStatusLive, observed, nil
}

func nativePIDAliveV1(pid int) (bool, error) {
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

func processFieldV1(pid int, field string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeoutV1)
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

func inspectWorkspaceProcessesV1(root string, excluded map[int]bool) ([]workspaceProcessV1, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nativeProcessProbeTimeoutV1)
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
	return parseWorkspaceProcessesV1(stdout.String(), root, excluded)
}

func parseWorkspaceProcessesV1(output, root string, excluded map[int]bool) ([]workspaceProcessV1, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	var pid int
	var command, fd, access string
	result := []workspaceProcessV1{}
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
			if pid <= 0 || excluded[pid] || !pathWithinResolvedV1(path, root) {
				continue
			}
			if fd == "cwd" || access == "w" || access == "u" {
				result = append(result, workspaceProcessV1{PID: pid, Command: command, FD: fd, Access: access, Path: path})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func pathWithinResolvedV1(path, root string) bool {
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
