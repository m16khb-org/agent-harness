package hostprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

type childSmokeSurfaceDigest struct {
	Host           string `json:"host"`
	Surface        string `json:"surface"`
	SemanticSHA256 string `json:"semantic_sha256"`
	SHA256         string `json:"sha256"`
}

type childSmokeActivationDigest struct {
	RootSHA256   string                    `json:"root_sha256"`
	BinarySHA256 string                    `json:"binary_sha256"`
	Surfaces     []childSmokeSurfaceDigest `json:"surfaces"`
}

type childSmokeHostEvidence struct {
	Version              string `json:"version"`
	SessionStartObserved bool   `json:"session_start_observed"`
	PreToolUseObserved   bool   `json:"pre_tool_use_observed"`
	MCPCallCount         int    `json:"mcp_call_count"`
	ResponseSHA256       string `json:"response_sha256"`
	ExitCode             int    `json:"exit_code"`
	DurationMS           int64  `json:"duration_ms"`
}

type childSmokeReceipt struct {
	SchemaVersion         int                        `json:"schema_version"`
	Issue                 int                        `json:"issue"`
	LocalHead             string                     `json:"local_head"`
	RemoteHead            string                     `json:"remote_head"`
	ChildBinarySHA256     string                     `json:"child_binary_sha256"`
	Before                childSmokeActivationDigest `json:"before"`
	Activated             childSmokeActivationDigest `json:"activated"`
	ActivatedRootSHA256   string                     `json:"activated_root_sha256"`
	ActivatedBinarySHA256 string                     `json:"activated_binary_sha256"`
	Codex                 childSmokeHostEvidence     `json:"codex"`
	Claude                childSmokeHostEvidence     `json:"claude"`
	Restore               childSmokeActivationDigest `json:"restore"`
	Verdict               string                     `json:"verdict"`
}

type childSmokeFixture struct {
	scenario   string
	localHead  string
	remoteHead string
	confirm    bool
}

type childSmokeRun struct {
	ExitCode           int
	Receipt            childSmokeReceipt
	ReceiptMode        os.FileMode
	RestoreCalls       int
	AfterMutationCalls int
	LockExists         bool
	Output             string
}

func TestChildHostSmokeParsesBoundedNativeEvents(t *testing.T) {
	for _, test := range []struct {
		name      string
		wantHooks bool
	}{{name: "codex-stream.jsonl"}, {name: "claude-stream.jsonl", wantHooks: true}} {
		raw, err := os.ReadFile(filepath.Join("testdata", "child-host-smoke", test.name))
		if err != nil {
			t.Fatal(err)
		}
		observed, err := observeHostStream(raw)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if observed.SessionStartObserved != test.wantHooks || observed.PreToolUseObserved != test.wantHooks || observed.MCPCallCount != 1 || len(observed.ResponseSHA256) != 64 {
			t.Fatalf("%s observation=%+v", test.name, observed)
		}
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "child-host-smoke", "invalid-stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := observeHostStream(raw); err == nil {
		t.Fatal("malformed stream was accepted")
	}
}

func TestChildHostSmokeModePersistsOnlyBoundedCodexObservation(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(root, "codex-observation.json")
	stream, err := os.ReadFile(filepath.Join("testdata", "child-host-smoke", "codex-stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	process := &codexFakeProcess{run: func(_ context.Context, request CommandRequest) (CommandOutput, error) {
		for _, arg := range request.Argv {
			if arg == "--ignore-user-config" {
				t.Fatalf("child smoke suppressed user hooks: %q", request.Argv)
			}
		}
		wantEnv := []string{
			"CODEX_HOME=/private/codex",
			"HARNESS_CHILD_SMOKE_HOOKS=1",
			"HARNESS_CHILD_SMOKE_OBSERVATION_FILE=" + observationPath,
			"PATH=/opt/bin",
		}
		if !reflect.DeepEqual(request.Env, wantEnv) {
			t.Fatalf("env=%q want=%q", request.Env, wantEnv)
		}
		writeCodexCapture(t, filepath.Join(request.Cwd, "result.json"), "run-token")
		writeChildSmokeHookMarkers(t, observationPath)
		return CommandOutput{Stdout: mcpOnlyHostStream(t, stream)}, nil
	}}
	environment := map[string]string{
		"HARNESS_CHILD_SMOKE_HOOKS":            "1",
		"HARNESS_CHILD_SMOKE_OBSERVATION_FILE": observationPath,
	}
	runner := NewCodexRunner("harness", Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return "/opt/bin/codex", nil },
		Getenv:   func(name string) string { return environment[name] },
		Environ: func() []string {
			return []string{"PATH=/opt/bin", "CODEX_HOME=/private/codex", "HARNESS_CHILD_SMOKE_HOOKS=1", "HARNESS_CHILD_SMOKE_OBSERVATION_FILE=" + observationPath, "SECRET=redacted"}
		},
	})
	result := runner.Run(context.Background(), codexRequest("default"))
	assertChildSmokeObservation(t, result, observationPath)
}

