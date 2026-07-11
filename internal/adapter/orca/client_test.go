package orca

import (
	"context"
	"errors"
	"fmt"
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
	addCompleteProbeLeafHelp(runner)

	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.RuntimeID != "runtime-1" || result.RepoID != "repo-1" || result.RepoRemoteName != "origin" || result.WorktreeBasePath != "../repo.worktrees" {
		t.Fatalf("probe result = %#v", result)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "version") || (strings.Contains(joined, " create") && !strings.HasSuffix(joined, " --help")) || (strings.Contains(joined, " dispatch ") && !strings.HasSuffix(joined, " --help")) {
			t.Fatalf("probe used forbidden command: %s", joined)
		}
	}
}

func TestProbeRequiresInstalledCodexHookTrustBypassFlag(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["codex --help"] = CommandOutput{Stdout: []byte("Usage: codex")}
	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Code != "codex_hook_trust_bypass_unsupported" {
		t.Fatalf("missing installed bypass flag probe = %#v", result)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, " create ") && !strings.HasSuffix(joined, " --help") {
			t.Fatalf("Codex capability probe mutated state: %s", joined)
		}
	}
}

func TestProbeDoesNotApplyCodexBypassRequirementToClaudeOrGJC(t *testing.T) {
	for _, agent := range []string{"claude", "gjc"} {
		t.Run(agent, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths[agent] = "/usr/local/bin/" + agent
			runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
			runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
			addCompleteProbeLeafHelp(runner)
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: agent})
			if err != nil || !result.Ready {
				t.Fatalf("%s probe changed: result=%#v err=%v", agent, result, err)
			}
			for _, call := range runner.calls {
				if reflect.DeepEqual(call, []string{"codex", "--help"}) {
					t.Fatalf("%s probe invoked the Codex-only capability check", agent)
				}
			}
		})
	}
}

func TestProbeRequiresCanonicalWorktreeBaseBeforeMutation(t *testing.T) {
	for _, tt := range []struct {
		name, base, code string
	}{
		{name: "missing", code: "worktree_base_unresolved"},
		{name: "mismatch", base: "../nested-workspaces", code: "worktree_base_mismatch"},
		{name: "matching", base: "../repo.worktrees"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths["codex"] = "/usr/local/bin/codex"
			runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
			runner.responses["orca repo show --repo path:/repo --json"] = CommandOutput{Stdout: []byte(fmt.Sprintf(`{"ok":true,"result":{"repo":{"id":"repo-1","path":"/repo","displayName":"repo","worktreeBasePath":%q,"gitRemoteIdentity":{"remoteName":"origin"}}}}`, tt.base))}
			addCompleteProbeLeafHelp(runner)
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			if tt.code == "" {
				if !result.Ready {
					t.Fatalf("matching base probe = %#v", result)
				}
			} else if result.Ready || result.Code != tt.code || len(runner.calls) != 2 {
				t.Fatalf("base probe = %#v calls=%#v", result, runner.calls)
			}
		})
	}
}

func TestProbeRequiresEveryInvokedLeafFlagBeforeMutation(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca worktree create --help"] = CommandOutput{Stdout: []byte("--repo --name --base-branch --no-parent --setup --comment --json")}

	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Code != "capability_missing" {
		t.Fatalf("missing leaf flag probe = %#v", result)
	}
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, " create ") && !strings.HasSuffix(joined, " --help") {
			t.Fatalf("probe mutated external state: %s", joined)
		}
	}
}

func TestProbeRequiresTaskUpdateLeafFlagsForCleanup(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca orchestration task-update --help"] = CommandOutput{Stdout: []byte("--id --status --json")}
	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Code != "capability_missing" {
		t.Fatalf("task-update missing --result must fail pre-mutation probe: %#v", result)
	}
}

