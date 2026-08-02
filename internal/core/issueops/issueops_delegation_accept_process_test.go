package issueops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	acceptProcessHelperModeEnv     = "HARNESS_ISSUEOPS_ACCEPT_HELPER"
	acceptProcessHelperStateEnv    = "HARNESS_ISSUEOPS_ACCEPT_STATE"
	acceptProcessHelperParentEnv   = "HARNESS_ISSUEOPS_ACCEPT_PARENT"
	acceptProcessHelperChildEnv    = "HARNESS_ISSUEOPS_ACCEPT_CHILD"
	acceptProcessHelperEvidenceEnv = "HARNESS_ISSUEOPS_ACCEPT_EVIDENCE"
	acceptProcessHelperReadyEnv    = "HARNESS_ISSUEOPS_ACCEPT_READY"
	acceptProcessHelperGateEnv     = "HARNESS_ISSUEOPS_ACCEPT_GATE"
)

func TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process IssueOps delegation accept test skipped in -short")
	}

	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	const workers = 4

	childIDs := make([]string, workers)
	evidenceByChild := make(map[string]string, workers)
	for i := 0; i < workers; i++ {
		started, err := startIssueOpsChildForTest(stateRoot, parent, IssueOpsChildStartRequest{
			ParentID:           parent.ID,
			Branch:             fmt.Sprintf("124-child-accept-%d", i),
			Title:              fmt.Sprintf("accepted child %d", i),
			TaskScope:          "cross-process accepted child verdict preservation",
			AcceptanceCriteria: []string{"parent verdict evidence is preserved"},
		})
		if err != nil {
			t.Fatalf("start child %d: %v", i, err)
		}
		child := started.Child
		child.Phase = IssueOpsPhaseDone
		writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
		childIDs[i] = child.ID
		evidenceByChild[child.ID] = fmt.Sprintf("worker-%d accepted verified diff", i)
	}

	gate := filepath.Join(stateRoot, "accept-gate")
	readyMarkers := make([]string, workers)
	outputs := make(chan acceptProcessOutput, workers)
	for i, childID := range childIDs {
		ready := filepath.Join(stateRoot, fmt.Sprintf("ready-worker-%d", i))
		readyMarkers[i] = ready
		cmd := acceptProcessHelperCommand(t, stateRoot, parent.ID, childID, evidenceByChild[childID], ready, gate)
		go func(worker int, cmd *exec.Cmd) {
			output, err := cmd.CombinedOutput()
			outputs <- acceptProcessOutput{worker: worker, output: string(output), err: err}
		}(i, cmd)
	}

	waitForAcceptProcessReadyMarkers(t, readyMarkers, 5*time.Second)
	if err := os.WriteFile(gate, []byte("go\n"), 0o600); err != nil {
		t.Fatalf("release accept gate: %v", err)
	}

	for i := 0; i < workers; i++ {
		result := <-outputs
		if result.err != nil {
			t.Fatalf("worker %d failed: %v\ncombined output:\n%s", result.worker, result.err, result.output)
		}
		if !strings.Contains(result.output, "accepted "+childIDs[result.worker]) {
			t.Fatalf("worker %d output did not preserve child id: %q", result.worker, result.output)
		}
	}

	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, childID := range childIDs {
		ref, ok := childRefByID(parentAfter.ChildCycles, childID)
		if !ok {
			t.Fatalf("accepted child ref %s missing from parent refs: %#v", childID, parentAfter.ChildCycles)
		}
		if ref.ValidationVerdict != "accepted" {
			t.Fatalf("child %s verdict lost: %#v", childID, ref)
		}
		if ref.ValidatedAt == "" {
			t.Fatalf("child %s validation timestamp lost: %#v", childID, ref)
		}
		if got, want := strings.Join(ref.ValidationEvidence, "\n"), evidenceByChild[childID]; got != want {
			t.Fatalf("child %s evidence lost: got %q want %q", childID, got, want)
		}
	}
}

type acceptProcessOutput struct {
	worker int
	output string
	err    error
}

func acceptProcessHelperCommand(t *testing.T, stateRoot, parentID, childID, evidence, ready, gate string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestIssueOpsDelegationAcceptProcessHelper$")
	cmd.Env = append(
		os.Environ(),
		acceptProcessHelperModeEnv+"=1",
		acceptProcessHelperStateEnv+"="+stateRoot,
		acceptProcessHelperParentEnv+"="+parentID,
		acceptProcessHelperChildEnv+"="+childID,
		acceptProcessHelperEvidenceEnv+"="+evidence,
		acceptProcessHelperReadyEnv+"="+ready,
		acceptProcessHelperGateEnv+"="+gate,
	)
	return cmd
}

func waitForAcceptProcessReadyMarkers(t *testing.T, markers []string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		missing := []string{}
		for _, marker := range markers {
			if _, err := os.Stat(marker); err != nil {
				missing = append(missing, filepath.Base(marker))
			}
		}
		if len(missing) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("workers did not reach ready barrier in time; missing=%v", missing)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIssueOpsDelegationAcceptProcessHelper(t *testing.T) {
	if os.Getenv(acceptProcessHelperModeEnv) == "" {
		t.Skip("subprocess helper only")
	}
	stateRoot := os.Getenv(acceptProcessHelperStateEnv)
	parentID := os.Getenv(acceptProcessHelperParentEnv)
	childID := os.Getenv(acceptProcessHelperChildEnv)
	evidence := os.Getenv(acceptProcessHelperEvidenceEnv)
	ready := os.Getenv(acceptProcessHelperReadyEnv)
	gate := os.Getenv(acceptProcessHelperGateEnv)
	if stateRoot == "" || parentID == "" || childID == "" || evidence == "" || ready == "" || gate == "" {
		t.Fatalf("missing helper environment")
	}
	parent, err := ReadIssueOps(stateRoot, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ready, []byte(childID+"\n"), 0o600); err != nil {
		t.Fatalf("write ready marker: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(gate); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for accept gate")
		}
		time.Sleep(10 * time.Millisecond)
	}
	result, err := acceptIssueOpsChildForTest(stateRoot, parent, childID, []string{evidence})
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "accepted" || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("accepted receipt missing verdict/timestamp: %#v", result.ParentRef)
	}
	fmt.Fprintf(os.Stdout, "accepted %s evidence %q at %s\n", childID, strings.Join(result.ParentRef.ValidationEvidence, "\n"), result.ParentRef.ValidatedAt)
}
