package executioncmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/core/issueops"
)

func TestReadExecutionIssueSnapshotFileAcceptsPrivateBoundedJSON(t *testing.T) {
	path := writeExecutionSnapshotTestFile(t, validExecutionSnapshotJSON(), 0o600)
	got, err := readExecutionIssueSnapshotFile(path)
	if err != nil || got == nil || got.Source != "glab_mcp" || got.Provider != "gitlab" {
		t.Fatalf("valid snapshot file failed: got=%#v err=%v", got, err)
	}
	if got.WebURL != "https://gitlab.example.com/acme/repo/-/issues/69" || got.State != "opened" {
		t.Fatalf("snapshot fields drifted: %#v", got)
	}
	if got, err := readExecutionIssueSnapshotFile(""); err != nil || got != nil {
		t.Fatalf("empty path = %#v, %v", got, err)
	}
}

func TestExecutionSnapshotFileFlagMapsToPrepareRequest(t *testing.T) {
	stateRoot, repo, id := executionSnapshotCLIRecord(t)
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	path := writeExecutionSnapshotTestFile(t, validExecutionSnapshotJSON(), 0o600)
	var output any
	err = Run([]string{
		"prepare", "--id", id, "--mode", "direct", "--owner-host", "claude",
		"--issue-snapshot-file", path,
		"--host", "codex", "--session-id", "snapshot-cli",
		"--session-pid", fmt.Sprint(receipt.PID),
		"--session-started-at", receipt.StartedAt,
		"--session-executable", receipt.Executable,
		"--cwd", repo, "--json",
	}, Deps{
		StateRoot: func() string { return stateRoot },
		Prepare: func(_ context.Context, _ string, request issueops.ExecutionPrepareRequest, _ issueops.ExecutionPrepareInvocation) (issueops.ExecutionPrepareResult, error) {
			return issueops.ExecutionPrepareResult{
				OK: true, ID: request.ID, RequestedMode: request.Mode, ResolvedMode: "direct",
			}, nil
		},
		PrintJSON: func(value any) error {
			output = value
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := output.(issueops.ExecutionPrepareResult)
	if !ok || got.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("snapshot flag did not reach core: result=%#v", output)
	}
}

func TestReadExecutionIssueSnapshotFileRejectsUnsafeInputs(t *testing.T) {
	valid := validExecutionSnapshotJSON()
	tests := map[string]func(*testing.T) string{
		"directory": func(t *testing.T) string {
			return t.TempDir()
		},
		"world_readable": func(t *testing.T) string {
			return writeExecutionSnapshotTestFile(t, valid, 0o644)
		},
		"oversized": func(t *testing.T) string {
			return writeExecutionSnapshotTestFile(t, strings.Repeat("x", (1<<20)+1), 0o600)
		},
		"invalid_json": func(t *testing.T) string {
			return writeExecutionSnapshotTestFile(t, "{", 0o600)
		},
		"unknown_field": func(t *testing.T) string {
			return writeExecutionSnapshotTestFile(t, strings.TrimSuffix(valid, "}")+`,"server_namespace":"private"}`, 0o600)
		},
		"trailing_value": func(t *testing.T) string {
			return writeExecutionSnapshotTestFile(t, valid+"\n{}", 0o600)
		},
		"symlink": func(t *testing.T) string {
			target := writeExecutionSnapshotTestFile(t, valid, 0o600)
			link := filepath.Join(t.TempDir(), "snapshot.json")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			return link
		},
	}
	for name, fixture := range tests {
		t.Run(name, func(t *testing.T) {
			if got, err := readExecutionIssueSnapshotFile(fixture(t)); err == nil {
				t.Fatalf("unsafe snapshot was accepted: %#v", got)
			}
		})
	}
}

func writeExecutionSnapshotTestFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func validExecutionSnapshotJSON() string {
	return `{"provider":"gitlab","source":"glab_mcp","web_url":"https://gitlab.example.com/acme/repo/-/issues/69","body":"AC-69","state":"opened"}`
}

func executionSnapshotCLIRecord(t *testing.T) (string, string, string) {
	t.Helper()
	stateRoot, repo := t.TempDir(), t.TempDir()
	branch := "69-snapshot-cli"
	issueURL := "https://gitlab.example.com/acme/repo/-/work_items/69"
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, branch),
		Repo:          repo,
		Branch:        branch,
		Phase:         issueops.IssueOpsPhasePlan,
		IssueURL:      issueURL,
		DesignReview:  &issueopscontract.IssueOpsDesignReview{Approved: true, ReviewedAt: "2026-07-28T00:00:00Z"},
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider: "gitlab", IssueURL: issueURL, Branch: branch,
			BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true,
			CreatedAt: "2026-07-28T00:00:00Z",
		},
		CreatedAt: "2026-07-28T00:00:00Z",
		UpdatedAt: "2026-07-28T00:00:00Z",
	}
	written, err := issueops.WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, repo, written.ID
}