func TestProbeRequiresLiveCompleteOrchestrationReadinessBeforeMutation(t *testing.T) {
	tests := []struct {
		name   string
		output CommandOutput
	}{
		{name: "rejected", output: CommandOutput{Stdout: []byte(`{"ok":false,"error":{"code":"experimental_disabled","message":"enable orchestration"}}`)}},
		{name: "invalid", output: CommandOutput{Stdout: []byte(`{"ok":`)}},
		{name: "missing count", output: CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[]}}`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths["codex"] = "/usr/local/bin/codex"
			runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
			runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
			addCompleteProbeLeafHelp(runner)
			runner.responses["orca orchestration task-list --json"] = tt.output
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready || result.Code != "orchestration_unready" || result.Detail == "" {
				t.Fatalf("orchestration readiness = %#v", result)
			}
			for _, call := range runner.calls {
				joined := strings.Join(call, " ")
				if strings.Contains(joined, " create ") && !strings.HasSuffix(joined, " --help") {
					t.Fatalf("readiness probe mutated state: %s", joined)
				}
			}
		})
	}
}

func TestProbeRequiresCompleteRuntimeAndRepoIdentity(t *testing.T) {
	tests := []struct {
		name, status, repo, code string
	}{
		{name: "runtime id", status: `{"ok":true,"result":{"runtime":{"reachable":true,"state":"ready"},"graph":{"state":"ready"}}}`, repo: `{"ok":true,"result":{"repo":{"id":"repo-1","path":"/repo","worktreeBasePath":"../repo.worktrees","gitRemoteIdentity":{"remoteName":"origin"}}}}`, code: "runtime_id_unresolved"},
		{name: "graph state", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":true,"state":"ready"},"graph":{}}}`, repo: `{"ok":true,"result":{"repo":{"id":"repo-1","path":"/repo","worktreeBasePath":"../repo.worktrees","gitRemoteIdentity":{"remoteName":"origin"}}}}`, code: "graph_not_ready"},
		{name: "repo id", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":true,"state":"ready"},"graph":{"state":"ready"}}}`, repo: `{"ok":true,"result":{"repo":{"path":"/repo","worktreeBasePath":"../repo.worktrees","gitRemoteIdentity":{"remoteName":"origin"}}}}`, code: "repo_identity_unresolved"},
		{name: "repo path", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":true,"state":"ready"},"graph":{"state":"ready"}}}`, repo: `{"ok":true,"result":{"repo":{"id":"repo-1","worktreeBasePath":"../repo.worktrees","gitRemoteIdentity":{"remoteName":"origin"}}}}`, code: "repo_identity_unresolved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths["codex"] = "/usr/local/bin/codex"
			runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(tt.status)}
			runner.responses["orca repo show --repo path:/repo --json"] = CommandOutput{Stdout: []byte(tt.repo)}
			addCompleteProbeLeafHelp(runner)
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready || result.Code != tt.code {
				t.Fatalf("identity probe = %#v, want %s", result, tt.code)
			}
		})
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
		{name: "unreachable", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":false,"state":"ready"},"graph":{"state":"ready"}}}`, code: "runtime_unreachable"},
		{name: "runtime", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":true,"state":"starting"},"graph":{"state":"ready"}}}`, code: "runtime_not_ready"},
		{name: "graph", status: `{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1","reachable":true,"state":"ready"},"graph":{"state":"loading"}}}`, code: "graph_not_ready"},
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
	runner.responses["orca terminal list --worktree id:worktree-1 --limit 512 --json"] = fixtureOutput(t, "terminal_list.json")
	terminal, err := NewClient(runner).RefreshTerminal(context.Background(), "worktree-1", "pty-2")
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Handle != "term-live" || terminal.PTYID != "pty-2" {
		t.Fatalf("refreshed terminal = %#v", terminal)
	}
}

func TestClientCreateTerminalAcceptsRuntimeIdentityWithoutPTY(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --worktree id:worktree-1 --command codex --title marker --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {
			"terminal": {
				"handle": "term-create",
				"worktreeId": "worktree-1",
				"title": "marker",
				"surface": "terminal"
			}
		}
	}`)}
	runner.responses["orca terminal list --worktree id:worktree-1 --limit 512 --json"] = fixtureOutput(t, "terminal_list.json")

	terminal, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
		WorktreeID: "worktree-1", Agent: "codex", Title: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Handle != "term-create" || terminal.PTYID != "" || terminal.WorktreeID != "worktree-1" {
		t.Fatalf("created terminal identity = %#v", terminal)
	}
	want := [][]string{
		{"orca", "terminal", "create", "--worktree", "id:worktree-1", "--command", "codex", "--title", "marker", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestClientCreateTerminalUsesCodexBypassOnlyWhenAttested(t *testing.T) {
	tests := []struct {
		name, agent, command string
		attested             bool
	}{
		{name: "attested Codex", agent: "codex", command: "codex --dangerously-bypass-hook-trust", attested: true},
		{name: "ordinary Codex", agent: "codex", command: "codex"},
		{name: "Claude unchanged", agent: "claude", command: "claude", attested: true},
		{name: "GJC unchanged", agent: "gjc", command: "gjc", attested: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			key := "orca terminal create --worktree id:worktree-1 --command " + tt.command + " --json"
			runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-create","worktreeId":"worktree-1"}}}`)}
			_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
				WorktreeID: "worktree-1", Agent: tt.agent, AllowCodexHookTrustBypass: tt.attested,
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"orca", "terminal", "create", "--worktree", "id:worktree-1", "--command", tt.command, "--json"}
			if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], want) {
				t.Fatalf("terminal launch = %#v, want %#v", runner.calls, want)
			}
		})
	}
}

