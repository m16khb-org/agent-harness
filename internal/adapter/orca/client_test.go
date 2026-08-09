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
	"sync"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestClientProjectsWorkerDoneThroughDedicatedSafeArgvMethod(t *testing.T) {
	runner := newFakeRunner(t)
	req := port.OrcaWorkerDoneRequest{
		RunID:      "run_issueops_1",
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed issue io-demo",
		Body:   "Implementation completed at abcdef. Verification evidence is persisted. The coordinator may inspect the submitted record.",
		TaskID: "task-1", DispatchID: "dispatch-1", Outcome: "succeeded", ChangedFiles: []string{"a.go", "report.md"}, ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--run", req.RunID, "--to", req.ToHandle, "--from", req.FromHandle, "--type", "worker_done", "--subject", req.Subject, "--body", req.Body, "--task-id", req.TaskID, "--dispatch-id", req.DispatchID, "--outcome", req.Outcome, "--files-modified", "a.go,report.md", "--report-path", req.ReportPath, "--json"}
	runner.responses[strings.Join(argv, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"message":{"id":"msg-1","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed issue io-demo","body":"Implementation completed at abcdef. Verification evidence is persisted. The coordinator may inspect the submitted record.","payload":"{\"taskId\":\"task-1\",\"dispatchId\":\"dispatch-1\",\"outcome\":\"succeeded\",\"filesModified\":[\"a.go\",\"report.md\"],\"reportPath\":\"/repo/report.md\"}","sequence":42}}}`)}
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
		RunID:      "run_issueops_1",
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed no-change issue io-demo",
		Body:   "Verification evidence is persisted and no files changed.",
		TaskID: "task-1", DispatchID: "dispatch-1", Outcome: "succeeded", ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--run", req.RunID, "--to", req.ToHandle, "--from", req.FromHandle, "--type", "worker_done", "--subject", req.Subject, "--body", req.Body, "--task-id", req.TaskID, "--dispatch-id", req.DispatchID, "--outcome", req.Outcome, "--report-path", req.ReportPath, "--json"}
	runner.responses[strings.Join(argv, " ")] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"message":{"id":"msg-clean","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed no-change issue io-demo","body":"Verification evidence is persisted and no files changed.","payload":"{\"taskId\":\"task-1\",\"dispatchId\":\"dispatch-1\",\"outcome\":\"succeeded\",\"filesModified\":[],\"reportPath\":\"/repo/report.md\"}","sequence":43}}}`)}

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
		RunID:      "run_issueops_1",
		FromHandle: "term_worker", ToHandle: "term_coordinator", Subject: "Completed issue io-demo",
		Body:   "Implementation completed. Verification evidence persisted. Coordinator inspection is ready.",
		TaskID: "task-1", DispatchID: "dispatch-1", Outcome: "succeeded", ChangedFiles: []string{"a.go"}, ReportPath: "/repo/report.md",
	}
	argv := []string{"orca", "orchestration", "send", "--run", valid.RunID, "--to", valid.ToHandle, "--from", valid.FromHandle, "--type", "worker_done", "--subject", valid.Subject, "--body", valid.Body, "--task-id", valid.TaskID, "--dispatch-id", valid.DispatchID, "--outcome", valid.Outcome, "--files-modified", "a.go", "--report-path", valid.ReportPath, "--json"}
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
		{name: "mismatch", stdout: `{"ok":true,"result":{"message":{"id":"msg-1","from_handle":"term_worker","to_handle":"term_coordinator","type":"worker_done","subject":"Completed issue io-demo","body":"Implementation completed. Verification evidence persisted. Coordinator inspection is ready.","payload":"{\"taskId\":\"task-other\",\"dispatchId\":\"dispatch-1\",\"outcome\":\"succeeded\",\"filesModified\":[\"a.go\"],\"reportPath\":\"/repo/report.md\"}","sequence":42}}}`, code: "worker_done_response_mismatch"},
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

