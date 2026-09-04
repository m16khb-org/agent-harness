package issueops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
)

const (
	childStartProcessHelperEnv         = "ISSUEOPS_ISSUEOPS_CHILD_START_HELPER"
	childStartProcessStateRootEnv      = "ISSUEOPS_ISSUEOPS_CHILD_START_STATE_ROOT"
	childStartProcessParentIDEnv       = "ISSUEOPS_ISSUEOPS_CHILD_START_PARENT_ID"
	childStartProcessParentWorktreeEnv = "ISSUEOPS_ISSUEOPS_CHILD_START_PARENT_WORKTREE"
	childStartProcessBranchEnv         = "ISSUEOPS_ISSUEOPS_CHILD_START_BRANCH"
	childStartProcessTitleEnv          = "ISSUEOPS_ISSUEOPS_CHILD_START_TITLE"
	childStartProcessReadyDirEnv       = "ISSUEOPS_ISSUEOPS_CHILD_START_READY_DIR"
	childStartProcessGateEnv           = "ISSUEOPS_ISSUEOPS_CHILD_START_GATE"
)

func TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process IssueOps child-start test skipped in -short")
	}
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	parent = seedChildStartParentRefsForTest(t, stateRoot, parent, 2000)
	readyDir := filepath.Join(stateRoot, "ready")
	if err := os.MkdirAll(readyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(stateRoot, "start.gate")

	const workers = 4
	type outcome struct {
		branch string
		output []byte
		err    error
	}
	outcomes := make(chan outcome, workers)
	for i := 0; i < workers; i++ {
		branch := fmt.Sprintf("12%d-child-process-sibling", i)
		title := fmt.Sprintf("process sibling %d", i)
		go func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestIssueOpsDelegationStartProcessHelper$")
			cmd.Env = append(os.Environ(),
				childStartProcessHelperEnv+"=1",
				childStartProcessStateRootEnv+"="+stateRoot,
				childStartProcessParentIDEnv+"="+parent.ID,
				childStartProcessParentWorktreeEnv+"="+parent.WorktreePath,
				childStartProcessBranchEnv+"="+branch,
				childStartProcessTitleEnv+"="+title,
				childStartProcessReadyDirEnv+"="+readyDir,
				childStartProcessGateEnv+"="+gate,
			)
			output, err := cmd.CombinedOutput()
			outcomes <- outcome{branch: branch, output: output, err: err}
		}()
	}

	waitForChildStartReadyMarkers(t, readyDir, workers)
	if err := os.WriteFile(gate, []byte("release\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < workers; i++ {
		result := <-outcomes
		if result.err != nil {
			t.Fatalf("child start subprocess for %s failed: %v\n%s", result.branch, result.err, result.output)
		}
		if !strings.Contains(string(result.output), "child-start-ok "+result.branch) {
			t.Fatalf("child start subprocess for %s did not report success:\n%s", result.branch, result.output)
		}
	}

	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentAfter.ChildCycles) != len(parent.ChildCycles)+workers {
		t.Fatalf("expected %d child refs after process-concurrent start, got %d", len(parent.ChildCycles)+workers, len(parentAfter.ChildCycles))
	}
	for i := 0; i < workers; i++ {
		branch := fmt.Sprintf("12%d-child-process-sibling", i)
		childID := NewIssueOpsID(parent.Repo, branch)
		ref, ok := childRefByID(parentAfter.ChildCycles, childID)
		if !ok {
			t.Fatalf("missing process sibling child ref for %s (%s): %#v", branch, childID, parentAfter.ChildCycles)
		}
		if ref.Branch != branch || ref.Title != fmt.Sprintf("process sibling %d", i) {
			t.Fatalf("process sibling ref for %s should preserve branch/title, got %#v", branch, ref)
		}
	}
}

func TestIssueOpsDelegationStartProcessHelper(t *testing.T) {
	if os.Getenv(childStartProcessHelperEnv) != "1" {
		t.Skip("subprocess helper only")
	}
	stateRoot := os.Getenv(childStartProcessStateRootEnv)
	parentID := os.Getenv(childStartProcessParentIDEnv)
	parentWorktree := os.Getenv(childStartProcessParentWorktreeEnv)
	branch := os.Getenv(childStartProcessBranchEnv)
	title := os.Getenv(childStartProcessTitleEnv)
	readyDir := os.Getenv(childStartProcessReadyDirEnv)
	gate := os.Getenv(childStartProcessGateEnv)
	for name, value := range map[string]string{
		childStartProcessStateRootEnv:      stateRoot,
		childStartProcessParentIDEnv:       parentID,
		childStartProcessParentWorktreeEnv: parentWorktree,
		childStartProcessBranchEnv:         branch,
		childStartProcessTitleEnv:          title,
		childStartProcessReadyDirEnv:       readyDir,
		childStartProcessGateEnv:           gate,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s is required", name)
		}
	}

	ready := filepath.Join(readyDir, branch+".ready")
	parent, err := ReadIssueOps(stateRoot, parentID)
	if err != nil {
		t.Fatal(err)
	}
	child, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: parent.Repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	child, err = stampIssueOpsChildDelegation(stateRoot, parent, child.ID, issueops.IssueOpsChildStartRequest{
		ParentID:           parentID,
		Branch:             branch,
		Title:              title,
		TaskScope:          "process sibling concurrency",
		AcceptanceCriteria: []string{"every process sibling persists"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ready, []byte(branch+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForChildStartGate(t, gate)

	ref, err := appendIssueOpsChildRef(stateRoot, parentID, child, issueops.IssueOpsChildStartRequest{
		ParentID:           parentID,
		Branch:             branch,
		Title:              title,
		TaskScope:          "process sibling concurrency",
		AcceptanceCriteria: []string{"every process sibling persists"},
	}, now, &IssueOpsActor{
		Host: "codex", SessionID: "test-session", AgentID: "test-agent", CWD: parentWorktree,
		NativeProcessAncestry: []issueops.NativeProcessReceipt{{
			PID: 1, StartedAt: "2026-07-22T00:00:00Z", Executable: "/usr/bin/codex",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref.Branch != branch || ref.Title != title || ref.CycleID != child.ID {
		t.Fatalf("ref mismatch for %s: %#v", branch, ref)
	}
	fmt.Printf("child-start-ok %s %s\n", branch, child.ID)
}

func waitForChildStartReadyMarkers(t *testing.T, readyDir string, want int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(readyDir)
		if err == nil {
			ready := 0
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".ready") {
					ready++
				}
			}
			if ready == want {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	entries, _ := os.ReadDir(readyDir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	t.Fatalf("timed out waiting for %d ready markers in %s; saw %v", want, readyDir, names)
}

func waitForChildStartGate(t *testing.T, gate string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(gate); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for child start gate %s", gate)
}

func seedChildStartParentRefsForTest(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord, count int) issueops.IssueOpsRecord {
	t.Helper()
	parent.ChildCycles = make([]issueops.IssueOpsChildCycleRef, 0, count)
	for i := 0; i < count; i++ {
		branch := fmt.Sprintf("90%d-existing-child-ref", i)
		parent.ChildCycles = append(parent.ChildCycles, issueops.IssueOpsChildCycleRef{
			CycleID:   NewIssueOpsID(parent.Repo, branch),
			Branch:    branch,
			Title:     fmt.Sprintf("existing child ref %d", i),
			CreatedAt: "2026-08-02T00:00:00Z",
		})
	}
	return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
}