func TestClientCreateTerminalRejectsIncompleteRuntimeIdentity(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --worktree id:worktree-1 --command codex --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {"terminal": {"ptyId": "pty-2", "worktreeId": "worktree-1"}}
	}`)}

	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "worktree-1", Agent: "codex"})
	if err == nil || !strings.Contains(err.Error(), "terminal identity") {
		t.Fatalf("CreateTerminal() error = %v, want terminal identity error", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("incomplete create identity made an extra call: %#v", runner.calls)
	}
}

func TestClientCreateTaskDecodesOfficialSnakeCaseShape(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration task-create --spec spec --task-title agent-harness marker --display-name 16-demo --json"] = fixtureOutput(t, "task_create.json")
	got, err := NewClient(runner).CreateTask(context.Background(), port.OrcaCreateTaskRequest{Spec: "spec", Title: "agent-harness marker", DisplayName: "16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "task-1" || got.Title != "agent-harness marker" || got.DisplayName != "16-demo" || got.Status != "ready" {
		t.Fatalf("official task projection = %#v", got)
	}
}

func TestClientListTasksUsesInstalledCountContract(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration task-list --json"] = fixtureOutput(t, "task_list.json")
	got, err := NewClient(runner).ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "task-1" || got[0].Title != "agent-harness marker" || got[0].Status != "ready" {
		t.Fatalf("task list projection = %#v", got)
	}
}

func TestClientRejectsIncompleteExternalLists(t *testing.T) {
	for _, tt := range []struct {
		name, command, field string
		call                 func(*Client) error
	}{
		{name: "worktree truncated", command: "orca worktree list --repo path:/repo --limit 512 --json", field: "worktrees", call: func(c *Client) error { _, err := c.ListWorktrees(context.Background(), "/repo"); return err }},
		{name: "terminal total mismatch", command: "orca terminal list --worktree id:wt-1 --limit 512 --json", field: "terminals", call: func(c *Client) error { _, err := c.ListTerminals(context.Background(), "wt-1"); return err }},
		{name: "task missing metadata", command: "orca orchestration task-list --json", field: "tasks", call: func(c *Client) error { _, err := c.ListTasks(context.Background()); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			body := `{"ok":true,"result":{"` + tt.field + `":[]}}`
			if strings.Contains(tt.name, "truncated") {
				body = `{"ok":true,"result":{"` + tt.field + `":[],"totalCount":0,"truncated":true}}`
			} else if strings.Contains(tt.name, "mismatch") {
				body = `{"ok":true,"result":{"` + tt.field + `":[],"totalCount":1,"truncated":false}}`
			}
			runner.responses[tt.command] = CommandOutput{Stdout: []byte(body)}
			if err := tt.call(NewClient(runner)); err == nil || (!strings.Contains(err.Error(), "incomplete") && !strings.Contains(err.Error(), "metadata")) {
				t.Fatalf("incomplete list error = %v", err)
			}
		})
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

func addCompleteProbeLeafHelp(runner *fakeRunner) {
	for command, flags := range map[string]string{
		"orca worktree create --help":             "--repo --name --base-branch --no-parent --setup --comment --issue --json",
		"orca worktree list --help":               "--repo --limit --json",
		"orca terminal create --help":             "--worktree --command --title --json",
		"orca terminal list --help":               "--worktree --limit --json",
		"orca orchestration task-create --help":   "--spec --task-title --display-name --json",
		"orca orchestration task-list --help":     "--json",
		"orca orchestration task-update --help":   "--id --status --result --json",
		"orca orchestration dispatch --help":      "--task --to --from --inject --return-preamble --json",
		"orca orchestration dispatch-show --help": "--task --json",
		"orca worktree rm --help":                 "--worktree --force --json",
	} {
		runner.responses[command] = CommandOutput{Stdout: []byte(flags)}
	}
	runner.responses["orca orchestration task-list --json"] = fixtureOutput(runner.t, "task_list.json")
	runner.responses["codex --help"] = CommandOutput{Stdout: []byte("--dangerously-bypass-hook-trust")}
}
