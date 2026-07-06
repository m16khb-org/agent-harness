package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsWorktreePrepareToolsRunsCodeGraphAgainstWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "codegraph.log")
	codegraph := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+logPath+"'\ncase \"$1\" in\nstatus) exit 1 ;;\ninit) exit 0 ;;\n*) exit 0 ;;\nesac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := makeIssueOpsCLIRepoForTest(t, "example")
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "1-demo")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatal(err)
	}
	id := record["id"].(string)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-demo", "--base-branch", "main", "--link-verified", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", worktree, "--json"})
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	var prepared map[string]any
	if err := json.Unmarshal([]byte(out), &prepared); err != nil {
		t.Fatalf("prepare-tools should return JSON: %v\n%s", err, out)
	}
	if prepared["codegraph_ready"] != true || prepared["codegraph_project_path"] != worktree {
		t.Fatalf("unexpected prepare-tools result: %#v", prepared)
	}
	if want := "export HARNESS_EXPECTED_WORKTREE=" + worktree; prepared["guidance"] != want {
		t.Fatalf("expected prepare-tools guidance %q, got %#v", want, prepared["guidance"])
	}
	statusOut := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"status", "--id", id, "--json"})
	})
	var status map[string]any
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("status should return JSON: %v\n%s", err, statusOut)
	}
	tools, ok := status["worktree_tools"].(map[string]any)
	if !ok {
		t.Fatalf("prepare-tools should persist worktree tool evidence on the IssueOps record: %#v", status)
	}
	if tools["codegraph_ready"] != true || tools["codegraph_project_path"] != worktree || tools["worktree_path"] != worktree {
		t.Fatalf("unexpected persisted worktree tool evidence: %#v", tools)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "status "+worktree) || !strings.Contains(string(log), "init -i "+worktree) {
		t.Fatalf("codegraph should be checked and initialized against worktree, got:\n%s", log)
	}
}

func TestRunIssueOpsWorktreePrepareToolsFailureWithJSONEmitsStructuredError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "prepare-tools-json-failure")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	if err == nil {
		t.Fatalf("prepare-tools without linked worktree should still return an error")
	}
	var failure map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &failure); unmarshalErr != nil {
		t.Fatalf("prepare-tools failure with --json should emit JSON stdout: %v\n%s", unmarshalErr, out)
	}
	errorText, _ := failure["error"].(string)
	if failure["ok"] != false || !strings.Contains(errorText, "worktree_path is required") {
		t.Fatalf("unexpected structured failure payload: %#v", failure)
	}
}

func TestRunIssueOpsWorktreePrepareToolsInstallsPnpmDependencies(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tools.log")
	codegraph := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nprintf 'codegraph %s\\n' \"$*\" >> '"+logPath+"'\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pnpm := filepath.Join(bin, "pnpm")
	if err := os.WriteFile(pnpm, []byte("#!/bin/sh\nprintf 'pnpm cwd=%s args=%s\\n' \"$PWD\" \"$*\" >> '"+logPath+"'\nmkdir -p node_modules\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := makeIssueOpsCLIRepoForTest(t, "example")
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "1-demo")
	if err := os.WriteFile(filepath.Join(worktree, "package.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatal(err)
	}
	id := record["id"].(string)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-demo", "--base-branch", "main", "--link-verified", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", worktree, "--json"})
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	var prepared map[string]any
	if err := json.Unmarshal([]byte(out), &prepared); err != nil {
		t.Fatalf("prepare-tools should return JSON: %v\n%s", err, out)
	}
	if prepared["package_manager"] != "pnpm" || prepared["dependencies_ready"] != true || prepared["dependencies_action"] != "pnpm_install" {
		t.Fatalf("prepare-tools should install pnpm dependencies: %#v", prepared)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	if !strings.Contains(text, "pnpm cwd="+worktree+" args=install --frozen-lockfile --prefer-offline") {
		t.Fatalf("pnpm install should run in worktree, got:\n%s", text)
	}
	if !strings.Contains(text, "codegraph status "+worktree) {
		t.Fatalf("codegraph should still be checked, got:\n%s", text)
	}
}
