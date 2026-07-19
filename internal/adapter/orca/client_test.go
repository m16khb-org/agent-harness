package orca

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestClientProjectsWorkerDoneThroughDedicatedSafeArgvMethod(t *testing.T) {
	runner := newFakeRunner(t)
	req := port.OrcaWorkerDoneRequest{
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed issue io-demo",
		Body:   "Implementation completed at abcdef. Verification evidence is persisted. The coordinator may inspect the submitted record.",
		TaskID: "task-1", DispatchID: "dispatch-1", ChangedFiles: []string{"a.go", "report.md"}, ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--to", req.ToHandle, "--from", req.FromHandle, "--type", "worker_done", "--subject", req.Subject, "--body", req.Body, "--task-id", req.TaskID, "--dispatch-id", req.DispatchID, "--files-modified", "a.go,report.md", "--report-path", req.ReportPath, "--json"}
	runner.responses[strings.Join(argv, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"message":{"id":"msg-1","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed issue io-demo","body":"Implementation completed at abcdef. Verification evidence is persisted. The coordinator may inspect the submitted record.","payload":"{\"taskId\":\"task-1\",\"dispatchId\":\"dispatch-1\",\"filesModified\":[\"a.go\",\"report.md\"],\"reportPath\":\"/repo/report.md\"}","sequence":42}}}`)}
	got, err := NewClient(runner).SendWorkerDone(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.MessageID != "msg-1" || got.Sequence != 42 || len(runner.calls) != 1 || !slices.Equal(runner.calls[0], argv) {
		t.Fatalf("worker_done projection = %#v calls=%#v", got, runner.calls)
	}
}

func TestClientProjectsNoChangeWorkerDoneWithoutFilesModifiedFlag(t *testing.T) {
	runner := newFakeRunner(t)
	req := port.OrcaWorkerDoneRequest{
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed no-change issue io-demo",
		Body:   "Verification evidence is persisted and no files changed.",
		TaskID: "task-1", DispatchID: "dispatch-1", ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--to", req.ToHandle, "--from", req.FromHandle, "--type", "worker_done", "--subject", req.Subject, "--body", req.Body, "--task-id", req.TaskID, "--dispatch-id", req.DispatchID, "--report-path", req.ReportPath, "--json"}
	runner.responses[strings.Join(argv, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"message":{"id":"msg-clean","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed no-change issue io-demo","body":"Verification evidence is persisted and no files changed.","payload":"{\"taskId\":\"task-1\",\"dispatchId\":\"dispatch-1\",\"filesModified\":[],\"reportPath\":\"/repo/report.md\"}","sequence":43}}}`)}

	got, err := NewClient(runner).SendWorkerDone(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.MessageID != "msg-clean" || got.Sequence != 43 || len(runner.calls) != 1 || !slices.Equal(runner.calls[0], argv) {
		t.Fatalf("no-change worker_done projection = %#v calls=%#v", got, runner.calls)
	}
}

