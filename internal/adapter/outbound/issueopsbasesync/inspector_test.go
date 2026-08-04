package issueopsbasesync

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

const (
	baseOID = "1111111111111111111111111111111111111111"
	workOID = "2222222222222222222222222222222222222222"
)

type gitCall struct {
	dir  string
	args []string
}

type fakeGitRunner struct {
	calls        []gitCall
	fetchCode    int
	fetchErr     error
	baseOID      string
	workOID      string
	ancestorCode int
}

func (r *fakeGitRunner) run(_ context.Context, dir string, args ...string) (int, string, string, error) {
	r.calls = append(r.calls, gitCall{dir: dir, args: append([]string(nil), args...)})
	if r.fetchErr != nil {
		return 0, "", "", r.fetchErr
	}
	switch args[0] {
	case "fetch":
		return r.fetchCode, "", "fetch failed", nil
	case "rev-parse":
		if args[1] == "FETCH_HEAD" {
			return 0, r.baseOID, "", nil
		}
		return 0, r.workOID, "", nil
	case "merge-base":
		return r.ancestorCode, "", "not an ancestor", nil
	default:
		return 127, "", "unexpected command", nil
	}
}

func TestInspectorObservesNoDriftWithExactReadOnlyCommands(t *testing.T) {
	runner := &fakeGitRunner{baseOID: baseOID, workOID: workOID}
	receipt, err := NewInspector(runner.run).Observe(context.Background(), basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt != (basesyncport.Receipt{BaseOID: baseOID, WorkOID: workOID, SyncRequired: false}) {
		t.Fatalf("receipt=%+v", receipt)
	}
	want := []gitCall{
		{dir: "/worktree", args: []string{"fetch", "--quiet", "origin", "main"}},
		{dir: "/worktree", args: []string{"rev-parse", "FETCH_HEAD"}},
		{dir: "/worktree", args: []string{"rev-parse", "HEAD"}},
		{dir: "/worktree", args: []string{"merge-base", "--is-ancestor", baseOID, workOID}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("git calls=%v want=%v", runner.calls, want)
	}
}

func TestInspectorReportsParentDrift(t *testing.T) {
	runner := &fakeGitRunner{baseOID: baseOID, workOID: workOID, ancestorCode: 1}
	receipt, err := NewInspector(runner.run).Observe(context.Background(), basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.SyncRequired || receipt.BaseOID != baseOID || receipt.WorkOID != workOID {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestInspectorFailsClosedWhenObservationIsUnavailable(t *testing.T) {
	tests := []struct {
		name    string
		request basesyncport.Request
		runner  *fakeGitRunner
		want    string
	}{
		{name: "missing base branch", request: basesyncport.Request{Worktree: "/worktree"}, runner: &fakeGitRunner{}, want: "base branch is required"},
		{name: "unreadable remote", request: basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"}, runner: &fakeGitRunner{fetchCode: 1}, want: "fetch base branch"},
		{name: "missing fetched oid", request: basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"}, runner: &fakeGitRunner{workOID: workOID}, want: "FETCH_HEAD"},
		{name: "command timeout", request: basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"}, runner: &fakeGitRunner{fetchErr: context.DeadlineExceeded}, want: "deadline exceeded"},
		{name: "unexpected ancestor failure", request: basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"}, runner: &fakeGitRunner{baseOID: baseOID, workOID: workOID, ancestorCode: 2}, want: "observe base ancestry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInspector(test.runner.run).Observe(context.Background(), test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want substring %q", err, test.want)
			}
			if test.name == "missing base branch" && len(test.runner.calls) != 0 {
				t.Fatalf("missing base branch ran git: %v", test.runner.calls)
			}
		})
	}
}

func TestInspectorRequiresRunner(t *testing.T) {
	_, err := NewInspector(nil).Observe(context.Background(), basesyncport.Request{Worktree: "/worktree", BaseBranch: "main"})
	if !errors.Is(err, ErrGitRunnerRequired) {
		t.Fatalf("error=%v", err)
	}
}
