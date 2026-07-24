package resourcecli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/resourcewait"
)

func TestRunWaitKeepsJSONStdoutCleanWhenProgressIsJSONL(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunWithDeps([]string{"wait", "--workspace-root", t.TempDir(), "--timeout", "3s", "--interval", "1s", "--progress", "jsonl", "--json"}, Deps{
		Stdout: &stdout,
		Stderr: &stderr,
		Sample: func(context.Context, string) (resourcewait.Sample, error) {
			return resourcewait.Sample{
				LogicalCPUCount:             8,
				Load1M:                      1,
				TotalMemoryBytes:            32 << 30,
				AvailableMemoryBytes:        16 << 30,
				WorkspaceDiskTotalBytes:     512 << 30,
				WorkspaceDiskAvailableBytes: 128 << 30,
				TempDiskTotalBytes:          512 << 30,
				TempDiskAvailableBytes:      128 << 30,
				PipeCapacityBytes:           16 << 10,
			}, nil
		},
		Now: func() time.Time { return now },
		Sleep: func(_ context.Context, duration time.Duration) error {
			now = now.Add(duration)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunWithDeps() error = %v", err)
	}
	var result resourcewait.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout.String())
	}
	if result.Status != resourcewait.StatusReady {
		t.Fatalf("status = %q, want ready", result.Status)
	}
	var sampleEvent map[string]any
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("stderr line is not JSONL: %v\n%s", err, line)
		}
		if event["event"] == "sample" {
			sampleEvent = event
		}
	}
	if _, ok := sampleEvent["elapsed_ms"]; !ok {
		t.Fatalf("sample progress is missing elapsed_ms: %s", stderr.String())
	}
}

func TestRunWaitRejectsUnsupportedProfile(t *testing.T) {
	err := RunWithDeps([]string{"wait", "--profile", "custom"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "only e2e") {
		t.Fatalf("error = %v, want e2e profile validation", err)
	}
}