func TestClientWorkerDonePreconditionAndAmbiguityCallCounts(t *testing.T) {
	valid := port.OrcaWorkerDoneRequest{
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed issue io-demo",
		Body:   "Implementation completed. Verification evidence persisted. Coordinator inspection is ready.",
		TaskID: "task-1", DispatchID: "dispatch-1", ChangedFiles: []string{"a.go"}, ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--to", valid.ToHandle, "--from", valid.FromHandle, "--type", "worker_done", "--subject", valid.Subject, "--body", valid.Body, "--task-id", valid.TaskID, "--dispatch-id", valid.DispatchID, "--files-modified", "a.go", "--report-path", valid.ReportPath, "--json"}
	command := strings.Join(argv, " ")

	t.Run("invalid precondition", func(t *testing.T) {
		runner := newFakeRunner(t)
		invalid := valid
		invalid.ToHandle = "@all"
		_, err := NewClient(runner).SendWorkerDone(context.Background(), invalid)
		var orcaErr *port.OrcaError
		if !errors.As(err, &orcaErr) || orcaErr.Code != "worker_done_invalid" || orcaErr.Invoked || len(runner.calls) != 0 {
			t.Fatalf("invalid precondition = %v calls=%#v", err, runner.calls)
		}
	})

	t.Run("timeout after invocation", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.errors[command] = &port.OrcaError{Code: "command_timeout", Detail: "timed out", Invoked: true, Timeout: true}
		_, err := NewClient(runner).SendWorkerDone(context.Background(), valid)
		var orcaErr *port.OrcaError
		if !errors.As(err, &orcaErr) || orcaErr.Code != "command_timeout" || !orcaErr.Invoked || !orcaErr.Timeout || len(runner.calls) != 1 {
			t.Fatalf("timeout evidence = %v calls=%#v", err, runner.calls)
		}
	})

	for _, tt := range []struct {
		name, stdout, code string
	}{
		{name: "malformed", stdout: `not-json secret=top-secret`, code: "worker_done_response_malformed"},
		{name: "mismatch", stdout: `{"ok":true,"result":{"message":{"id":"msg-1","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed issue io-demo","body":"Implementation completed. Verification evidence persisted. Coordinator inspection is ready.","payload":"{\"taskId\":\"task-other\",\"dispatchId\":\"dispatch-1\",\"filesModified\":[\"a.go\"],\"reportPath\":\"/repo/report.md\"}","sequence":42}}}`, code: "worker_done_response_mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses[command] = CommandOutput{Invoked: true, Stdout: []byte(tt.stdout)}
			_, err := NewClient(runner).SendWorkerDone(context.Background(), valid)
			var orcaErr *port.OrcaError
			if !errors.As(err, &orcaErr) || orcaErr.Code != tt.code || !orcaErr.Invoked || len(runner.calls) != 1 || len(orcaErr.Detail) > 1027 || strings.Contains(orcaErr.Detail, "top-secret") {
				t.Fatalf("%s evidence = %#v calls=%#v", tt.name, orcaErr, runner.calls)
			}
		})
	}
}

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

func TestProbeRequiresGitHubIssueFlagOnlyForGitHubProvider(t *testing.T) {
	for _, tt := range []struct {
		provider string
		ready    bool
	}{
		{provider: "github", ready: false},
		{provider: "gitlab", ready: true},
	} {
		t.Run(tt.provider, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths["codex"] = "/usr/local/bin/codex"
			runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
			runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
			addCompleteProbeLeafHelp(runner)
			runner.responses["orca worktree create --help"] = CommandOutput{Stdout: []byte("--repo --name --base-branch --no-parent --setup --comment --json")}
			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex", Provider: tt.provider})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready != tt.ready || result.Provider != tt.provider {
				t.Fatalf("%s provider-aware probe = %#v", tt.provider, result)
			}
			if !tt.ready && result.Code != "capability_missing" {
				t.Fatalf("GitHub probe without --issue code = %q", result.Code)
			}
		})
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
			runner.responses["orca orchestration task-list --ready --json"] = tt.output
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

func TestClientAdoptsExistingGitHubWorktreeWithIssueAndMarker(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca worktree set --worktree id:worktree-1 --comment agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1 --issue 16 --json"
	runner.responses[command] = fixtureOutput(t, "worktree_create.json")
	got, err := NewClient(runner).AdoptWorktree(context.Background(), port.OrcaAdoptWorktreeRequest{
		WorktreeID: "worktree-1", Provider: "github", Issue: 16, Comment: "agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "worktree-1" || got.InstanceID != "instance-1" || len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
		t.Fatalf("adopted worktree = %#v calls=%#v", got, runner.calls)
	}
}

func TestClientShowsExistingWorktreeByExactPath(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca worktree show --worktree path:/repo.worktrees/16-demo --json"
	runner.responses[command] = fixtureOutput(t, "worktree_create.json")
	got, err := NewClient(runner).ShowWorktree(context.Background(), "/repo.worktrees/16-demo")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "worktree-1" || got.InstanceID != "instance-1" || len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
		t.Fatalf("shown worktree = %#v calls=%#v", got, runner.calls)
	}
}

