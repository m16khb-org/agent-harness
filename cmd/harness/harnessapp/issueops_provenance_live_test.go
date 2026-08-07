package harnessapp

import (
	"agent-harness/internal/adapter/issueops"
	commandparsecontract "agent-harness/internal/contract/commandparse"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/commandparse"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurrentRelayReleasedReseedGeneratedCommandDogfood(t *testing.T) {
	binary := os.Getenv("AGENT_HARNESS_CURRENT_RELAY_DOGFOOD_BINARY")
	lifecycleID := os.Getenv("AGENT_HARNESS_CURRENT_RELAY_DOGFOOD_ID")
	if binary == "" || lifecycleID == "" {
		t.Skip("live current-relay dogfood requires an exact binary and lifecycle id")
	}
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryDigest := sha256.Sum256(binaryBytes)

	live, err := issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), lifecycleID)
	if err != nil {
		t.Fatal(err)
	}
	if live.Execution == nil || live.Execution.Orca == nil {
		t.Fatal("live dogfood lifecycle is not an Orca execution")
	}
	stateRoot, fixture, _, _, _ := seedOrcaClaimSnapshot(t)
	orcaBinding := *live.Execution.Orca
	fixture.IssueURL = live.IssueURL
	fixture.BranchPrepare.Provider = "github"
	fixture.BranchPrepare.IssueURL = live.IssueURL
	fixture.Execution.Orca = &orcaBinding
	fixture.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	fixture.Execution.Lease.Holder = nil
	fixture.Execution.Lease.ClaimTokenSHA256 = ""
	if _, err := issueops.WriteIssueOps(stateRoot, fixture); err != nil {
		t.Fatal(err)
	}

	stateBase := t.TempDir()
	configuredStateRoot := filepath.Join(stateBase, "issueops_v1")
	if err := os.Rename(stateRoot, configuredStateRoot); err != nil {
		t.Fatal(err)
	}
	actor := claimWiringActor(t)
	process := actor.SessionProcess
	previewArgs := []string{
		"issueops", "execution", "replace", "--id", fixture.ID, "--expected-generation", "1", "--preview",
		"--host", actor.Host, "--session-id", actor.SessionID,
		"--session-pid", processPID(process), "--session-started-at", process.StartedAt,
		"--session-executable", process.Executable, "--cwd", fixture.Execution.Workspace.Root, "--json",
	}
	previewBytes := runCurrentRelayDogfoodBinary(t, binary, stateBase, fixture.Execution.Workspace.Root, previewArgs)
	var preview issueops.ExecutionReplaceResult
	if err := json.Unmarshal(previewBytes, &preview); err != nil {
		t.Fatalf("decode replacement preview: %v\n%s", err, previewBytes)
	}
	assertCurrentRelayDogfoodCommand(t, preview.NextCommand, binary, hex.EncodeToString(binaryDigest[:]), 1)

	commandTokens := commandparse.SplitCommandTokens(preview.NextCommand)
	reseedBytes := runCurrentRelayDogfoodBinary(t, commandTokens[0], stateBase, fixture.Execution.Workspace.Root, commandTokens[1:])
	persisted, err := issueops.ReadIssueOps(configuredStateRoot, fixture.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Generation != 2 || persisted.Execution.Lease.Status != issueopscontract.LeaseStatusClaimable {
		t.Fatalf("current-relay reseed lease = %+v", persisted.Execution.Lease)
	}
	output := strings.TrimSpace(string(reseedBytes))
	marker := " next="
	at := strings.Index(output, marker)
	if at < 0 {
		t.Fatalf("current-relay reseed output omitted next command: %q", output)
	}
	assertCurrentRelayDogfoodCommand(t, output[at+len(marker):], binary, hex.EncodeToString(binaryDigest[:]), 2)
}

func runCurrentRelayDogfoodBinary(t *testing.T, binary, stateBase, cwd string, args []string) []byte {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = cwd
	command.Env = append(os.Environ(), "HARNESS_STATE_DIR="+stateBase)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("current-relay dogfood %v: %v\n%s", args, err, output)
	}
	return output
}

func assertCurrentRelayDogfoodCommand(t *testing.T, command, binary, digest string, generation uint64) {
	t.Helper()
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 3 || tokens[0] != binary || tokens[1] != "issueops" {
		t.Fatalf("generated command does not select exact dogfood binary: %q", command)
	}
	_, provenance, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(tokens[2:])
	if err != nil || !present {
		t.Fatalf("generated command provenance err=%v present=%t: %q", err, present, command)
	}
	if provenance.ExecutablePath != binary || provenance.ExecutableSHA256 != digest || provenance.LeaseGeneration != generation {
		t.Fatalf("generated command provenance = %+v", provenance)
	}
}

func processPID(receipt *issueopscontract.NativeProcessReceipt) string {
	if receipt == nil {
		return "0"
	}
	return fmt.Sprintf("%d", receipt.PID)
}
