package statuscli

import (
	"encoding/json"
	"issueops/internal/adapter/preflight"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/daemoncli"
	statestore "issueops/internal/adapter/outbound/state"
	worker "issueops/internal/adapter/worker"
	inspect "issueops/internal/contract/inspect"
	"issueops/internal/testsupport"
)

func TestBuildHarnessStatusReportsStateWorkerAndSelfVerify(t *testing.T) {
	repo := t.TempDir()
	stateDir := t.TempDir()
	workerDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	t.Setenv("ISSUEOPS_WORKER_DIR", workerDir)
	if _, err := statestore.StateWrite("self-verify-latest", `{"ok":true}`); err != nil {
		t.Fatalf("write self verify state: %v", err)
	}
	if _, err := worker.EnqueueWorkerJob("smoke", "payload"); err != nil {
		t.Fatalf("enqueue worker job: %v", err)
	}

	status := BuildStatus(repo)

	if status.Kind != "harness_status" || status.Repo != repo {
		t.Fatalf("unexpected status identity: %#v", status)
	}
	if !status.State.OK || len(status.State.Records) != 1 {
		t.Fatalf("expected isolated state record, got %#v", status.State)
	}
	if !status.Workers.OK || len(status.Workers.Jobs) != 1 {
		t.Fatalf("expected isolated worker job, got %#v", status.Workers)
	}
	if !status.SelfVerify.Found || status.SelfVerify.LatestKey != "self-verify-latest" || status.SelfVerify.Bytes == 0 {
		t.Fatalf("expected self verify latest state, got %#v", status.SelfVerify)
	}
	if status.Daemon.Message == "" {
		t.Fatalf("daemon status should include an operator-facing message")
	}
}

func TestBuildHarnessStatusSharesDaemonAdmissionWithDoctor(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	oldDeps := deps
	t.Cleanup(func() { Configure(oldDeps) })
	want := daemoncli.Status{
		Running:           true,
		Reachable:         true,
		IdentityVerified:  true,
		ActiveConnections: 64,
		MaxConnections:    64,
		Accepting:         false,
		Draining:          false,
	}
	Configure(Deps{
		GitPreflight:      preflight.GitPreflight,
		IssueOpsRoot:      func() string { return repo },
		ResolveTarget:     func(target string) string { return target },
		Version:           "test",
		InspectHarness:    func(string) inspect.InspectInfo { return inspect.InspectInfo{} },
		CheckDaemonStatus: func() daemoncli.Status { return want },
	})

	status := BuildStatus(repo)
	if status.Daemon != want {
		t.Fatalf("unexpected daemon status: %#v", status.Daemon)
	}
	if status.Doctor.ActiveConnections != 64 || status.Doctor.MaxConnections != 64 || status.Doctor.Accepting || status.Doctor.Draining {
		t.Fatalf("status doctor drifted from daemon admission: doctor=%#v daemon=%#v", status.Doctor, status.Daemon)
	}
	for _, issue := range status.Doctor.Issues {
		if issue.Code == "daemon_connection_limit_reached" {
			return
		}
	}
	t.Fatalf("status doctor did not evaluate saturated daemon admission: %+v", status.Doctor)
}

func TestRunStatusWritesTextAndJSON(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())

	text := captureStatusVerifyStdout(t, func() error {
		return RunStatus([]string{"--repo", repo})
	})
	if !strings.Contains(text, "issueops system-status:") || !strings.Contains(text, "daemon running:") {
		t.Fatalf("unexpected status text output:\n%s", text)
	}

	jsonText := captureStatusVerifyStdout(t, func() error {
		return RunStatus([]string{"--repo", repo, "--json"})
	})
	var decoded Status
	if err := json.Unmarshal([]byte(jsonText), &decoded); err != nil {
		t.Fatalf("decode status JSON: %v\n%s", err, jsonText)
	}
	if decoded.Kind != "harness_status" || decoded.Repo != repo {
		t.Fatalf("unexpected status JSON payload: %#v", decoded)
	}
}

func TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/verifywork\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	result := BuildVerifyWork(repo, false, []string{"git", "status", "--short"})
	if !result.OK {
		t.Fatalf("expected verify-work result to be ok, warnings=%v", result.Warnings)
	}

	assertEvidenceItem(t, result.EvidenceMatrix, "git_preflight", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "guard_check", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "passed")

	if len(result.Evidence) == 0 {
		t.Fatalf("expected legacy evidence strings to remain populated")
	}
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "test", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "build", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "vet", "./..."})
}

func TestBuildVerifyWorkSerializesEmptySuggestedCommands(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := BuildVerifyWork(repo, false, nil)
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "skipped")
	if len(result.SuggestedCommands) != 0 {
		t.Fatalf("expected no suggested commands without project signals, got %#v", result.SuggestedCommands)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal verify-work result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal verify-work payload: %v", err)
	}
	commands, ok := decoded["suggested_commands"].([]any)
	if !ok {
		t.Fatalf("expected suggested_commands to serialize as an array, payload=%s", string(payload))
	}
	if len(commands) != 0 {
		t.Fatalf("expected empty suggested_commands array, got %#v", commands)
	}
}

func TestBuildVerifyWorkMarksDeniedCommandFailed(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	result := BuildVerifyWork(repo, false, []string{"sh", "-c", "true"})
	if result.OK {
		t.Fatalf("expected denied command to fail verify-work")
	}
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "failed")
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func assertEvidenceItem(t *testing.T, items []WorkEvidenceItem, name string, status string) {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			if item.Status != status {
				t.Fatalf("evidence item %s status=%s, want %s", name, item.Status, status)
			}
			if item.Summary == "" {
				t.Fatalf("evidence item %s has empty summary", name)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %s in %#v", name, items)
}

func assertSuggestedCommand(t *testing.T, commands []WorkSuggestedCommand, want []string) {
	t.Helper()
	for _, command := range commands {
		if equalStringSlices(command.Command, want) {
			if command.Name == "" || command.Reason == "" {
				t.Fatalf("suggested command %#v must include name and reason", command)
			}
			return
		}
	}
	t.Fatalf("missing suggested command %v in %#v", want, commands)
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}