func TestClientCreateWorktreeUsesProviderSpecificIssueMetadata(t *testing.T) {
	for _, tt := range []struct {
		name, provider, command, output string
		wantGitLabIssue                 int
		wantGitLabPresent               bool
	}{
		{
			name: "github", provider: "github",
			command: "orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --no-parent --setup skip --comment marker --issue 16 --json",
			output:  `{"ok":true,"result":{"worktree":{"id":"wt-1","instanceId":"inst-1","repoId":"repo-1","path":"/repo.worktrees/16-demo","linkedIssue":16,"linkedGitLabIssue":null}},"_meta":{"runtimeId":"runtime-1"}}`,
		},
		{
			name: "gitlab null native metadata", provider: "gitlab",
			command: "orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --no-parent --setup skip --comment marker --json",
			output:  `{"ok":true,"result":{"worktree":{"id":"wt-1","instanceId":"inst-1","repoId":"repo-1","path":"/repo.worktrees/16-demo","linkedIssue":null,"linkedGitLabIssue":null}},"_meta":{"runtimeId":"runtime-1"}}`,
		},
		{
			name: "gitlab exact native metadata", provider: "gitlab",
			command:         "orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --no-parent --setup skip --comment marker --json",
			output:          `{"ok":true,"result":{"worktree":{"id":"wt-1","instanceId":"inst-1","repoId":"repo-1","path":"/repo.worktrees/16-demo","linkedIssue":null,"linkedGitLabIssue":16}},"_meta":{"runtimeId":"runtime-1"}}`,
			wantGitLabIssue: 16, wantGitLabPresent: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses[tt.command] = CommandOutput{Stdout: []byte(tt.output)}
			got, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
				Repo: "/repo", Name: "16-demo", BaseBranch: "refs/remotes/origin/16-demo", Provider: tt.provider, Issue: 16, Comment: "marker",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != tt.command {
				t.Fatalf("%s create argv = %#v", tt.provider, runner.calls)
			}
			if (got.GitLabIssue != nil) != tt.wantGitLabPresent || got.GitLabIssue != nil && *got.GitLabIssue != tt.wantGitLabIssue {
				t.Fatalf("%s linked GitLab metadata = %#v", tt.provider, got.GitLabIssue)
			}
		})
	}
}

func TestClientRefreshesTerminalHandleByWorktreeAndPTY(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal list --worktree id:worktree-1 --limit 512 --json"] = fixtureOutput(t, "terminal_list.json")
	terminal, err := NewClient(runner).RefreshTerminal(context.Background(), "worktree-1", "pty-2")
	if err != nil {
		t.Fatal(err)
	}
	if terminal.RuntimeID != "runtime-1" || terminal.Handle != "term-live" || terminal.PTYID != "pty-2" || terminal.TabID != "tab-live" || terminal.LeafID != "leaf-live" || terminal.Title != "agent-harness issueops=io-demo ownership=epoch-1 attempt=1" {
		t.Fatalf("refreshed terminal = %#v", terminal)
	}
}

