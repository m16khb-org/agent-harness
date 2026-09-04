package mcpcli

import (
	"encoding/json"
	workercontract "issueops/internal/contract/worker"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPWorkerRunReadOnlyAllowsGitStatus(t *testing.T) {
	workerDir := t.TempDir()
	t.Setenv("ISSUEOPS_WORKER_DIR", workerDir)
	repo := makeGitRepoForContract(t)

	job := callMCPWorkerRunReadOnlyForTest(t, map[string]any{
		"kind":           "qa",
		"workspace_root": repo,
		"cwd":            repo,
		"argv":           []string{"git", "status", "--short"},
	})
	if !job.OK || job.Status != workercontract.WorkerStatusSucceeded {
		t.Fatalf("worker_run_read_only did not succeed: %+v", job)
	}
	if job.Result == nil || !job.Result.Executed || job.Result.ExitCode != 0 || !job.Result.ReadOnly {
		t.Fatalf("unexpected command result: %+v", job.Result)
	}
	if job.Result.Policy.WriteAllowed || job.Result.Policy.NetworkAllowed || job.Result.Policy.ShellAllowed {
		t.Fatalf("read-only MCP worker widened policy: %+v", job.Result.Policy)
	}
}

func TestMCPWorkerRunReadOnlyDeniesWriteNetworkAndShell(t *testing.T) {
	workerDir := t.TempDir()
	t.Setenv("ISSUEOPS_WORKER_DIR", workerDir)
	repo := makeGitRepoForContract(t)

	cases := []struct {
		name   string
		argv   []string
		reason string
	}{
		{name: "write", argv: []string{"touch", "marker"}, reason: "write_not_allowed"},
		{name: "network", argv: []string{"curl", "https://example.invalid"}, reason: "network_not_allowed"},
		{name: "shell", argv: []string{"sh", "-c", "echo hi"}, reason: "shell_interpreter_not_allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := callMCPWorkerRunReadOnlyForTest(t, map[string]any{
				"kind":           "qa-" + tc.name,
				"workspace_root": repo,
				"cwd":            repo,
				"argv":           tc.argv,
			})
			if job.Status != workercontract.WorkerStatusFailed || job.Result == nil || job.Result.Policy.Allowed {
				t.Fatalf("unsafe command was not denied: %+v", job)
			}
			if !containsStringForWorkerMCPTest(job.Result.Policy.DenyReasons, tc.reason) {
				t.Fatalf("deny reasons %v missing %q", job.Result.Policy.DenyReasons, tc.reason)
			}
			if tc.name == "write" {
				if _, err := filepath.Abs(filepath.Join(repo, "marker")); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func callMCPWorkerRunReadOnlyForTest(t *testing.T, args map[string]any) workercontract.WorkerJob {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": "worker_run_read_only", "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := HandleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("worker_run_read_only rpc error: %+v", rpcErr)
	}
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected MCP result type %T", result)
	}
	content, ok := outer["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected MCP content: %#v", outer["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("unexpected MCP text content: %#v", content[0])
	}
	var job workercontract.WorkerJob
	if err := json.Unmarshal([]byte(text), &job); err != nil {
		t.Fatalf("unmarshal worker job %q: %v", text, err)
	}
	return job
}

func containsStringForWorkerMCPTest(values []string, want string) bool {
	for _, value := range values {
		if value == want || strings.Contains(value, want) {
			return true
		}
	}
	return false
}
