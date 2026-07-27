package issueops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

// Preview is the operator's evidence for whether confirm can start the sealed
// Orca execution. It may skip mutation-only actor checks, but it must not call
// a noncanonical CWD executable when confirm will reject that same request.
func TestOrcaPreparePreviewRejectsTheSameNoncanonicalCWDAsConfirm(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	badCWD := filepath.Join(record.Repo, "not-the-source-or-worktree")

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: badCWD,
		OwnerHost: "claude",
	}, ExecutionPrepareDependencies{
		Orca: readyOrcaFake(), ReadIssue: executionIssueSnapshotReader,
	})
	if err == nil || !strings.Contains(err.Error(), "Orca prepare cwd") {
		t.Fatalf("preview must reject the same noncanonical cwd as confirm: %v", err)
	}
}

// The owner prompt treats the remote issue as the implementation SSOT.
// Showing a successful preview for a body that confirm cannot seal moves the
// first failure behind the operator's explicit approval.
func TestOrcaPreparePreviewValidatesTheRemoteIssueOwnerContract(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	readCalls := 0
	incomplete := func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		readCalls++
		return port.ExecutionIssueSnapshot{
			URL:  request.URL,
			Body: "## 문제\n\n수용 기준과 정확한 검증 명령 블록이 아직 없다.\n",
		}, nil
	}

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo,
		OwnerHost: "claude",
	}, ExecutionPrepareDependencies{
		Orca: readyOrcaFake(), ReadIssue: incomplete,
	})
	if err == nil || !strings.Contains(err.Error(), "acceptance IDs") {
		t.Fatalf("preview must reject an owner contract that confirm cannot seal: %v", err)
	}
	if readCalls != 1 {
		t.Fatalf("preview must read the remote issue exactly once, got %d", readCalls)
	}
}
