package executioncmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"agent-harness/internal/adapter/issueops"
	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func TestExecutionResumeCLIParsesOnlyTheExactFlagSurface(t *testing.T) {
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"resume", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "3",
		"--host", "codex", "--session-id", "session-resume",
		"--session-pid", fmt.Sprint(receipt.PID),
		"--session-started-at", receipt.StartedAt,
		"--session-executable", receipt.Executable,
		"--cwd", "/repo.worktrees/resume", "--confirm", "--json",
	}
	if err := Run(args, Deps{}); err == nil || !strings.Contains(err.Error(), "state root is unavailable") {
		t.Fatalf("exact resume flags did not reach execution routing: %v", err)
	}

	withSnapshot := append(append([]string(nil), args...), "--issue-snapshot-file", "/tmp/issue.json")
	if err := Run(withSnapshot, Deps{}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("resume accepted issue snapshot flag: %v", err)
	}
}

func TestExecutionResumeCLIInvokesInjectedHandler(t *testing.T) {
	stateRoot := t.TempDir()
	args := []string{
		"resume", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "3",
		"--host", "codex", "--session-id", "session-resume",
		"--session-pid", "42", "--session-started-at", "2026-07-31T00:00:00Z",
		"--session-executable", "/usr/local/bin/codex", "--cwd", "/repo.worktrees/resume", "--confirm", "--json",
	}
	calls := 0
	var output any
	err := Run(args, Deps{
		StateRoot: func() string { return stateRoot },
		Resume: func(_ context.Context, gotRoot string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			calls++
			if gotRoot != stateRoot || request.ID != "io-aaaaaaaaaaaa" || request.ExpectedGeneration != 3 || request.CWD != "/repo.worktrees/resume" || !request.Confirm {
				t.Fatalf("resume handler request=%+v state_root=%q", request, gotRoot)
			}
			return issueops.ExecutionResumeResult{OK: true, ID: request.ID, ResumeDisposition: "existing_binding"}, nil
		},
		PrintJSON: func(value any) error { output = value; return nil },
	})
	if err != nil || calls != 1 {
		t.Fatalf("resume CLI err=%v calls=%d", err, calls)
	}
	result, ok := output.(issueops.ExecutionResumeResult)
	if !ok || !result.OK || result.ID != "io-aaaaaaaaaaaa" || result.ResumeDisposition != "existing_binding" {
		t.Fatalf("resume CLI output=%#v", output)
	}
}

func TestExecutionResumeCLIInvokesHandlerWithObservedActor(t *testing.T) {
	stateRoot := t.TempDir()
	args := []string{"resume", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "3", "--confirm", "--json"}
	observation := nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CLAUDE_CODE_SESSION_ID" {
				return "claude-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/resume", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return []model.NativeProcessReceipt{
				{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/agent-harness"},
				{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/claude/bin/claude"},
			}, nil
		},
	}
	calls := 0
	err := Run(args, Deps{
		StateRoot: func() string { return stateRoot },
		Resume: func(_ context.Context, _ string, request issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			calls++
			if request.Actor.Host != "claude" || request.Actor.SessionID != "claude-session" ||
				request.Actor.SessionProcess == nil || request.Actor.SessionProcess.PID != 42 ||
				request.CWD != "/repo.worktrees/resume" {
				t.Fatalf("observed resume request=%+v", request)
			}
			return issueops.ExecutionResumeResult{OK: true, ID: request.ID}, nil
		},
		PrintJSON:              func(any) error { return nil },
		nativeActorObservation: &observation,
	})
	if err != nil || calls != 1 {
		t.Fatalf("actor-free resume err=%v calls=%d", err, calls)
	}
}

