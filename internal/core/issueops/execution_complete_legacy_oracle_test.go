package issueops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-harness/internal/core/preflight"
)

func TestExecutionCompletionLegacyOracleUsesInjectedClock(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "198-completion-clock")
	prepareExecutionCompletionFixture(t, stateRoot, &fixture)
	actor := executionActor("codex", "completion-clock")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(fixture.worktree, "turing.json")
	if err := os.WriteFile(report, []byte(`{"status":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 8, 2, 0, 0, 0, 123456789, time.UTC)
	result, err := completeExecutionWithClock(stateRoot, ExecutionCompleteRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor, CWD: fixture.worktree,
		FinalHead:        preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"),
		TuringReportPath: report, Verification: []string{"go test ./... -count=1"},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69", Confirm: true,
	}, ExecutionCompleteDeps{}, func() time.Time { return fixed })
	if err != nil {
		t.Fatal(err)
	}
	want := fixed.Format(time.RFC3339Nano)
	if got := result.Execution.Completion.CompletedAt; got != want {
		t.Fatalf("completed_at=%q want=%q", got, want)
	}
	if got := result.Execution.Lease.ReleasedAt; got != want {
		t.Fatalf("released_at=%q want=%q", got, want)
	}
	persisted, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := persisted.PhaseLedger[IssueOpsPhasePR].CompletedAt; got != want {
		t.Fatalf("pr completed_at=%q want=%q", got, want)
	}
	if got := persisted.PhaseLedger[IssueOpsPhaseDone].EnteredAt; got != want {
		t.Fatalf("done entered_at=%q want=%q", got, want)
	}
}