func TestProbeRequiresHostModelSelectionCapability(t *testing.T) {
	for _, tt := range []struct {
		name  string
		agent string
		help  string
	}{
		{name: "codex", agent: "codex", help: "--dangerously-bypass-hook-trust"},
		{name: "claude", agent: "claude", help: "Usage: claude"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.lookPaths["orca"] = "/usr/local/bin/orca"
			runner.lookPaths[tt.agent] = "/usr/local/bin/" + tt.agent
			runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
			runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
			addCompleteProbeLeafHelp(runner)
			runner.responses[tt.agent+" --help"] = CommandOutput{Stdout: []byte(tt.help)}

			result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: tt.agent})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready || result.Code != "host_model_selection_unsupported" {
				t.Fatalf("missing %s model selection probe = %#v", tt.agent, result)
			}
			for _, call := range runner.calls {
				joined := strings.Join(call, " ")
				if strings.Contains(joined, " create ") && !strings.HasSuffix(joined, " --help") {
					t.Fatalf("%s capability probe mutated state: %s", tt.agent, joined)
				}
			}
		})
	}
}

func TestProbeDoesNotApplyCodexBypassRequirementToClaude(t *testing.T) {
	for _, agent := range []string{"claude"} {
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
			runner.responses["orca orchestration run-list --json"] = tt.output
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
	runner.responses["orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --parent-worktree path:/repo.worktrees/15-umbrella --setup skip --comment agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1 --issue 16 --json"] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"worktree-1","instanceId":"instance-1","repoId":"repo-1","path":"/repo.worktrees/16-demo","head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","branch":"refs/heads/16-demo","comment":"agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1","baseRef":"refs/remotes/origin/16-demo","linkedIssue":16,"parentWorktreeId":"repo-1::/repo.worktrees/15-umbrella","lineage":{"capture":{"source":"explicit-cli-flag","confidence":"explicit"}}}},"_meta":{"runtimeId":"runtime-1"}}`)}
	client := NewClient(runner)
	result, err := client.CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "16-demo", BaseBranch: "refs/remotes/origin/16-demo",
		ParentWorktree: "/repo.worktrees/15-umbrella",
		Issue:          16,
		Comment:        "agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "worktree-1" || result.InstanceID != "instance-1" ||
		result.ParentWorktreeID != "repo-1::/repo.worktrees/15-umbrella" ||
		result.LineageSource != "explicit-cli-flag" || result.LineageConfidence != "explicit" {
		t.Fatalf("created worktree = %#v", result)
	}
	want := []string{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "16-demo", "--base-branch", "refs/remotes/origin/16-demo", "--parent-worktree", "path:/repo.worktrees/15-umbrella", "--setup", "skip", "--comment", "agent-harness:cycle=io-demo;attempt=1;epoch=epoch-1", "--issue", "16", "--json"}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("argv = %#v, want %#v", runner.calls, want)
	}
}

func TestClientCanonicalizesNamespacedCreatedBranchAndUpstream(t *testing.T) {
	runner := newFakeRunner(t)
	create := "orca worktree create --repo path:/repo --name 16-demo --base-branch refs/remotes/origin/16-demo --no-parent --setup skip --comment marker --issue 16 --json"
	runner.responses[create] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"wt-1","instanceId":"inst-1","repoId":"repo-1","path":"/repo.worktrees/16-demo","head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","branch":"refs/heads/alice/16-demo","comment":"marker","baseRef":"refs/remotes/origin/16-demo","linkedIssue":16}},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["git branch -m 16-demo"] = CommandOutput{Invoked: true}
	runner.responses["git branch --set-upstream-to refs/remotes/origin/16-demo 16-demo"] = CommandOutput{Invoked: true}

	got, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "16-demo", BaseBranch: "refs/remotes/origin/16-demo", Provider: "github", Issue: 16, Comment: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{
		{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "16-demo", "--base-branch", "refs/remotes/origin/16-demo", "--no-parent", "--setup", "skip", "--comment", "marker", "--issue", "16", "--json"},
		{"git", "branch", "-m", "16-demo"},
		{"git", "branch", "--set-upstream-to", "refs/remotes/origin/16-demo", "16-demo"},
	}
	if got.Branch != "16-demo" || !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("canonicalized worktree = %#v calls=%#v", got, runner.calls)
	}
}

func TestClientCanonicalizesFromExactSHAWithoutUsingSHAAsUpstream(t *testing.T) {
	runner := newFakeRunner(t)
	baseSHA := strings.Repeat("a", 40)
	create := "orca worktree create --repo path:/repo --name 72-fix --base-branch " + baseSHA + " --no-parent --setup skip --comment marker --issue 72 --json"
	runner.responses[create] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"wt-72","instanceId":"inst-72","repoId":"repo-1","path":"/repo.worktrees/72-fix","head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","branch":"refs/heads/alice/72-fix","comment":"marker","linkedIssue":72}},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["git branch -m 72-fix"] = CommandOutput{Invoked: true}

	got, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "72-fix", BaseBranch: baseSHA,
		Provider: "github", Issue: 72, Comment: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{
		{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "72-fix", "--base-branch", baseSHA, "--no-parent", "--setup", "skip", "--comment", "marker", "--issue", "72", "--json"},
		{"git", "branch", "-m", "72-fix"},
	}
	if got.Branch != "72-fix" || !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("exact base/upstream identities were conflated: worktree=%#v calls=%#v", got, runner.calls)
	}
}

// IssueOps child의 원격 브랜치는 Orca 생성 시점에 아직 없어야 한다. 따라서 exact
// base SHA만 전달된 경우 namespace 제거는 로컬 rename으로 끝나야 한다.
func TestClientCanonicalizesFromExactSHAWithoutAnUpstream(t *testing.T) {
	runner := newFakeRunner(t)
	baseSHA := strings.Repeat("a", 40)
	create := "orca worktree create --repo path:/repo --name 191-spike --base-branch " + baseSHA + " --no-parent --setup skip --comment marker --issue 191 --json"
	runner.responses[create] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"wt-191","instanceId":"inst-191","repoId":"repo-1","path":"/repo.worktrees/191-spike","head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","branch":"refs/heads/alice/191-spike","comment":"marker","linkedIssue":191}},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["git branch -m 191-spike"] = CommandOutput{Invoked: true}

	got, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "191-spike", BaseBranch: baseSHA,
		Provider: "github", Issue: 191, Comment: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{
		{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "191-spike", "--base-branch", baseSHA, "--no-parent", "--setup", "skip", "--comment", "marker", "--issue", "191", "--json"},
		{"git", "branch", "-m", "191-spike"},
	}
	if got.Branch != "191-spike" || !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("부재한 upstream을 설정했다: worktree=%#v calls=%#v", got, runner.calls)
	}
}

func TestClientCanonicalizesGitLabNumericSuffixAgainstSealedRemoteBranch(t *testing.T) {
	runner := newFakeRunner(t)
	baseSHA := strings.Repeat("a", 40)
	remoteRef := "refs/remotes/origin/2609-fix"
	revParse := "git rev-parse --verify --quiet " + remoteRef
	create := "orca worktree create --repo path:/repo --name 2609-fix --base-branch " + baseSHA + " --no-parent --setup skip --comment marker --json"
	runner.responses[revParse] = CommandOutput{Invoked: true, Stdout: []byte(baseSHA + "\n")}
	runner.responses[create] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"wt-2609","instanceId":"inst-2609","repoId":"repo-1","path":"/repo.worktrees/2609-fix","head":"` + baseSHA + `","branch":"refs/heads/2609-fix-2","comment":"marker","linkedGitLabIssue":null}},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["git branch -m 2609-fix"] = CommandOutput{Invoked: true}
	runner.responses["git branch --set-upstream-to "+remoteRef+" 2609-fix"] = CommandOutput{Invoked: true}

	got, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "2609-fix", BaseBranch: baseSHA,
		Provider: "gitlab", Issue: 2609, Comment: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := [][]string{
		{"git", "rev-parse", "--verify", "--quiet", remoteRef},
		{"orca", "worktree", "create", "--repo", "path:/repo", "--name", "2609-fix", "--base-branch", baseSHA, "--no-parent", "--setup", "skip", "--comment", "marker", "--json"},
		{"git", "rev-parse", "--verify", "--quiet", remoteRef},
		{"git", "branch", "-m", "2609-fix"},
		{"git", "branch", "--set-upstream-to", remoteRef, "2609-fix"},
	}
	if got.Branch != "2609-fix" || !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("GitLab 예약 브랜치를 정규화하지 못했다: worktree=%#v calls=%#v", got, runner.calls)
	}
}

func TestClientRejectsGitLabNumericSuffixWithoutExactSealedRemoteBranch(t *testing.T) {
	runner := newFakeRunner(t)
	baseSHA := strings.Repeat("a", 40)
	otherSHA := strings.Repeat("b", 40)
	remoteRef := "refs/remotes/origin/2609-fix"
	revParse := "git rev-parse --verify --quiet " + remoteRef
	create := "orca worktree create --repo path:/repo --name 2609-fix --base-branch " + baseSHA + " --no-parent --setup skip --comment marker --json"
	runner.responses[revParse] = CommandOutput{Invoked: true, Stdout: []byte(otherSHA + "\n")}
	runner.responses[create] = CommandOutput{Invoked: true, Stdout: []byte(`{"ok":true,"result":{"worktree":{"id":"wt-2609","instanceId":"inst-2609","repoId":"repo-1","path":"/repo.worktrees/2609-fix","head":"` + baseSHA + `","branch":"refs/heads/2609-fix-2","comment":"marker","linkedGitLabIssue":null}},"_meta":{"runtimeId":"runtime-1"}}`)}

	if _, err := NewClient(runner).CreateWorktree(context.Background(), port.OrcaCreateWorktreeRequest{
		Repo: "/repo", Name: "2609-fix", BaseBranch: baseSHA,
		Provider: "gitlab", Issue: 2609, Comment: "marker",
	}); err == nil {
		t.Fatal("원격 브랜치 SHA가 봉인된 base와 다르면 숫자 접미사를 정규화하면 안 된다")
	}
	if len(runner.calls) != 2 {
		t.Fatalf("불일치 뒤 rename이나 upstream mutation이 실행됐다: %#v", runner.calls)
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

func TestClientCreateTerminalUsesCallerSelectedHostLaunchProfile(t *testing.T) {
	for _, tt := range []struct {
		name, agent, model, effort, command string
	}{
		{
			name: "Codex Terra high", agent: "codex", model: "gpt-5.6-terra", effort: "high",
			command: "codex --model 'gpt-5.6-terra' -c model_reasoning_effort='high' --dangerously-bypass-hook-trust",
		},
		{
			name: "Claude Opus 4.8", agent: "claude", model: "opus",
			command: "claude --model 'opus'",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
			key := "orca terminal create --worktree id:worktree-1 --command " + tt.command + " --json"
			runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-create","worktreeId":"worktree-1"}}}`)}

			_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
				WorktreeID: "worktree-1", Agent: tt.agent, Model: tt.model, ReasoningEffort: tt.effort, AllowCodexHookTrustBypass: true,
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