func TestExecutionReplaceCLIInvokesExecutionWithObservedActor(t *testing.T) {
	previousExecute := execDeps.ExecuteExecution
	t.Cleanup(func() { execDeps.ExecuteExecution = previousExecute })

	var got model.ExecutionActionRequest
	execDeps.ExecuteExecution = func(_ context.Context, _ string, request model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		got = request
		return model.ExecutionReplaceResult{
			OK: true, ID: request.ID, Action: request.ReplaceAction,
			Execution: model.Execution{Lease: model.WriteLease{Generation: request.ExpectedGeneration}},
		}, nil
	}
	observation := nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CODEX_THREAD_ID" {
				return "codex-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/parent", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return []model.NativeProcessReceipt{
				{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/agent-harness"},
				{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/codex/bin/codex"},
			}, nil
		},
	}
	err := Run([]string{
		"replace", "--id", "io-aaaaaaaaaaaa", "--expected-generation", "13", "--preview", "--json",
	}, Deps{
		StateRoot:              func() string { return t.TempDir() },
		PrintJSON:              func(any) error { return nil },
		nativeActorObservation: &observation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionReplace || got.ReplaceAction != model.ExecutionReplacePreview || got.ExpectedGeneration != 13 {
		t.Fatalf("replace request=%+v", got)
	}
	if got.Actor.Host != "codex" || got.Actor.SessionID != "codex-session" ||
		got.Actor.SessionProcess == nil || got.Actor.SessionProcess.PID != 42 ||
		got.CWD != "/repo.worktrees/parent" {
		t.Fatalf("observed replace request=%+v", got)
	}
}

func TestResolveResumeActorObservesNativeIdentityWhenActorFlagsAreAbsent(t *testing.T) {
	flags, visited := resumeActorFlagsForTest(t, nil)
	ancestry := []model.NativeProcessReceipt{
		{PID: 101, StartedAt: "2026-08-01T00:00:01Z", Executable: "/tmp/agent-harness"},
		{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/opt/codex/bin/codex"},
	}
	actor, cwd, err := resolveNativeActor("resume", flags, visited, nativeActorObservation{
		Getenv: func(key string) string {
			if key == "CODEX_THREAD_ID" {
				return "codex-session"
			}
			return ""
		},
		Getwd: func() (string, error) { return "/repo.worktrees/resume", nil },
		PID:   func() int { return 101 },
		ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
			return ancestry, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.Host != "codex" || actor.SessionID != "codex-session" || cwd != "/repo.worktrees/resume" {
		t.Fatalf("resolved actor=%+v cwd=%q", actor, cwd)
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID != 42 || len(actor.ProcessAncestry) != 1 {
		t.Fatalf("native host receipt was not selected: %+v", actor)
	}
}

func TestResolveResumeActorPreservesCompleteExplicitFlags(t *testing.T) {
	args := []string{
		"--host", "claude", "--session-id", "claude-session", "--agent-id", "worker-1",
		"--session-pid", "42", "--session-started-at", "2026-08-01T00:00:00Z",
		"--session-executable", "/opt/claude/bin/claude", "--cwd", "/repo.worktrees/resume",
	}
	flags, visited := resumeActorFlagsForTest(t, args)
	actor, cwd, err := resolveNativeActor("resume", flags, visited, nativeActorObservation{
		Getenv: func(string) string { t.Fatal("explicit flags must not read native env"); return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.Host != "claude" || actor.SessionID != "claude-session" || actor.AgentID != "worker-1" ||
		actor.SessionProcess == nil || actor.SessionProcess.PID != 42 || cwd != "/repo.worktrees/resume" {
		t.Fatalf("explicit actor=%+v cwd=%q", actor, cwd)
	}
}

func TestResolveResumeActorRejectsPartialExplicitFlags(t *testing.T) {
	for name, args := range map[string][]string{
		"host only":     {"--host", "codex"},
		"cwd only":      {"--cwd", "/repo.worktrees/resume"},
		"agent id only": {"--agent-id", "worker-1"},
	} {
		t.Run(name, func(t *testing.T) {
			flags, visited := resumeActorFlagsForTest(t, args)
			_, _, err := resolveNativeActor("resume", flags, visited, nativeActorObservation{})
			if err == nil || !strings.Contains(err.Error(), "all ACTOR_FLAGS or none") {
				t.Fatalf("partial actor flags error = %v", err)
			}
		})
	}
}

func resumeActorFlagsForTest(t *testing.T, args []string) (actorFlags, map[string]bool) {
	t.Helper()
	fs := flag.NewFlagSet("resume actor test", flag.ContinueOnError)
	flags := addActorFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return flags, visitedFlagNames(fs)
}

func TestExecutionResumeUsageIsAdvertisedOnce(t *testing.T) {
	if got := strings.Count(Usage, "issueops execution resume"); got != 1 {
		t.Fatalf("resume usage count = %d", got)
	}
	if !strings.Contains(Usage, "execution resume --id ID --expected-generation N [ACTOR_FLAGS] --confirm") {
		t.Fatalf("resume usage does not advertise actor-free recovery: %s", Usage)
	}
}

func TestExecutionReplaceUsageAdvertisesOptionalActorFlags(t *testing.T) {
	if !strings.Contains(Usage, "[--issue-snapshot-file PATH] [ACTOR_FLAGS] [--confirm]") {
		t.Fatalf("replace usage does not advertise actor-free recovery: %s", Usage)
	}
}
