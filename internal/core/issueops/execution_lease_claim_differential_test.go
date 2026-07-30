package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestExecutionLeaseClaimDifferential(t *testing.T) {
	direct := readClaimGolden(t, "direct-v1.golden.json")
	if direct.Scenario != "direct-v1" || direct.SideEffects["holder_index"] != "created_with_record" || direct.SideEffects["token"] != "removed_after_apply" {
		t.Fatalf("unexpected direct fixture: %#v", direct)
	}
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "197-claim-golden-direct")
	actor := executionActor("codex", "claim-golden-direct")
	claimed, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	})
	if err != nil {
		t.Fatalf("direct vertical claim: %v", err)
	}
	if claimed.Execution.Lease.Status != direct.Lease.Status || claimed.Execution.Lease.ClaimTokenSHA256 != direct.Lease.ClaimTokenSHA256 || claimed.Execution.Lease.ReleasedAt != direct.Lease.ReleasedAt {
		t.Fatalf("direct lease=%#v, golden=%#v", claimed.Execution.Lease, direct.Lease)
	}
	if _, err := os.Stat(fixture.tokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("direct claim token should be removed after apply: %v", err)
	}
	indexes, err := ListLeaseHolderIndexes(stateRoot)
	if err != nil {
		t.Fatalf("list holder indexes: %v", err)
	}
	if len(indexes) != 1 || indexes[0].LifecycleID != fixture.record.ID || indexes[0].Generation != 1 || indexes[0].Host != actor.Host || indexes[0].SessionID != actor.SessionID {
		t.Fatalf("direct holder index=%#v", indexes)
	}

	orca := readClaimGolden(t, "orca-v1.golden.json")
	if orca.Scenario != "orca-v1" || orca.SideEffects["local_packet_recheck"] != "inside_repository" || orca.SideEffects["token"] != "removed_after_apply" {
		t.Fatalf("unexpected Orca fixture: %#v", orca)
	}
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: claim golden\n\n## verification\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	orcaRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	orcaClaimed, err := claimViaVerticalWithDeps(context.Background(), orcaRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "claim-golden-orca"), CWD: prepared.Workspace.Root,
		TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: reader})
	if err != nil {
		t.Fatalf("Orca vertical claim: %v", err)
	}
	if orcaClaimed.Execution.Lease.Status != orca.Lease.Status || orcaClaimed.Execution.Lease.ClaimTokenSHA256 != orca.Lease.ClaimTokenSHA256 {
		t.Fatalf("Orca lease=%#v, golden=%#v", orcaClaimed.Execution.Lease, orca.Lease)
	}
	if _, err := os.Stat(prepared.ClaimTokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Orca claim token should be removed after apply: %v", err)
	}

	errorsGolden := readClaimGolden(t, "error-contract.golden.json")
	if errorsGolden.Scenario != "error-contract" || len(errorsGolden.Errors) != 4 {
		t.Fatalf("unexpected error fixture: %#v", errorsGolden)
	}
	assertClaimGoldenErrors(t, errorsGolden.Errors)
}

type claimGolden struct {
	SchemaVersion int               `json:"schema_version"`
	Scenario      string            `json:"scenario"`
	Lease         model.WriteLease  `json:"lease"`
	SideEffects   map[string]string `json:"side_effects"`
	Errors        []string          `json:"errors"`
}

func readClaimGolden(t *testing.T, name string) claimGolden {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "claimvertical", name))
	if err != nil {
		t.Fatal(err)
	}
	var golden claimGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.SchemaVersion != 1 {
		t.Fatalf("claim fixture %s schema_version=%d", name, golden.SchemaVersion)
	}
	return golden
}

func assertClaimGoldenErrors(t *testing.T, expected []string) {
	t.Helper()
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "197-claim-golden-errors-generation")
	_, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 2, Actor: executionActor("codex", "claim-golden-generation"), CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	})
	if got := fmt.Sprint(err); got != strings.Replace(expected[0], "N", "2", 1) {
		t.Fatalf("generation error=%q, want %q", got, strings.Replace(expected[0], "N", "2", 1))
	}

	stateRoot = t.TempDir()
	fixture = newClaimableExecutionFixture(t, stateRoot, "197-claim-golden-errors-cwd")
	_, err = claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: executionActor("codex", "claim-golden-cwd"), CWD: t.TempDir(), TokenFile: fixture.tokenPath,
	})
	if got := fmt.Sprint(err); got != expected[1] {
		t.Fatalf("cwd error=%q, want %q", got, expected[1])
	}

	stateRoot = t.TempDir()
	fixture = newClaimableExecutionFixture(t, stateRoot, "197-claim-golden-errors-token")
	if err := os.WriteFile(fixture.tokenPath, []byte("not-the-current-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: executionActor("codex", "claim-golden-token"), CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	})
	if got := fmt.Sprint(err); got != expected[2] {
		t.Fatalf("token error=%q, want %q", got, expected[2])
	}

	issueBody := "## acceptance criteria\n\n- [ ] AC-01: claim golden\n\n## verification\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	orcaRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	_, err = claimViaVerticalWithDeps(context.Background(), orcaRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "claim-golden-missing-digests"), CWD: prepared.Workspace.Root, TokenFile: prepared.ClaimTokenPath,
	}, ExecutionClaimDependencies{ReadIssue: reader})
	if got := fmt.Sprint(err); got != expected[3] {
		t.Fatalf("Orca sealed-context error=%q, want %q", got, expected[3])
	}
}

func TestExecuteExecutionClaimUsesInjectedHandler(t *testing.T) {
	testExecuteExecutionClaimUsesInjectedHandler(t)
}

func testExecuteExecutionClaimUsesInjectedHandler(t *testing.T) {
	t.Helper()
	_, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{Action: ExecutionActionClaim, ID: "io-claim-handler"}, ExecutionActionDependencies{})
	if !errors.Is(err, ErrClaimHandlerUnavailable) {
		t.Fatalf("claim error=%v, want unavailable handler", err)
	}

	called := 0
	result, err := ExecuteExecution(context.Background(), t.TempDir(), ExecutionActionRequest{
		Action: ExecutionActionClaim, ID: "io-claim-handler", Generation: 3, CWD: "/canonical/worktree", TokenFile: "/canonical/worktree/token",
	}, ExecutionActionDependencies{Claim: func(_ context.Context, stateRoot string, request ExecutionClaimRequest) (ExecutionResult, error) {
		called++
		if stateRoot == "" || request.ID != "io-claim-handler" || request.Generation != 3 || request.CWD != "/canonical/worktree" || request.TokenFile != "/canonical/worktree/token" {
			t.Fatalf("unexpected injected claim request: root=%q request=%+v", stateRoot, request)
		}
		return ExecutionResult{OK: true, ID: request.ID}, nil
	}})
	if err != nil {
		t.Fatalf("execute claim: %v", err)
	}
	got, ok := result.(ExecutionResult)
	if !ok || !got.OK || got.ID != "io-claim-handler" || called != 1 {
		t.Fatalf("result=%#v called=%d", result, called)
	}
}