func TestClientListsCompleteGlobalTerminalInventoryWithoutWorktreeSelector(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal list --limit 512 --json"] = fixtureOutput(t, "terminal_list.json")
	terminals, err := NewClient(runner).ListTerminals(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(terminals) != 2 || terminals[0].Handle != "term-old" || terminals[1].Handle != "term-live" {
		t.Fatalf("global terminal inventory = %#v", terminals)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != "orca terminal list --limit 512 --json" {
		t.Fatalf("global terminal inventory argv = %#v", runner.calls)
	}
}

func TestClientDecodesRuntimeRolloverStableTerminalAndWorktreeIdentity(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal list --worktree id:worktree-1 --limit 512 --json"] = fixtureOutput(t, "terminal_list_runtime_rollover.json")
	runner.responses["orca worktree list --repo path:/repo --limit 512 --json"] = fixtureOutput(t, "worktree_list_runtime_rollover.json")
	client := NewClient(runner)
	terminals, err := client.ListTerminals(context.Background(), "worktree-1")
	if err != nil {
		t.Fatal(err)
	}
	worktrees, err := client.ListWorktrees(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(terminals) != 1 || terminals[0].RuntimeID != "runtime-2" || terminals[0].TabID != "tab-stable" || terminals[0].LeafID != "leaf-stable" || terminals[0].StableTabTitle != "agent-harness issueops=io-demo ownership=epoch-1 attempt=1" || terminals[0].Title == terminals[0].StableTabTitle {
		t.Fatalf("runtime rollover terminal = %#v", terminals)
	}
	if len(worktrees) != 1 || worktrees[0].RuntimeID != "runtime-2" || worktrees[0].InstanceID != "instance-2" || worktrees[0].Comment != "agent-harness issueops=io-demo ownership=epoch-1 attempt=1" {
		t.Fatalf("runtime rollover worktree = %#v", worktrees)
	}
}

func TestClientNormalizesWorktreeHeadRefBranch(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca worktree list --repo path:/repo --limit 512 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"worktrees":[{"id":"wt-main","instanceId":"instance-main","repoId":"repo-1","path":"/repo","head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","branch":"refs/heads/main"}],"totalCount":1,"truncated":false},"_meta":{"runtimeId":"runtime-1"}}`)}

	worktrees, err := NewClient(runner).ListWorktrees(context.Background(), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 1 || worktrees[0].Branch != "main" {
		t.Fatalf("normalized worktree branch = %#v", worktrees)
	}
}

func TestStableVisualTabTitlesBoundsTotalInventoryAcrossLayouts(t *testing.T) {
	layouts := make([]visualLayoutPayload, 2)
	for i := 0; i < port.OrcaMaxBaselineIDs+1; i++ {
		layout := i % len(layouts)
		layouts[layout].Root.Tabs = append(layouts[layout].Root.Tabs, struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		}{TabID: fmt.Sprintf("tab-%d", i), Title: "marker", ActiveLeafID: fmt.Sprintf("leaf-%d", i)})
	}
	if _, err := stableVisualTabTitles(layouts); err == nil || !strings.Contains(err.Error(), "visual tab inventory exceeds") {
		t.Fatalf("unbounded visual tab inventory error = %v", err)
	}
}

func TestClientCreateTerminalNegotiatesOnlyFixedBuiltInLaunchShape(t *testing.T) {
	for _, tt := range []struct {
		name, help, create string
	}{
		{name: "fixed agent", help: "--worktree --agent --title --json", create: "orca terminal create --worktree id:worktree-1 --agent codex --title marker --json"},
		{name: "fixed command", help: "--worktree --command --title --json", create: "orca terminal create --worktree id:worktree-1 --command codex --dangerously-bypass-hook-trust --title marker --json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte(tt.help)}
			runner.responses[tt.create] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-1","worktreeId":"worktree-1"}},"_meta":{"runtimeId":"runtime-1"}}`)}
			terminal, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
				WorktreeID: "worktree-1", Agent: "codex", Title: "marker", AllowCodexHookTrustBypass: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if terminal.RuntimeID != "runtime-1" || len(runner.calls) != 2 || strings.Join(runner.calls[1], " ") != tt.create {
				t.Fatalf("fixed launch negotiation terminal=%#v calls=%#v", terminal, runner.calls)
			}
		})
	}
}

func TestClientBootstrapsExactLegacyTerminalWithAttestedCodex(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal send --terminal term-legacy --text codex --dangerously-bypass-hook-trust --enter --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"send":{"accepted":true}}}`)}
	runner.responses["orca terminal wait --terminal term-legacy --for tui-idle --timeout-ms 10000 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"wait":{"satisfied":true}}}`)}
	if err := NewClient(runner).BootstrapTerminalAgent(context.Background(), port.OrcaBootstrapTerminalAgentRequest{TerminalHandle: "term-legacy", Agent: "codex", AllowCodexHookTrustBypass: true}); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"orca", "terminal", "send", "--terminal", "term-legacy", "--text", "codex --dangerously-bypass-hook-trust", "--enter", "--json"},
		{"orca", "terminal", "wait", "--terminal", "term-legacy", "--for", "tui-idle", "--timeout-ms", "10000", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("bootstrap calls = %#v, want %#v", runner.calls, want)
	}
}

