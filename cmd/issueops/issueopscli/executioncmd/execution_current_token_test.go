package executioncmd

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestClaimRejectsAmbiguousCurrentAndFileTokenSelectors(t *testing.T) {
	err := runClaim([]string{
		"--id", "io-439", "--generation", "2", "--claim-current-token", "--claim-token-file", "/tmp/token", "--json",
	}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous claim token selectors error = %v, want exactly-one rejection", err)
	}
}

func TestClaimRejectsPartialActorFlags(t *testing.T) {
	err := runClaim([]string{
		"--id", "io-direct", "--generation", "2", "--claim-current-token", "--host", "codex", "--json",
	}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "all ACTOR_FLAGS or none") {
		t.Fatalf("partial claim actor flags error = %v", err)
	}
}

func TestClaimWithoutActorFlagsUsesNativeReceipt(t *testing.T) {
	previousExecute := execDeps.ExecuteExecution
	t.Cleanup(func() { execDeps.ExecuteExecution = previousExecute })

	var got model.ExecutionActionRequest
	execDeps.ExecuteExecution = func(_ context.Context, _ string, request model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		got = request
		return struct{}{}, nil
	}
	observation := nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CODEX_THREAD_ID" {
				return "codex-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/direct", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return []model.NativeProcessReceipt{
				{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/issueops"},
				{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/codex/bin/codex"},
			}, nil
		},
	}
	err := runClaim([]string{
		"--id", "io-direct", "--generation", "2", "--claim-current-token", "--json",
	}, Deps{
		StateRoot:              func() string { return t.TempDir() },
		PrintJSON:              func(any) error { return nil },
		nativeActorObservation: &observation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Actor.Host != "codex" || got.Actor.SessionID != "codex-session" ||
		got.Actor.SessionProcess == nil || got.Actor.SessionProcess.PID != 42 ||
		got.CWD != "/repo.worktrees/direct" {
		t.Fatalf("claim request=%+v", got)
	}
}

func TestClaimWithoutActorFlagsUsesOmoNativeReceipt(t *testing.T) {
	previousExecute := execDeps.ExecuteExecution
	t.Cleanup(func() { execDeps.ExecuteExecution = previousExecute })

	var got model.ExecutionActionRequest
	execDeps.ExecuteExecution = func(_ context.Context, _ string, request model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		got = request
		return struct{}{}, nil
	}
	observation := nativeActorObservation{
		Getenv: func(key string) string {
			if key == "PI_SESSION_ID" {
				return "omo-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/direct", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return []model.NativeProcessReceipt{
				{PID: 101, StartedAt: "2026-08-12T00:00:01Z", Executable: "/tmp/issueops"},
				{PID: 42, StartedAt: "2026-08-12T00:00:00Z", Executable: "/Users/test/Library/pnpm/bin/omo"},
			}, nil
		},
	}
	err := runClaim([]string{
		"--id", "io-direct", "--generation", "2", "--claim-current-token", "--json",
	}, Deps{
		StateRoot:              func() string { return t.TempDir() },
		PrintJSON:              func(any) error { return nil },
		nativeActorObservation: &observation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Actor.Host != "omo" || got.Actor.SessionID != "omo-session" ||
		got.Actor.SessionProcess == nil || got.Actor.SessionProcess.PID != 42 ||
		got.CWD != "/repo.worktrees/direct" {
		t.Fatalf("claim request=%+v", got)
	}
}