func TestChildHostSmokeModePersistsOnlyBoundedClaudeObservation(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(root, "claude-observation.json")
	stream, err := os.ReadFile(filepath.Join("testdata", "child-host-smoke", "claude-stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	request := claudeProbeRequest()
	environment := map[string]string{
		"HARNESS_CHILD_SMOKE_HOOKS":            "1",
		"HARNESS_CHILD_SMOKE_OBSERVATION_FILE": observationPath,
	}
	runner := NewClaudeRunner("harness", Dependencies{
		Process: claudeCommandRunner{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
			if got := argumentValue(t, command.Argv, "--setting-sources"); got != "user" {
				t.Fatalf("setting sources=%q want user", got)
			}
			writeClaudeCapture(t, filepath.Join(command.Cwd, "result.json"), request.RunToken)
			writeChildSmokeHookMarkers(t, observationPath)
			return CommandOutput{Stdout: mcpOnlyHostStream(t, stream)}, nil
		}},
		LookPath: func(string) (string, error) { return "/opt/bin/claude", nil },
		Getenv:   func(name string) string { return environment[name] },
		Environ: func() []string {
			return []string{"PATH=/opt/bin", "HARNESS_CHILD_SMOKE_HOOKS=1", "HARNESS_CHILD_SMOKE_OBSERVATION_FILE=" + observationPath, "SECRET=redacted"}
		},
	})
	result := runner.Run(context.Background(), request)
	assertChildSmokeObservation(t, result, observationPath)
}

func TestChildHostSmokeRejectsSymlinkedHookMarker(t *testing.T) {
	root := t.TempDir()
	observationPath := filepath.Join(root, "observation.json")
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("{\"event\":\"SessionStart\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, observationPath+".hooks"); err != nil {
		t.Fatal(err)
	}
	_, err := observeRecordedHookEvents(Dependencies{Getenv: func(name string) string {
		if name == "HARNESS_CHILD_SMOKE_OBSERVATION_FILE" {
			return observationPath
		}
		return ""
	}})
	if err == nil {
		t.Fatal("symlinked hook marker was accepted")
	}
}

func writeChildSmokeHookMarkers(t *testing.T, observationPath string) {
	t.Helper()
	data := []byte("{\"event\":\"SessionStart\"}\n{\"event\":\"PreToolUse\"}\n")
	if err := os.WriteFile(observationPath+".hooks", data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mcpOnlyHostStream(t *testing.T, stream []byte) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(stream))
	var output bytes.Buffer
	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		typeName, _ := event["type"].(string)
		if typeName != "item.completed" && typeName != "tool_result" && typeName != "assistant" && typeName != "user" {
			continue
		}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(data)
		output.WriteByte('\n')
	}
	return output.Bytes()
}