func TestClientCreateTerminalCapabilityLossIsPreInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --title --json")}
	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "worktree-1", Agent: "codex"})
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "terminal_create_capability_missing" || orcaErr.Invoked {
		t.Fatalf("terminal capability loss error = %#v", err)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != "orca terminal create --help" {
		t.Fatalf("terminal capability loss invoked mutation: %#v", runner.calls)
	}
}

func TestProbeAcceptsFixedAgentTerminalCreateCapability(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --agent --title --json")}
	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil || !result.Ready {
		t.Fatalf("fixed --agent capability probe = %#v err=%v", result, err)
	}
}

func TestClientCreateTerminalAcceptsRuntimeIdentityWithoutPTY(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
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
		{"orca", "terminal", "create", "--help"},
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
			runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
			key := "orca terminal create --worktree id:worktree-1 --command " + tt.command + " --json"
			runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-create","worktreeId":"worktree-1"}}}`)}
			_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
				WorktreeID: "worktree-1", Agent: tt.agent, AllowCodexHookTrustBypass: tt.attested,
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"orca", "terminal", "create", "--worktree", "id:worktree-1", "--command", tt.command, "--json"}
			if len(runner.calls) != 2 || !reflect.DeepEqual(runner.calls[1], want) {
				t.Fatalf("terminal launch = %#v, want %#v", runner.calls, want)
			}
		})
	}
}

func TestClientCreateTerminalRejectsIncompleteRuntimeIdentity(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
	runner.responses["orca terminal create --worktree id:worktree-1 --command codex --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {"terminal": {"ptyId": "pty-2", "worktreeId": "worktree-1"}}
	}`)}

	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "worktree-1", Agent: "codex"})
	if err == nil || !strings.Contains(err.Error(), "terminal identity") {
		t.Fatalf("CreateTerminal() error = %v, want terminal identity error", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("incomplete create identity made an extra call: %#v", runner.calls)
	}
}

func TestClientListAllTasksProjectsCompletionSemanticsWithoutRawResult(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration task-list --brief --json"
	runner.responses[command] = fixtureOutput(t, "task_list_all.json")

	got, err := NewClient(runner).ListAllTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].RuntimeID != "runtime-1" || got[0].ID != "task-ready" || got[0].CompletedAt != "" || got[0].HasResult || got[1].RuntimeID != "runtime-1" || got[1].ID != "task-complete" || got[1].CompletedAt != "2026-07-19T01:02:03.000Z" || !got[1].HasResult {
		t.Fatalf("all-task semantic projection = %#v", got)
	}
	projected := fmt.Sprintf("%#v", got)
	if strings.Contains(projected, "raw-spec-must-not-escape") || strings.Contains(projected, "raw-result-must-not-escape") {
		t.Fatalf("all-task projection retained raw content: %s", projected)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
		t.Fatalf("all-task command = %#v", runner.calls)
	}
}

func TestClientListAllTasksRejectsCountMismatch(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration task-list --brief --json"
	runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[{"id":"task-1","status":"ready"}],"count":2}}`)}

	_, err := NewClient(runner).ListAllTasks(context.Background())

	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("count mismatch error = %v", err)
	}
}

func TestClientListGatesRequiresCountEquality(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		runner := newFakeRunner(t)
		command := "orca orchestration gate-list --json"
		runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[{"id":"gate-1","task_id":"task-1","status":"pending","question":"raw-question-must-not-escape"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}

		got, err := NewClient(runner).ListGates(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].RuntimeID != "runtime-1" || got[0].ID != "gate-1" || got[0].TaskID != "task-1" || got[0].Status != "pending" || strings.Contains(fmt.Sprintf("%#v", got), "raw-question-must-not-escape") {
			t.Fatalf("gate projection = %#v", got)
		}
		if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
			t.Fatalf("gate-list command = %#v", runner.calls)
		}
	})

	t.Run("count mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration gate-list --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[],"count":1}}`)}

		_, err := NewClient(runner).ListGates(context.Background())

		if err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("gate count mismatch error = %v", err)
		}
	})
}