func TestClientBootstrapsExactOwnedTerminalWithSealedCodexProfile(t *testing.T) {
	runner := newFakeRunner(t)
	command := `codex --model 'gpt-5.6-terra' -c model_reasoning_effort='high' --dangerously-bypass-hook-trust`
	runner.responses["orca terminal send --terminal term-owned --text "+command+" --enter --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"send":{"accepted":true}}}`)}
	runner.responses["orca terminal wait --terminal term-owned --for tui-idle --timeout-ms 10000 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"wait":{"satisfied":true}}}`)}
	if err := NewClient(runner).BootstrapTerminalAgent(context.Background(), port.OrcaBootstrapTerminalAgentRequest{TerminalHandle: "term-owned", Agent: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high", AllowCodexHookTrustBypass: true}); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"orca", "terminal", "send", "--terminal", "term-owned", "--text", command, "--enter", "--json"},
		{"orca", "terminal", "wait", "--terminal", "term-owned", "--for", "tui-idle", "--timeout-ms", "10000", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("bootstrap calls = %#v, want %#v", runner.calls, want)
	}
}

func TestClientCreateTerminalCapabilityLossIsPreInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --title --json")}
	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "worktree-1", Agent: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high"})
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "terminal_create_capability_missing" || orcaErr.Invoked {
		t.Fatalf("terminal capability loss error = %#v", err)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != "orca terminal create --help" {
		t.Fatalf("terminal capability loss invoked mutation: %#v", runner.calls)
	}
}

func TestProbeRejectsAgentOnlyTerminalCreateCapability(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --agent --title --json")}
	result, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil || result.Ready || result.Code != "capability_missing" {
		t.Fatalf("agent-only capability probe = %#v err=%v", result, err)
	}
}

func TestClientCreateTerminalAcceptsRuntimeIdentityWithoutPTY(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
	command := `codex --model 'gpt-5.6-terra' -c model_reasoning_effort='high'`
	runner.responses["orca terminal create --worktree id:worktree-1 --command "+command+" --title marker --json"] = CommandOutput{Stdout: []byte(`{
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
		WorktreeID: "worktree-1", Agent: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high", Title: "marker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Handle != "term-create" || terminal.PTYID != "" || terminal.WorktreeID != "worktree-1" {
		t.Fatalf("created terminal identity = %#v", terminal)
	}
	want := [][]string{
		{"orca", "terminal", "create", "--help"},
		{"orca", "terminal", "create", "--worktree", "id:worktree-1", "--command", command, "--title", "marker", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestClientCreateTerminalUsesCodexBypassOnlyWhenAttested(t *testing.T) {
	tests := []struct {
		name, agent, model, effort, command string
		attested                            bool
	}{
		{name: "attested Codex", agent: "codex", model: "caller-model", effort: "high", command: `codex --model 'caller-model' -c model_reasoning_effort='high' --dangerously-bypass-hook-trust`, attested: true},
		{name: "ordinary Codex", agent: "codex", model: "caller-model", effort: "xhigh", command: `codex --model 'caller-model' -c model_reasoning_effort='xhigh'`},
		{name: "Claude caller profile", agent: "claude", model: "caller-model", effort: "max", command: "claude --model 'caller-model' --effort 'max'", attested: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses["orca terminal create --help"] = CommandOutput{Stdout: []byte("--worktree --command --title --json")}
			key := "orca terminal create --worktree id:worktree-1 --command " + tt.command + " --json"
			runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminal":{"handle":"term-create","worktreeId":"worktree-1"}}}`)}
			_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{
				WorktreeID: "worktree-1", Agent: tt.agent, Model: tt.model, ReasoningEffort: tt.effort, AllowCodexHookTrustBypass: tt.attested,
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
	command := `codex --model 'gpt-5.6-terra' -c model_reasoning_effort='high'`
	runner.responses["orca terminal create --worktree id:worktree-1 --command "+command+" --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {"terminal": {"ptyId": "pty-2", "worktreeId": "worktree-1"}}
	}`)}

	_, err := NewClient(runner).CreateTerminal(context.Background(), port.OrcaCreateTerminalRequest{WorktreeID: "worktree-1", Agent: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high"})
	if err == nil || !strings.Contains(err.Error(), "terminal identity") {
		t.Fatalf("CreateTerminal() error = %v, want terminal identity error", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("incomplete create identity made an extra call: %#v", runner.calls)
	}
}

func TestClientListAllTasksProjectsCompletionSemanticsWithoutRawResult(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration task-list --brief --run run_issueops_1 --json"
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
	if len(runner.calls) != 2 || strings.Join(runner.calls[1], " ") != command {
		t.Fatalf("all-task command = %#v", runner.calls)
	}
}

func TestClientListAllTasksRejectsCountMismatch(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration task-list --brief --run run_issueops_1 --json"
	runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runId":"run_issueops_1","tasks":[{"id":"task-1","status":"ready"}],"count":2}}`)}

	_, err := NewClient(runner).ListAllTasks(context.Background())

	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("count mismatch error = %v", err)
	}
}

func TestClientListFailedTasksUsesStatusFilter(t *testing.T) {
	runner := newFakeRunner(t)
	command := "orca orchestration task-list --status failed --run run_issueops_1 --json"
	runner.responses[command] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runId":"run_issueops_1","tasks":[{"id":"task-failed","status":"failed"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}

	got, err := NewClient(runner).ListFailedTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "task-failed" || got[0].Status != "failed" || got[0].RuntimeID != "runtime-1" {
		t.Fatalf("failed-task projection = %#v", got)
	}
	if len(runner.calls) != 2 || strings.Join(runner.calls[1], " ") != command {
		t.Fatalf("failed-task command = %#v", runner.calls)
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
			command: "orca orchestration task-list --brief --run run_issueops_1 --json",
			result:  `{"runId":"run_issueops_1","tasks":[{"id":"","status":"ready"}],"count":1}`,
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
	runner.responses["orca orchestration task-create --spec spec --task-title agent-harness marker --display-name 16-demo --run run_issueops_1 --json"] = fixtureOutput(t, "task_create.json")
	got, err := NewClient(runner).CreateTask(context.Background(), port.OrcaCreateTaskRequest{RunID: "run_issueops_1", Spec: "spec", Title: "agent-harness marker", DisplayName: "16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "task-1" || got.Title != "agent-harness marker" || got.DisplayName != "16-demo" || got.Status != "ready" {
		t.Fatalf("official task projection = %#v", got)
	}
}

func TestClientListTasksUsesInstalledCountContract(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration task-list --ready --run run_issueops_1 --json"] = fixtureOutput(t, "task_list.json")
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
	runner.responses["orca orchestration task-list --status dispatched --run run_issueops_1 --json"] = CommandOutput{Stdout: []byte(`{
		"ok": true,
		"result": {"runId": "run_issueops_1", "tasks": [{"id": "task-dispatched", "task_title": "writer", "status": "dispatched"}], "count": 1},
		"_meta": {"runtimeId": "runtime-1"}
	}`)}
	got, err := NewClient(runner).ListDispatchedTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuntimeID != "runtime-1" || got[0].ID != "task-dispatched" || got[0].Status != "dispatched" {
		t.Fatalf("dispatched task projection = %#v", got)
	}
	if len(runner.calls) != 2 || strings.Join(runner.calls[1], " ") != "orca orchestration task-list --status dispatched --run run_issueops_1 --json" {
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

func TestClientExecutionInventoryPreservesRuntimeForEmptyRows(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca terminal list --worktree id:wt-1 --limit 512 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminals":[],"visualLayouts":[],"totalCount":0,"truncated":false},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["orca orchestration task-list --brief --run run_issueops_1 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runId":"run_issueops_1","tasks":[],"count":0},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["orca orchestration dispatch-show --task task-1 --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"dispatch":null},"_meta":{"runtimeId":"runtime-1"}}`)}
	client := NewClient(runner)

	terminals, err := client.listTerminalsInventory(context.Background(), "wt-1")
	if err != nil || terminals.RuntimeID != "runtime-1" || len(terminals.Rows) != 0 {
		t.Fatalf("empty terminal inventory lost its runtime envelope: inventory=%#v err=%v", terminals, err)
	}
	tasks, err := client.listAllTasksInventory(context.Background())
	if err != nil || tasks.RuntimeID != "runtime-1" || len(tasks.Rows) != 0 {
		t.Fatalf("empty task inventory lost its runtime envelope: inventory=%#v err=%v", tasks, err)
	}
	dispatch, err := client.showDispatchInventory(context.Background(), "task-1")
	if err != nil || dispatch.RuntimeID != "runtime-1" || dispatch.Dispatch != nil {
		t.Fatalf("absent dispatch inventory lost its runtime envelope: inventory=%#v err=%v", dispatch, err)
	}
}

func TestClientShowTerminalInventoryPreservesPaneRuntimeEvidence(t *testing.T) {
	for _, paneRuntimeID := range []int{1, -1} {
		t.Run(fmt.Sprintf("pane-runtime-%d", paneRuntimeID), func(t *testing.T) {
			runner := newFakeRunner(t)
			command := "orca terminal show --terminal term_live --json"
			runner.responses[command] = CommandOutput{Invoked: true, Stdout: []byte(fmt.Sprintf(`{
				"ok": true,
				"result": {"terminal": {
					"handle": "term_live",
					"ptyId": "repo::/worktree@@pane",
					"worktreeId": "repo::/worktree",
					"connected": true,
					"writable": true,
					"paneRuntimeId": %d
				}},
				"_meta": {"runtimeId": "runtime-1"}
			}`, paneRuntimeID))}

			got, err := NewClient(runner).showTerminalInventory(context.Background(), "term_live")
			if err != nil {
				t.Fatal(err)
			}
			if got.RuntimeID != "runtime-1" || got.Terminal.RuntimeID != "runtime-1" ||
				got.Terminal.PTYID != "repo::/worktree@@pane" || got.PaneRuntimeID == nil ||
				*got.PaneRuntimeID != paneRuntimeID {
				t.Fatalf("terminal 상세 증거가 손실됐다: %#v", got)
			}
			if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != command {
				t.Fatalf("terminal 상세 조회 argv가 다르다: %#v", runner.calls)
			}
		})
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
		{name: "task missing metadata", command: "orca orchestration task-list --ready --run run_issueops_1 --json", field: `runId":"run_issueops_1","tasks`, call: func(c *Client) error { _, err := c.ListTasks(context.Background()); return err }},
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
	mu        sync.Mutex
	calls     [][]string
}

func newFakeRunner(t *testing.T) *fakeRunner {
	t.Setenv("ORCA_TERMINAL_HANDLE", "term_coordinator")
	return &fakeRunner{
		t:         t,
		lookPaths: map[string]string{},
		responses: map[string]CommandOutput{"orca orchestration run-list --json": fixtureOutput(t, "run_list.json")},
		errors:    map[string]error{},
	}
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if path := f.lookPaths[file]; path != "" {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ time.Duration, argv []string) (CommandOutput, error) {
	copyArgv := append([]string(nil), argv...)
	f.mu.Lock()
	f.calls = append(f.calls, copyArgv)
	f.mu.Unlock()
	key := strings.Join(argv, " ")
	if err := f.errors[key]; err != nil {
		// 실제 ExecRunner처럼 비영 종료에서도 캡처된 stdout을 함께 돌려준다 —
		// runJSON의 실패-envelope 복원 경로(#97)를 검증할 수 있어야 한다.
		return f.responses[key], err
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
		"orca orchestration run-create --help":    "--objective --from --json",
		"orca orchestration run-list --help":      "--cursor --json",
		"orca orchestration run-current --help":   "--from --json",
		"orca orchestration run-use --help":       "--id --from --json",
		"orca orchestration task-create --help":   "--spec --task-title --display-name --run --from --json",
		"orca orchestration task-list --help":     "--ready --status --run --json",
		"orca orchestration gate-list --help":     "--run --json",
		"orca orchestration task-update --help":   "--id --status --result --run --from --json",
		"orca orchestration dispatch --help":      "--task --to --run --from --inject --return-preamble --json",
		"orca orchestration dispatch-show --help": "--task --preamble --from --json",
		"orca orchestration send --help":          "--run --to --from --type --subject --body --task-id --dispatch-id --outcome --files-modified --report-path --json",
		"orca worktree rm --help":                 "--worktree --force --json",
	} {
		runner.responses[command] = CommandOutput{Stdout: []byte(flags)}
	}
	runner.responses["orca orchestration run-current --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"run":null},"_meta":{"runtimeId":"runtime-1"}}`)}
	runner.responses["orca orchestration run-list --json"] = fixtureOutput(runner.t, "run_list.json")
	runner.responses["codex --help"] = CommandOutput{Stdout: []byte("--model --config --dangerously-bypass-hook-trust")}
	runner.responses["claude --help"] = CommandOutput{Stdout: []byte("--model")}
}