func assertChildSmokeObservation(t *testing.T, result port.HostProbeResult, path string) {
	t.Helper()
	if !result.Completed || !result.SessionStartObserved || !result.PreToolUseObserved || len(result.ResponseSHA256) != 64 || result.ExitCode != 0 {
		t.Fatalf("result=%+v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var observation hostStreamObservation
	if err := decodeStrictJSON(data, &observation); err != nil {
		t.Fatal(err)
	}
	if !observation.SessionStartObserved || !observation.PreToolUseObserved || observation.MCPCallCount != 1 || observation.ResponseSHA256 != result.ResponseSHA256 {
		t.Fatalf("observation=%+v", observation)
	}
	if strings.Contains(string(data), "captured") || strings.Contains(string(data), "agent_harness_probe") {
		t.Fatalf("observation retained native transcript: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("observation mode=%v err=%v", info, err)
	}
}

func TestChildHostSmokeCompleteFakeTwoHostPass(t *testing.T) {
	result := runChildHostSmokeFixture(t, childSmokeFixture{confirm: true})
	if result.ExitCode != 0 || result.Receipt.Verdict != "pass" || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 || result.LockExists {
		t.Fatalf("result=%+v output=%s", result, result.Output)
	}
	if result.ReceiptMode.Perm() != 0o600 || !reflect.DeepEqual(result.Receipt.Before, result.Receipt.Restore) {
		t.Fatalf("private exact restore drift: mode=%#o before=%+v restore=%+v", result.ReceiptMode.Perm(), result.Receipt.Before, result.Receipt.Restore)
	}
	if result.Receipt.ActivatedRootSHA256 != result.Receipt.Activated.RootSHA256 || result.Receipt.ActivatedBinarySHA256 != result.Receipt.ChildBinarySHA256 || result.Receipt.Activated.BinarySHA256 != result.Receipt.ChildBinarySHA256 {
		t.Fatalf("activated identity drift: %+v", result.Receipt)
	}
	for _, host := range []childSmokeHostEvidence{result.Receipt.Codex, result.Receipt.Claude} {
		if !host.SessionStartObserved || !host.PreToolUseObserved || host.MCPCallCount != 1 || len(host.ResponseSHA256) != 64 || host.ExitCode != 0 {
			t.Fatalf("host evidence=%+v", host)
		}
	}
}

func TestChildHostSmokeAlwaysRestoresBeforeReturningFailure(t *testing.T) {
	result := runChildHostSmokeFixture(t, childSmokeFixture{scenario: "claude-session-failure", confirm: true})
	if result.ExitCode == 0 || result.Receipt.Verdict != "fail" || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 || result.LockExists {
		t.Fatalf("unsafe failure receipt: %+v output=%s", result, result.Output)
	}
}

func TestChildHostSmokeFailsClosedOnPostActivationDrift(t *testing.T) {
	for _, scenario := range []string{
		"codex-version-drift",
		"claude-version-drift",
		"codex-session-start-missing",
		"claude-pre-tool-use-missing",
		"codex-mcp-zero",
		"claude-mcp-two",
		"codex-mcp-readback-mismatch",
		"claude-mcp-readback-mismatch",
		"codex-mcp-output-over-limit",
		"codex-observation-extra-field",
		"activated-digest-missing",
		"activated-digest-blank",
		"activated-binary-mismatch",
		"activation-failure",
		"restore-failure",
		"signal-during-codex",
		"signal-during-restore",
	} {
		t.Run(scenario, func(t *testing.T) {
			result := runChildHostSmokeFixture(t, childSmokeFixture{scenario: scenario, confirm: true})
			if result.ExitCode == 0 || result.Receipt.Verdict != "fail" || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 || result.LockExists {
				t.Fatalf("scenario=%s result=%+v output=%s", scenario, result, result.Output)
			}
		})
	}
}

func TestChildHostSmokeRepairsInstallerRawDigestDrift(t *testing.T) {
	result := runChildHostSmokeFixture(t, childSmokeFixture{scenario: "restore-raw-digest-drift", confirm: true})
	if result.ExitCode != 0 || result.Receipt.Verdict != "pass" || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 || result.LockExists {
		t.Fatalf("raw restore repair failed: result=%+v output=%s", result, result.Output)
	}
	if !reflect.DeepEqual(result.Receipt.Before, result.Receipt.Restore) {
		t.Fatalf("raw restore drift remained: before=%+v restore=%+v", result.Receipt.Before, result.Receipt.Restore)
	}
}

func TestChildHostSmokeRecreatesMissingConfigAfterRestoreFailure(t *testing.T) {
	result := runChildHostSmokeFixture(t, childSmokeFixture{scenario: "restore-missing-file", confirm: true})
	if result.ExitCode == 0 || result.Receipt.Verdict != "fail" || result.RestoreCalls != 1 || result.AfterMutationCalls != 0 || result.LockExists {
		t.Fatalf("missing-file recovery did not fail closed: result=%+v output=%s", result, result.Output)
	}
	if !reflect.DeepEqual(result.Receipt.Before, result.Receipt.Restore) {
		t.Fatalf("missing config was not restored exactly: before=%+v restore=%+v", result.Receipt.Before, result.Receipt.Restore)
	}
}

func TestChildHostSmokeRejectsAuthorityAndConfirmationBeforeMutation(t *testing.T) {
	head := strings.Repeat("1", 40)
	for _, fixture := range []childSmokeFixture{{confirm: false}, {confirm: true, localHead: strings.Repeat("2", 40)}, {confirm: true, remoteHead: strings.Repeat("2", 40)}, {scenario: "source-activation-mismatch", confirm: true}} {
		result := runChildHostSmokeFixture(t, fixture)
		if result.ExitCode == 0 || result.Receipt.Verdict != "fail" || result.ReceiptMode.Perm() != 0o600 || result.RestoreCalls != 0 || result.AfterMutationCalls != 0 || result.LockExists {
			t.Fatalf("unsafe authority result=%+v output=%s", result, result.Output)
		}
		if fixture.remoteHead != "" && fixture.remoteHead == head {
			t.Fatal("test fixture did not drift")
		}
	}
}

func runChildHostSmokeFixture(t *testing.T, fixture childSmokeFixture) childSmokeRun {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot := filepath.Join(root, "source")
	childRoot := filepath.Join(root, "child")
	home := filepath.Join(root, "home")
	codexHome := filepath.Join(home, ".codex")
	stateRoot := filepath.Join(root, "state")
	outputRoot := filepath.Join(root, "private")
	fakeBin := filepath.Join(root, "fake-bin")
	for _, dir := range []string{sourceRoot, childRoot, home, codexHome, stateRoot, outputRoot, fakeBin} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{sourceRoot, childRoot} {
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeManagedSurfaceFixture(t, home, codexHome, sourceRoot)
	if fixture.scenario == "source-activation-mismatch" {
		writeManagedSurfaceFixture(t, home, codexHome, sourceRoot+"x")
	}
	writeExecutable(t, filepath.Join(sourceRoot, "bin", "agent-harness"), "#!/usr/bin/env bash\nprintf 'source-version\\n'\n")
	template := filepath.Join(root, "child-binary-template")
	writeExecutable(t, template, fakeChildBinaryScript)
	writeExecutable(t, filepath.Join(sourceRoot, "scripts", "install-native.sh"), fakeInstallScript("source"))
	writeExecutable(t, filepath.Join(childRoot, "scripts", "install-native.sh"), fakeInstallScript("child"))
	writeExecutable(t, filepath.Join(fakeBin, "go"), fakeGoScript)
	writeExecutable(t, filepath.Join(fakeBin, "git"), fakeGitScript)
	writeExecutable(t, filepath.Join(fakeBin, "python3"), fakePythonScript)
	writeExecutable(t, filepath.Join(fakeBin, "codex"), fakeHostScript("codex"))
	writeExecutable(t, filepath.Join(fakeBin, "claude"), fakeHostScript("claude"))

	head := strings.Repeat("1", 40)
	localHead := fixture.localHead
	if localHead == "" {
		localHead = head
	}
	remoteHead := fixture.remoteHead
	if remoteHead == "" {
		remoteHead = head
	}
	logPath := filepath.Join(root, "calls.log")
	outputPath := filepath.Join(outputRoot, "receipt.json")
	script := filepath.Join(findHostSmokeRepoRoot(t), "scripts", "verify-child-host-smoke.sh")
	args := []string{
		script,
		"--issue", "230",
		"--source-root", sourceRoot,
		"--child-root", childRoot,
		"--head", head,
		"--remote-ref", "refs/heads/230-child-smoke",
		"--json-out", outputPath,
	}
	if fixture.confirm {
		args = append(args, "--confirm-user-activation")
	}
	command := exec.Command("/bin/bash", args...)
	command.Dir = childRoot
	command.Env = append(os.Environ(),
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"HOME="+home,
		"CODEX_HOME="+codexHome,
		"HARNESS_STATE_DIR="+stateRoot,
		"FAKE_CHILD_BINARY_TEMPLATE="+template,
		"FAKE_HEAD="+localHead,
		"FAKE_REMOTE_HEAD="+remoteHead,
		"FAKE_SCENARIO="+fixture.scenario,
		"FAKE_STATE_ROOT="+stateRoot,
		"FAKE_CALL_LOG="+logPath,
		"REAL_PYTHON="+mustLookPath(t, "python3"),
	)
	output, runErr := command.CombinedOutput()
	result := childSmokeRun{Output: string(output)}
	if runErr != nil {
		if exit, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exit.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	if data, err := os.ReadFile(outputPath); err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result.Receipt); err != nil {
			t.Fatalf("decode receipt: %v\n%s\noutput=%s", err, data, output)
		}
		if info, err := os.Lstat(outputPath); err == nil {
			result.ReceiptMode = info.Mode()
		}
	}
	if calls, err := os.ReadFile(logPath); err == nil {
		for _, call := range strings.Split(string(calls), "\n") {
			switch call {
			case "source_restore":
				result.RestoreCalls++
			case "merge", "cleanup":
				result.AfterMutationCalls++
			}
		}
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "child-host-smoke.lock")); err == nil {
		result.LockExists = true
	}
	return result
}

func mustLookPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func findHostSmokeRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}

func writeManagedSurfaceFixture(t *testing.T, home, codexHome, root string) {
	t.Helper()
	binary := filepath.Join(root, "bin", "agent-harness")
	files := map[string]string{
		filepath.Join(codexHome, "config.toml"): fmt.Sprintf("[mcp_servers.agent_harness]\ncommand = %q\nargs = [\"mcp\"]\n[mcp_servers.agent_harness.env]\nHARNESS_ROOT = %q\n", binary, root),
		filepath.Join(codexHome, "hooks.json"): fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":%q}]}],"PreToolUse":[{"hooks":[{"command":%q}]}]}}
`, "'"+binary+"' hook session-start", "'"+binary+"' hook pre-tool-use"),
		filepath.Join(home, ".claude.json"): fmt.Sprintf(`{"mcpServers":{"agent_harness":{"type":"stdio","command":%q,"args":["mcp"],"env":{"HARNESS_ROOT":%q}}}}
`, binary, root),
		filepath.Join(home, ".claude", "settings.json"): fmt.Sprintf(`{"hooks":{"SessionStart":[{"hooks":[{"command":%q}]}],"PreToolUse":[{"matcher":"*","hooks":[{"command":%q}]}]}}
`, "'"+binary+"' hook session-start", "'"+binary+"' hook pre-tool-use"),
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func fakeInstallScript(identity string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
identity=%q
if [[ %q == source ]]; then
  printf 'source_restore\n' >>"$FAKE_CALL_LOG"
  if [[ "${FAKE_SCENARIO:-}" == signal-during-restore ]]; then kill -TERM "$PPID"; fi
  [[ "${FAKE_SCENARIO:-}" != restore-failure ]] || exit 19
else
  printf 'child_activate\n' >>"$FAKE_CALL_LOG"
  [[ "${FAKE_SCENARIO:-}" != activation-failure ]] || exit 17
fi
mkdir -p "$CODEX_HOME" "$HOME/.claude"
python3 - "$HARNESS_ROOT" "$HOME" "$CODEX_HOME" <<'PY'
import json
import os
import sys

root, home, codex_home = sys.argv[1:]
binary = os.path.join(root, "bin", "agent-harness")
with open(os.path.join(codex_home, "config.toml"), "w", encoding="utf-8") as handle:
    handle.write(f'[mcp_servers.agent_harness]\ncommand = "{binary}"\nargs = ["mcp"]\n[mcp_servers.agent_harness.env]\nHARNESS_ROOT = "{root}"\n')
hooks = {"hooks": {
    "SessionStart": [{"hooks": [{"command": f"'{binary}' hook session-start"}]}],
    "PreToolUse": [{"hooks": [{"command": f"'{binary}' hook pre-tool-use"}]}],
}}
with open(os.path.join(codex_home, "hooks.json"), "w", encoding="utf-8") as handle:
    json.dump(hooks, handle, separators=(",", ":"))
    handle.write("\n")
with open(os.path.join(home, ".claude.json"), "w", encoding="utf-8") as handle:
    json.dump({"mcpServers": {"agent_harness": {"type": "stdio", "command": binary, "args": ["mcp"], "env": {"HARNESS_ROOT": root}}}}, handle, separators=(",", ":"))
    handle.write("\n")
claude_hooks = {"hooks": {
    "SessionStart": [{"hooks": [{"command": f"'{binary}' hook session-start"}]}],
    "PreToolUse": [{"matcher": "*", "hooks": [{"command": f"'{binary}' hook pre-tool-use"}]}],
}}
with open(os.path.join(home, ".claude", "settings.json"), "w", encoding="utf-8") as handle:
    json.dump(claude_hooks, handle, separators=(",", ":"))
    handle.write("\n")
PY
chmod 0600 "$CODEX_HOME/config.toml" "$CODEX_HOME/hooks.json" "$HOME/.claude.json" "$HOME/.claude/settings.json"
if [[ %q == source && "${FAKE_SCENARIO:-}" == restore-missing-file ]]; then
  rm "$CODEX_HOME/hooks.json"
  exit 20
fi
if [[ %q == source && "${FAKE_SCENARIO:-}" == restore-raw-digest-drift ]]; then
  printf ' ' >>"$CODEX_HOME/config.toml"
fi
if [[ %q == child && "${FAKE_SCENARIO:-}" == activated-digest-missing ]]; then
  rm -f "$CODEX_HOME/hooks.json"
fi
if [[ %q == child && "${FAKE_SCENARIO:-}" == activated-binary-mismatch ]]; then
  printf 'drift\n' >>"$HARNESS_ROOT/bin/agent-harness"
fi
printf '{"ok":true}\n'
`, identity, identity, identity, identity, identity, identity)
}

func fakeHostScript(host string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
host=%q
if [[ "${1:-}" == --version ]]; then
  count_file="$FAKE_STATE_ROOT/%s-version-count"
  count=0
  [[ ! -f "$count_file" ]] || count="$(cat "$count_file")"
  count=$((count + 1))
  printf '%%s' "$count" >"$count_file"
  if [[ "${FAKE_SCENARIO:-}" == %s-version-drift && "$count" -gt 1 ]]; then
    printf '%s-v2\n'
  else
    printf '%s-v1\n'
  fi
  exit 0
fi
if [[ "${1:-}" == mcp && "${2:-}" == get && "${3:-}" == agent_harness ]]; then
  root="$HARNESS_ROOT"
  if [[ "${FAKE_SCENARIO:-}" == "$host-mcp-output-over-limit" && "$root" == */child ]]; then
    "$REAL_PYTHON" -c 'import sys; sys.stdout.write("x" * 70000)'
    exit 0
  fi
  if [[ "${FAKE_SCENARIO:-}" == "$host-mcp-readback-mismatch" && "$root" == */child ]]; then
    root="${root}x"
  fi
  if [[ "$host" == codex ]]; then
    printf '{"name":"agent_harness","enabled":true,"transport":{"type":"stdio","command":"%%s/bin/agent-harness","args":["mcp"],"env":{"HARNESS_ROOT":"%%s"}}}\n' "$root" "$root"
  else
    printf 'agent_harness:\n  Scope: User config (available in all your projects)\n  Status: ✔ Connected\n  Type: stdio\n  Command: %%s/bin/agent-harness\n  Args: mcp\n  Environment:\n    HARNESS_ROOT=%%s\n' "$root" "$root"
  fi
  exit 0
fi
exit 2
`, host, host, host, host, host)
}

const fakeGoScript = `#!/usr/bin/env bash
set -euo pipefail
output=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == -o ]]; then
    output="$2"
    shift 2
    continue
  fi
  shift