func TestClientOperationalInventoryRejectsMalformedIdentity(t *testing.T) {
	tests := []struct {
		name    string
		command string
		result  string
		call    func(*Client) error
	}{
		{
			name:    "task id",
			command: "orca orchestration task-list --brief --json",
			result:  `{"tasks":[{"id":"","status":"ready"}],"count":1}`,
			call:    func(client *Client) error { _, err := client.ListAllTasks(context.Background()); return err },
		},
		{
			name:    "gate task id",
			command: "orca orchestration gate-list --json",
			result:  `{"gates":[{"id":"gate-1","task_id":"","status":"pending"}],"count":1}`,
			call:    func(client *Client) error { _, err := client.ListGates(context.Background()); return err },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses[test.command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":` + test.result + `}`)}

			err := test.call(NewClient(runner))

			if err == nil || !strings.Contains(err.Error(), "identity") {
				t.Fatalf("malformed %s error = %v", test.name, err)
			}
		})
	}
}

func TestClientInboxPresenceProvesOnlyBoundedZero(t *testing.T) {
	tests := []struct {
		name            string
		result          string
		count           int
		rows            int
		completeAbsence bool
	}{
		{name: "bounded zero", result: `{"messages":[],"count":0}`, completeAbsence: true},
		{name: "present", result: `{"messages":[{"id":"msg-1","subject":"raw-subject-must-not-escape","body":"raw-body-must-not-escape","payload":"raw-payload-must-not-escape"}],"count":1}`, count: 1, rows: 1},
		{name: "count mismatch", result: `{"messages":[{"id":"msg-1","body":"raw-body-must-not-escape"}],"count":0}`, rows: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			command := "orca orchestration inbox --limit 1 --json"
			runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":` + test.result + `,"_meta":{"runtimeId":"runtime-1"}}`)}

			got, err := NewClient(runner).InboxPresence(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.RuntimeID != "runtime-1" || got.Count != test.count || got.RowCount != test.rows || got.CompleteAbsence != test.completeAbsence {
				t.Fatalf("inbox presence = %#v", got)
			}
			projected := fmt.Sprintf("%#v", got)
			if strings.Contains(projected, "raw-subject") || strings.Contains(projected, "raw-body") || strings.Contains(projected, "raw-payload") {
				t.Fatalf("inbox presence retained raw content: %s", projected)
			}
			if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
				t.Fatalf("inbox command = %#v", runner.calls)
			}
		})
	}
}

func TestClientResolveRepoReturnsCanonicalRegistration(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca repo show --repo path:/absolute/repo --json"
	runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"repo":{"id":"repo-1","path":"/absolute/repo","displayName":"repo","worktreeBasePath":"../repo.worktrees","gitRemoteIdentity":{"remoteName":"origin"}}},"_meta":{"runtimeId":"runtime-1"}}`)}

	got, err := NewClient(runner).ResolveRepo(context.Background(), "/absolute/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got.RuntimeID != "runtime-1" || got.ID != "repo-1" || got.Path != "/absolute/repo" || got.RemoteName != "origin" || len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
		t.Fatalf("canonical repo projection = %#v calls=%#v", got, runner.calls)
	}

	t.Run("path mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"repo":{"id":"repo-1","path":"/different/repo"}}}`)}

		_, err := NewClient(runner).ResolveRepo(context.Background(), "/absolute/repo")

		if err == nil || !strings.Contains(err.Error(), "repo identity") {
			t.Fatalf("repo path mismatch error = %v", err)
		}
	})
}

