package issueopscompletion

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	completioncontract "agent-harness/internal/contract/issueopscompletion"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

var fixedCompletionTime = time.Date(2026, 8, 2, 1, 2, 3, 4, time.UTC)

func TestCompleteCommitsBeforeBestEffortOrcaSettle(t *testing.T) {
	trace := []string{}
	repository := &repositoryFake{record: activeCompletionRecord("orca"), trace: &trace}
	environment := &environmentFake{trace: &trace, canonical: true, head: strings.Repeat("a", 40), report: "/worktree/report.json"}
	settler := &settlerFake{trace: &trace, err: errors.New("orca unavailable")}
	service := NewService(repository, environment, fixedClock{fixedCompletionTime}, tracedLiveInspector(&trace), settler)

	result, err := service.Complete(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.OrcaTaskError != "orca unavailable" || result.OrcaTaskSettled {
		t.Fatalf("settle projection = %+v", result)
	}
	wantTrace := []string{"process", "read", "artifact", "cwd", "head", "report", "commit", "settle:run-198:task-198"}
	if !reflect.DeepEqual(trace, wantTrace) {
		t.Fatalf("trace = %#v, want %#v", trace, wantTrace)
	}
	if repository.record.Phase != "done" || repository.record.Lease.Status != "released" || repository.record.Completion == nil {
		t.Fatalf("completion was not committed: %+v", repository.record)
	}
}

func TestCompleteIdenticalRetrySkipsEnvironmentAndStillSettles(t *testing.T) {
	record := activeCompletionRecord("orca")
	record.Phase = "done"
	record.Lease.Status = "released"
	record.Lease.Holder = nil
	record.Lease.ReleasedAt = fixedCompletionTime.Format(time.RFC3339Nano)
	record.Completion = &completioncontract.Completion{
		FinalHead: strings.Repeat("a", 40), TuringReportPath: "/worktree/report.json",
		Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198",
		CompletedAt: fixedCompletionTime.Format(time.RFC3339Nano),
	}
	trace := []string{}
	repository := &repositoryFake{record: record, trace: &trace}
	service := NewService(repository, &environmentFake{trace: &trace, canonical: true}, fixedClock{fixedCompletionTime}, tracedLiveInspector(&trace), &settlerFake{trace: &trace})

	result, err := service.Complete(context.Background(), validRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !result.OrcaTaskSettled {
		t.Fatalf("retry result = %+v", result)
	}
	if want := []string{"process", "read", "cwd", "artifact", "settle:run-198:task-198"}; !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace = %#v, want %#v", trace, want)
	}
}

func TestCompleteRejectsNonCanonicalCWDWithoutCommit(t *testing.T) {
	trace := []string{}
	repository := &repositoryFake{record: activeCompletionRecord("direct"), trace: &trace}
	environment := &environmentFake{trace: &trace, canonical: false}
	service := NewService(repository, environment, fixedClock{fixedCompletionTime}, tracedLiveInspector(&trace), nil)

	_, err := service.Complete(context.Background(), validRequest())
	if err == nil || err.Error() != "completion cwd must be the canonical worktree" {
		t.Fatalf("error = %v", err)
	}
	if repository.commits != 0 {
		t.Fatalf("commits = %d", repository.commits)
	}
}

func TestCompleteRepositoryFailureDoesNotSettle(t *testing.T) {
	trace := []string{}
	repository := &repositoryFake{record: activeCompletionRecord("orca"), trace: &trace, commitErr: errors.New("commit failed")}
	environment := &environmentFake{trace: &trace, canonical: true, head: strings.Repeat("a", 40), report: "/worktree/report.json"}
	service := NewService(repository, environment, fixedClock{fixedCompletionTime}, tracedLiveInspector(&trace), &settlerFake{trace: &trace})

	_, err := service.Complete(context.Background(), validRequest())
	if err == nil || err.Error() != "commit failed" {
		t.Fatalf("error = %v", err)
	}
	for _, step := range trace {
		if strings.HasPrefix(step, "settle:") {
			t.Fatalf("settler ran after failed commit: %#v", trace)
		}
	}
}

func TestCompleteReopenedGenerationKeepsExactlyOnceContract(t *testing.T) {
	for _, test := range []struct {
		name       string
		generation uint64
		newHead    string
	}{
		{name: "issue 261 generation 6", generation: 6, newHead: "ff27b34520e4e253d8ebfd523e4e4352bf93e8d8"},
		{name: "issue 237 generation 3", generation: 3, newHead: "9c8db06313cfce39d17a53123f84da1fc4bc7b34"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := activeCompletionRecord("direct")
			record.Lease.Generation = test.generation
			record.Ledger = map[string]completioncontract.LedgerEntry{
				"pr":   {Phase: "pr", Notes: []string{"stale: completed execution reseed"}},
				"done": {Phase: "done", EnteredAt: "old-done", Notes: []string{"stale: completed execution reseed"}},
			}
			trace := []string{}
			repository := &repositoryFake{record: record, trace: &trace}
			environment := &environmentFake{trace: &trace, canonical: true, head: test.newHead, report: "/worktree/new-report.json"}
			service := NewService(repository, environment, fixedClock{fixedCompletionTime}, tracedLiveInspector(&trace), nil)
			request := validRequest()
			request.Generation = test.generation
			request.FinalHead = test.newHead
			request.TuringReportPath = "/worktree/new-report.json"
			request.Verification = []string{"new verification"}
			request.RemoteArtifactURL = "https://github.com/acme/repo/pull/304"

			if _, err := service.Complete(context.Background(), request); err != nil {
				t.Fatalf("complete reopened generation: %v", err)
			}
			if repository.record.Completion == nil || repository.record.Completion.Generation != test.generation || repository.record.Completion.FinalHead != test.newHead || repository.record.Lease.Status != "released" || repository.record.Phase != "done" {
				t.Fatalf("new completion=%+v", repository.record)
			}
			if len(repository.record.Ledger["pr"].Notes) != 0 || len(repository.record.Ledger["done"].Notes) != 0 {
				t.Fatalf("completed-reseed stale notes remain: %+v", repository.record.Ledger)
			}
			if _, err := service.Complete(context.Background(), request); err != nil {
				t.Fatalf("identical completion retry: %v", err)
			}
			different := request
			different.Verification = []string{"different verification"}
			if _, err := service.Complete(context.Background(), different); err == nil || err.Error() != "execution completion already exists with different evidence" {
				t.Fatalf("different completion retry error=%v", err)
			}
			if repository.commits != 1 {
				t.Fatalf("completion commits=%d want=1", repository.commits)
			}
		})
	}
}