done
[[ -n "$output" ]]
cp "$FAKE_CHILD_BINARY_TEMPLATE" "$output"
chmod 0700 "$output"
`

const fakePythonScript = `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != - ]]; then
  exec "$REAL_PYTHON" "$@"
fi
program="$(mktemp)"
trap 'rm -f "$program"' EXIT
sed -n '1,$p' >"$program"
"$REAL_PYTHON" "$program" "${@:2}"
if [[ "${FAKE_SCENARIO:-}" == activated-digest-blank && "${3:-}" == */activated.json ]]; then
  "$REAL_PYTHON" - "$3" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as handle:
    value = json.load(handle)
value["binary_sha256"] = ""
with open(path, "w", encoding="utf-8") as handle:
    json.dump(value, handle, sort_keys=True, separators=(",", ":"))
    handle.write("\n")
PY
fi
`

const fakeGitScript = `#!/usr/bin/env bash
set -euo pipefail
args=" $* "
if [[ "$args" == *" status --porcelain "* ]]; then
  exit 0
fi
if [[ "$args" == *" rev-parse HEAD "* ]]; then
  printf '%s\n' "$FAKE_HEAD"
  exit 0
fi
if [[ "$args" == *" ls-remote "* ]]; then
  ref="${*: -1}"
  printf '%s\t%s\n' "$FAKE_REMOTE_HEAD" "$ref"
  exit 0