func TestClientAvailableUsesPathLookupOnly(t *testing.T) {
	runner := newFakeRunner(t)
	if NewClient(runner).Available() {
		t.Fatal("missing Orca binary reported available")
	}
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	if !NewClient(runner).Available() || len(runner.calls) != 0 {
		t.Fatalf("available check ran commands: %#v", runner.calls)
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
	runner.responses["orca orchestration task-list --ready --json"] = fixtureOutput(t, "task_list.json")
	got, err := NewClient(runner).ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "task-1" || got[0].Title != "agent-harness marker" || got[0].Status != "ready" {
		t.Fatalf("task list projection = %#v", got)
	}
}

func TestClientListDispatchedTasksUsesServerFilteredCompleteInventory(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration task-list --status dispatched --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {"tasks": [{"id": "task-dispatched", "task_title": "writer", "status": "dispatched"}], "count": 1},
		"_meta": {"runtimeId": "runtime-1"}
	}`)}
	got, err := NewClient(runner).ListDispatchedTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuntimeID != "runtime-1" || got[0].ID != "task-dispatched" || got[0].Status != "dispatched" {
		t.Fatalf("dispatched task projection = %#v", got)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != "orca orchestration task-list --status dispatched --json" {
		t.Fatalf("dispatched task inventory was not server filtered: %#v", runner.calls)
	}
}

func TestClientShowDispatchDecodesInstalledShapeWithoutInjectedField(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration dispatch-show --task task-1 --json"] = fixtureOutput(t, "dispatch_show.json")
	got, err := NewClient(runner).ShowDispatch(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RuntimeID != "runtime-1" || got.ID != "dispatch-1" || got.TaskID != "task-1" || got.AssigneeHandle != "term-live" || got.Status != "dispatched" {
		t.Fatalf("installed dispatch-show projection = %#v", got)
	}
	if got.Injected {
		t.Fatalf("dispatch-show must not synthesize absent injected evidence: %#v", got)
	}
}

func TestClientShowDispatchNullReturnsNotFound(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration dispatch-show --task task-absent --json"] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"dispatch":null}}`)}
	_, err := NewClient(runner).ShowDispatch(context.Background(), "task-absent")
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "not_found" {
		t.Fatalf("dispatch=null error = %v, want Orca not_found", err)
	}
}

func TestClientShowDispatchFromRequestsOfficialPreambleForSealedCoordinator(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration dispatch-show --task task-1 --preamble --from term_coordinator --json"
	runner.responses[command] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"dispatch":{"id":"dispatch-1","task_id":"task-1","assignee_handle":"term_worker","status":"dispatched"},"preamble":"Your coordinator's terminal handle is: term_coordinator\nYour task ID is: task-1\n--dispatch-id dispatch-1"}}`)}
	got, err := NewClient(runner).ShowDispatchFrom(context.Background(), "task-1", "term_coordinator")
	if err != nil {
		t.Fatal(err)
	}
	if got.Preamble == "" || len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
		t.Fatalf("dispatch preamble projection=%#v calls=%#v", got, runner.calls)
	}
}

func TestClientRejectsIncompleteExternalLists(t *testing.T) {
	for _, tt := range []struct {
		name, command, field string
		call                 func(*Client) error
	}{
		{name: "worktree truncated", command: "orca worktree list --repo path:/repo --limit 512 --json", field: "worktrees", call: func(c *Client) error { _, err := c.ListWorktrees(context.Background(), "/repo"); return err }},
		{name: "terminal total mismatch", command: "orca terminal list --worktree id:wt-1 --limit 512 --json", field: "terminals", call: func(c *Client) error { _, err := c.ListTerminals(context.Background(), "wt-1"); return err }},
		{name: "task missing metadata", command: "orca orchestration task-list --ready --json", field: "tasks", call: func(c *Client) error { _, err := c.ListTasks(context.Background()); return err }},
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
		"orca orchestration task-list --help":     "--ready --status --json",
		"orca orchestration task-update --help":   "--id --status --result --json",
		"orca orchestration dispatch --help":      "--task --to --from --inject --return-preamble --json",
		"orca orchestration dispatch-show --help": "--task --preamble --from --json",
		"orca orchestration send --help":          "--to --from --type --subject --body --task-id --dispatch-id --files-modified --report-path --json",
		"orca worktree rm --help":                 "--worktree --force --json",
	} {
		runner.responses[command] = CommandOutput{Stdout: []byte(flags)}
	}
	runner.responses["orca orchestration task-list --ready --json"] = fixtureOutput(runner.t, "task_list.json")
	runner.responses["codex --help"] = CommandOutput{Stdout: []byte("--dangerously-bypass-hook-trust")}
}
