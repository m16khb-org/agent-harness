package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestProbeDoesNotUseVersionOrMutate(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	runner.responses["orca worktree --help"] = CommandOutput{Stdout: []byte("list create rm")}
	runner.responses["orca terminal --help"] = CommandOutput{Stdout: []byte("list create send")}
	runner.responses["orca orchestration --help"] = CommandOutput{Stdout: []byte("task-list task-create task-update dispatch dispatch-show send check")}

	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.RuntimeID != "runtime-1" || result.RepoID != "repo-1" || result.RepoRemoteName != "origin" {
		t.Fatalf("probe result = %#v", result)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "version") || strings.Contains(joined, " create") || strings.Contains(joined, " dispatch ") {
			t.Fatalf("probe used forbidden command: %s", joined)
		}
	}
}

func TestProbeRequiresRepoRemoteNameBeforeMutation(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"repo":{"id":"repo-1","path":"/repo","displayName":"repo"}}}`)}

	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Code != "repo_remote_unresolved" {
		t.Fatalf("probe result = %#v", result)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("remote-name failure continued probing: %#v", runner.calls)
	}
}

func TestProbeRequiresReachableReadyRuntimeAndGraph(t *testing.T) {
	tests := []struct {
		name   string
		status string
		code   string
	}{
		{name: "unreachable", status: `{"ok":true,"result":{"runtime":{"reachable":false,"state":"ready"},"graph":{"state":"ready"}}}`, code: "runtime_unreachable"},
		{name: "runtime", status: `{"ok":true,"result":{"runtime":{"reachable":true,"state":"starting"},"graph":{"state":"ready"}}}`, code: "runtime_not_ready"},
		{name: "graph", status: `{"ok":true,"result":{"runtime":{"reachable":true,"state":"ready"},"graph":{"state":"loading"}}}`, code: "graph_not_ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths["codex"] = "/usr/local/bin/codex"
			runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(tt.status)}
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready || result.Code != tt.code {
				t.Fatalf("probe result = %#v, want code %s", result, tt.code)
			}
			if len(runner.calls) != 1 {
				t.Fatalf("failed status probe continued with calls: %#v", runner.calls)
			}
		})
	}
}

func TestClientDecodesStdoutWithHandshakeNoiseOnStderr(t *testing.T) {
	runner := newFakeRunner(t)
	output := fixtureOutput(t, "status_ready.json")
	output.Stderr = []byte("[relay-connect] Handshake OK\n")
	runner.responses["orca status --json"] = output
	status, err := NewClient(runner).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.RuntimeID != "runtime-1" || !status.RuntimeReachable {
		t.Fatalf("decoded status = %#v", status)
	}
}

func TestClientRejectsMalformedOrOversizedEnvelope(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(`{"ok":`)}
		_, err := NewClient(runner).Status(context.Background())
		if err == nil || !strings.Contains(err.Error(), "decode") {
			t.Fatalf("Status() error = %v, want decode error", err)
		}
	})
	t.Run("oversized", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(strings.Repeat("x", MaxEnvelopeBytes+1))}
		_, err := NewClient(runner).Status(context.Background())
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("Status() error = %v, want size error", err)
		}
	})
}

func TestClientBuildsSpikeVerifiedArgvWithoutShell(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --no-parent --setup skip --comment agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1 --issue 16 --json"] = fixtureOutput(t, "worktree_create.json")
	client := NewClient(runner)
	result, err := client.CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "16-demo", BaseBranch: "refs/remotes/origin/16-demo", Issue: 16, Comment: "agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "worktree-1" || result.InstanceID != "instance-1" {
		t.Fatalf("created worktree = %#v", result)
	}
	want := []string{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "16-demo", "--base-branch", "refs/remotes/origin/16-demo", "--no-parent", "--setup", "skip", "--comment", "agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1", "--issue", "16", "--json"}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("argv = %#v, want %#v", runner.calls, want)
	}
}

func TestClientRefreshesTerminalHandleByWorktreeAndPTY(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal list --worktree id:worktree-1 --json"] = fixtureOutput(t, "terminal_list.json")
	terminal, err := NewClient(runner).RefreshTerminal(context.Background(), "worktree-1", "pty-2")
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Handle != "term-live" || terminal.PTYID != "pty-2" {
		t.Fatalf("refreshed terminal = %#v", terminal)
	}
}

type fakeRunner struct {
	t         *testing.T
	lookPaths map[string]string
	responses map[string]CommandOutput
	errors    map[string]error
	calls     [][]string
}

func newFakeRunner(t *testing.T) *fakeRunner {
	return &fakeRunner{t: t, lookPaths: map[string]string{}, responses: map[string]CommandOutput{}, errors: map[string]error{}}
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if path := f.lookPaths[file]; path != "" {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ time.Duration, argv []string) (CommandOutput, error) {
	copyArgv := append([]string(nil), argv...)
	f.calls = append(f.calls, copyArgv)
	key := strings.Join(argv, " ")
	if err := f.errors[key]; err != nil {
		return CommandOutput{}, err
	}
	output, ok := f.responses[key]
	if !ok {
		f.t.Fatalf("unexpected command: %s", key)
	}
	return output, nil
}

func fixtureOutput(t *testing.T, name string) CommandOutput {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return CommandOutput{Stdout: raw}
}