fi
exit 2
`

const fakeChildBinaryScript = `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version)
    printf 'child-version\n'
    ;;
  install)
    printf '{"ok":true,"root":"%s","dry_run":true,"project_local":true,"hosts":[{"host":"codex","ok":true,"dry_run":true},{"host":"claude","ok":true,"dry_run":true}],"files":[],"links":[]}\n' "$HARNESS_ROOT"
    ;;
  contract)
    host=""
    previous=""
    for arg in "$@"; do
      if [[ "$previous" == --hosts ]]; then host="$arg"; fi
      previous="$arg"
    done
    [[ -n "$host" && -n "${HARNESS_CHILD_SMOKE_OBSERVATION_FILE:-}" ]]
    if [[ "$host" == codex ]]; then
      hook_config="$CODEX_HOME/hooks.json"
    else
      hook_config="$HOME/.claude/settings.json"
    fi
    grep -Fq 'HARNESS_CHILD_SMOKE_HOOKS=1' "$hook_config"
    grep -Fq "$HARNESS_CHILD_SMOKE_OBSERVATION_FILE" "$hook_config"
    [[ "${FAKE_SCENARIO:-}" != "$host-session-failure" ]] || exit 9
    if [[ "${FAKE_SCENARIO:-}" == signal-during-codex && "$host" == codex ]]; then
      kill -TERM "$PPID"
      exit 12
    fi
    session=true
    pretool=true
    calls=1
    [[ "${FAKE_SCENARIO:-}" != "$host-session-start-missing" ]] || session=false
    [[ "${FAKE_SCENARIO:-}" != "$host-pre-tool-use-missing" ]] || pretool=false
    [[ "${FAKE_SCENARIO:-}" != "$host-mcp-zero" ]] || calls=0
    [[ "${FAKE_SCENARIO:-}" != "$host-mcp-two" ]] || calls=2
    if [[ "${FAKE_SCENARIO:-}" == "$host-observation-extra-field" ]]; then
      printf '{"session_start_observed":%s,"pre_tool_use_observed":%s,"mcp_call_count":%s,"response_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","exit_code":0,"duration_ms":1,"transcript":"secret-material"}\n' "$session" "$pretool" "$calls" >"$HARNESS_CHILD_SMOKE_OBSERVATION_FILE"
    else
      printf '{"session_start_observed":%s,"pre_tool_use_observed":%s,"mcp_call_count":%s,"response_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","exit_code":0,"duration_ms":1}\n' "$session" "$pretool" "$calls" >"$HARNESS_CHILD_SMOKE_OBSERVATION_FILE"
    fi
    chmod 0600 "$HARNESS_CHILD_SMOKE_OBSERVATION_FILE"
    printf '{"ok":true}\n'
    ;;
  *)
    exit 2
    ;;
esac
`