type repositoryFake struct {
	record    completioncontract.RecordSnapshot
	trace     *[]string
	commits   int
	commitErr error
}

func (f *repositoryFake) Update(_ context.Context, _ string, transition RecordTransition) (RepositoryResult, error) {
	*f.trace = append(*f.trace, "read")
	next, changed, err := transition(f.record.Clone())
	if err != nil {
		return RepositoryResult{}, err
	}
	if changed {
		*f.trace = append(*f.trace, "commit")
		if f.commitErr != nil {
			return RepositoryResult{}, f.commitErr
		}
		f.record = next.Clone()
		f.commits++
	}
	return RepositoryResult{Record: f.record.Clone(), Execution: stableExecution(f.record)}, nil
}

type environmentFake struct {
	trace     *[]string
	canonical bool
	head      string
	report    string
	artifact  error
}

func (f *environmentFake) VerifyArtifact(_ completioncontract.RecordSnapshot, _ string) error {
	*f.trace = append(*f.trace, "artifact")
	return f.artifact
}
func (f *environmentFake) PathsMatch(_, _ string) bool {
	*f.trace = append(*f.trace, "cwd")
	return f.canonical
}
func (f *environmentFake) CurrentHead(context.Context, string) (string, error) {
	*f.trace = append(*f.trace, "head")
	return f.head, nil
}
func (f *environmentFake) VerifyReport(_, _ string) (string, error) {
	*f.trace = append(*f.trace, "report")
	return f.report, nil
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type settlerFake struct {
	trace *[]string
	err   error
}

func (f *settlerFake) Settle(_ context.Context, runID, taskID string) error {
	*f.trace = append(*f.trace, "settle:"+runID+":"+taskID)
	return f.err
}

func tracedLiveInspector(trace *[]string) ProcessInspector {
	return func(_ context.Context, receipt completioncontract.ProcessReceipt) (string, completioncontract.ProcessReceipt, error) {
		*trace = append(*trace, "process")
		return "live", receipt, nil
	}
}

func validRequest() Request {
	receipt := completioncontract.ProcessReceipt{PID: 198, StartedAt: "2026-08-02T00:00:00Z", Executable: "/bin/codex"}
	return Request{ID: "io-198", Generation: 1, Actor: completioncontract.Actor{Host: "codex", SessionID: "session-198", Process: &receipt}, Ancestry: []completioncontract.ProcessReceipt{receipt}, CWD: "/worktree", FinalHead: strings.Repeat("a", 40), TuringReportPath: "/worktree/report.json", Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/acme/repo/pull/198", Confirm: true}
}

func activeCompletionRecord(mode string) completioncontract.RecordSnapshot {
	receipt := completioncontract.ProcessReceipt{PID: 198, StartedAt: "2026-08-02T00:00:00Z", Executable: "/bin/codex"}
	record := completioncontract.RecordSnapshot{
		ID: "io-198", Prepared: true, Phase: "pr", CanonicalRoot: "/worktree", Mode: mode,
		Lease:    completioncontract.Lease{Generation: 1, Status: "active", Holder: &completioncontract.Actor{Host: "codex", SessionID: "session-198", Process: &receipt}, ClaimedAt: "2026-08-02T00:00:01Z"},
		Artifact: &completioncontract.RemoteArtifact{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/198", Labels: []string{"enhancement"}, Assignees: []string{"m16khb"}, VerifiedAt: "2026-08-02T00:00:02Z", TargetBranch: "main"}, BaseBranch: "main",
		Ledger: map[string]completioncontract.LedgerEntry{"pr": {Phase: "pr", EnteredAt: "2026-08-02T00:00:02Z"}},
	}
	if mode == "orca" {
		record.Orca = &completioncontract.OrcaBinding{RunID: "run-198", TaskID: "task-198"}
	}
	return record
}

func stableExecution(record completioncontract.RecordSnapshot) leasecontract.Execution {
	return leasecontract.Execution{Mode: record.Mode, Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: record.CanonicalRoot, Branch: "198", BaseHead: strings.Repeat("b", 40), Driver: map[bool]string{true: "orca", false: "git"}[record.Mode == "orca"], LinkedAt: "2026-08-02T00:00:00Z"}, Lease: leasecontract.Lease{Generation: record.Lease.Generation, Status: record.Lease.Status}}
}
