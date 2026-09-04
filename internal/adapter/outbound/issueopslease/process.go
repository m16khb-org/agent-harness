package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	leasedomain "issueops/internal/domain/issueopslease"
)

const processProbeTimeout = 3 * time.Second

func InspectNativeProcess(ctx context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
	alive, err := pidAlive(receipt.PID)
	if err != nil {
		return "unknown", leasedomain.ProcessReceipt{}, fmt.Errorf("inspect native process identity: %w", err)
	}
	if !alive {
		return "dead", leasedomain.ProcessReceipt{}, nil
	}
	observed, err := observeReceipt(ctx, receipt.PID)
	if err != nil {
		return "unknown", leasedomain.ProcessReceipt{}, fmt.Errorf("inspect live native process identity: %w", err)
	}
	if observed != receipt {
		return "identity-mismatch", observed, nil
	}
	return "live", observed, nil
}

func observeReceipt(ctx context.Context, pid int) (leasedomain.ProcessReceipt, error) {
	if pid <= 0 {
		return leasedomain.ProcessReceipt{}, fmt.Errorf("native process pid must be positive")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probe, cancel := context.WithTimeout(ctx, processProbeTimeout)
	defer cancel()
	startedRaw, err := processField(probe, pid, "lstart=")
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	started, err := parseProcessStart(startedRaw)
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	executable, err := processField(probe, pid, "comm=")
	if err != nil {
		return leasedomain.ProcessReceipt{}, err
	}
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return leasedomain.ProcessReceipt{}, fmt.Errorf("native process executable identity is empty")
	}
	return leasedomain.ProcessReceipt{PID: pid, StartedAt: started.UTC().Format(time.RFC3339), Executable: executable}, nil
}
func processField(ctx context.Context, pid int, field string) (string, error) {
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
		return "", fmt.Errorf("native process field %s is empty", field)
	}
	return value, nil
}
func parseProcessStart(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"Mon Jan _2 15:04:05 2006", "Mon Jan 2 15:04:05 2006"} {
		if started, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return started, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid process start identity %q", value)
}
func pidAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
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
