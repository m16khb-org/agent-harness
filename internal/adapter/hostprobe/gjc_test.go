package hostprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/toolconformance"
	"agent-harness/internal/port"
)

type fakeGJCProcess struct {
	requests []CommandRequest
	run      func(CommandRequest) (CommandOutput, error)
}

func (f *fakeGJCProcess) Run(_ context.Context, request CommandRequest) (CommandOutput, error) {
	request.Argv = append([]string(nil), request.Argv...)
	request.Env = append([]string(nil), request.Env...)
	f.requests = append(f.requests, request)
	if f.run == nil {
		return CommandOutput{}, nil
	}
	return f.run(request)
}

func TestGJCRunnerUsesOnlyIsolatedProjectAndAgentRoots(t *testing.T) {
	request := testGJCRequest()
	process := &fakeGJCProcess{}
	var episodeRoot string
	process.run = func(command CommandRequest) (CommandOutput, error) {
		switch {
		case reflect.DeepEqual(command.Argv[1:3], []string{"plugin", "install"}):
			episodeRoot = filepath.Dir(command.Cwd)
			assertPrivateDirectory(t, command.Cwd)
			assertPrivateDirectory(t, filepath.Join(episodeRoot, "agent"))
			bundleRoot := command.Argv[3]
			assertPrivateDirectory(t, bundleRoot)
			entries, err := os.ReadDir(bundleRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 {
				t.Fatalf("bundle file count = %d", len(entries))
			}
			if names := []string{entries[0].Name(), entries[1].Name()}; !reflect.DeepEqual(names, []string{"gajae-plugin.json", "launcher.ts"}) {
				t.Fatalf("bundle files = %v", names)
			}
			assertGJCBundle(t, bundleRoot, request, filepath.Join(episodeRoot, "capture.json"))
		case command.Argv[1] == "-p":
			writeGJCCapture(t, filepath.Join(filepath.Dir(command.Cwd), "capture.json"), request.RunToken)
		default:
			t.Fatalf("unexpected argv: %v", command.Argv)
		}
		return CommandOutput{}, nil
	}
	runner := NewGJCRunner("/opt/agent-harness", testGJCDependencies(process))
	result := runner.Run(context.Background(), request)
	if !result.Completed {
		t.Fatalf("result = %+v", result)
	}
	if len(process.requests) != 2 {
		t.Fatalf("process calls = %d", len(process.requests))
	}
	install, invoke := process.requests[0], process.requests[1]
	wantInstall := []string{"/usr/local/bin/gjc", "plugin", "install", filepath.Join(episodeRoot, "bundle"), "--project"}
	if !reflect.DeepEqual(install.Argv, wantInstall) {
		t.Fatalf("install argv = %q want %q", install.Argv, wantInstall)
	}
	wantInvoke := []string{
		"/usr/local/bin/gjc", "-p", "--mode=json", "--no-session", "--no-tools", "--no-lsp", "--no-skills", "--no-rules", "--model", request.Model, request.Prompt,
	}
	if !reflect.DeepEqual(invoke.Argv, wantInvoke) {
		t.Fatalf("invoke argv = %q want %q", invoke.Argv, wantInvoke)
	}
	wantEnv := []string{
		"GJC_CODING_AGENT_DIR=" + filepath.Join(episodeRoot, "agent"),
		"GJC_TOKEN=isolated-token",
		"HOME=/safe/home",
		"LANG=C.UTF-8",
		"LC_CTYPE=UTF-8",
		"PATH=/safe/bin",
		"TMPDIR=/safe/tmp",
	}
	sort.Strings(wantEnv)
	if !reflect.DeepEqual(install.Env, wantEnv) || !reflect.DeepEqual(invoke.Env, wantEnv) {
		t.Fatalf("env = %q want %q", invoke.Env, wantEnv)
	}
	if containsEnvironment(invoke.Env, "GJC_CODING_AGENT_DIR=/real-agent") || containsEnvironment(invoke.Env, "UNRELATED=secret") {
		t.Fatalf("isolated environment leaked: %q", invoke.Env)
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root was not cleaned up: %v", err)
	}
}

func TestGJCRunnerRejectsDefaultModelOrUnavailableAuthBeforeInvocation(t *testing.T) {
	for name, mutate := range map[string]func(*port.HostProbeRequest, map[string]string){
		"default_model": func(request *port.HostProbeRequest, _ map[string]string) { request.Model = "default" },
		"missing_model": func(request *port.HostProbeRequest, _ map[string]string) { request.Model = "" },
		"missing_auth":  func(_ *port.HostProbeRequest, env map[string]string) { delete(env, "GJC_TOKEN") },
	} {
		t.Run(name, func(t *testing.T) {
			request := testGJCRequest()
			env := map[string]string{"GJC_TOKEN": "isolated-token"}
			mutate(&request, env)
			process := &fakeGJCProcess{run: func(CommandRequest) (CommandOutput, error) {
				t.Fatal("process invoked")
				return CommandOutput{}, nil
			}}
			deps := testGJCDependencies(process)
			deps.Getenv = func(name string) string { return env[name] }
			result := NewGJCRunner("/opt/agent-harness", deps).Run(context.Background(), request)
			if result.Cause != "harness_environment" || result.Code != "isolated_auth_unavailable" {
				t.Fatalf("result = %+v", result)
			}
			if len(process.requests) != 0 {
				t.Fatalf("process calls = %d", len(process.requests))
			}
		})
	}
}

func TestGJCRunnerReportsMissingCaptureAndCleansUp(t *testing.T) {
	request := testGJCRequest()
	process := &fakeGJCProcess{}
	var episodeRoot string
	process.run = func(command CommandRequest) (CommandOutput, error) {
		episodeRoot = filepath.Dir(command.Cwd)
		return CommandOutput{}, nil
	}
	result := NewGJCRunner("/opt/agent-harness", testGJCDependencies(process)).Run(context.Background(), request)
	if result.Completed || result.Cause != "unknown" || result.Code != "no_call" {
		t.Fatalf("result = %+v", result)
	}
	if len(process.requests) != 2 {
		t.Fatalf("process calls = %d", len(process.requests))
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root was not cleaned up: %v", err)
	}
}

func TestGJCRunnerPreflightUsesRestrictedEnvironment(t *testing.T) {
	process := &fakeGJCProcess{run: func(CommandRequest) (CommandOutput, error) {
		return CommandOutput{Stdout: []byte("gjc 0.9.5\n")}, nil
	}}
	preflight := NewGJCRunner("/opt/agent-harness", testGJCDependencies(process)).Preflight(context.Background(), testGJCRequest())
	if !preflight.Ready || preflight.Version != "gjc 0.9.5" {
		t.Fatalf("preflight = %+v", preflight)
	}
	if len(process.requests) != 1 || !reflect.DeepEqual(process.requests[0].Argv, []string{"/usr/local/bin/gjc", "--version"}) {
		t.Fatalf("requests = %+v", process.requests)
	}
	for _, want := range []string{"HOME=/safe/home", "LANG=C.UTF-8", "LC_CTYPE=UTF-8", "PATH=/safe/bin", "TMPDIR=/safe/tmp"} {
		if !containsEnvironment(process.requests[0].Env, want) {
			t.Fatalf("preflight environment missing %q: %q", want, process.requests[0].Env)
		}
	}
	agentRoot := environmentValue(process.requests[0].Env, "GJC_CODING_AGENT_DIR")
	if agentRoot == "" || agentRoot == "/real-agent" {
		t.Fatalf("preflight agent root is not isolated: %q", process.requests[0].Env)
	}
	if _, err := os.Stat(filepath.Dir(agentRoot)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight root was not cleaned: %v", err)
	}
}

func testGJCRequest() port.HostProbeRequest {
	return port.HostProbeRequest{
		FixtureID:    "empty_object",
		SchemaSHA256: "schema",
		Prompt:       "Call the probe once.",
		Model:        "gjc-test-model",
		Profile:      "clean",
		Attempt:      1,
		RunToken:     "run-token",
		GJCAuthEnv:   []string{"GJC_TOKEN"},
	}
}

func testGJCDependencies(process CommandRunner) Dependencies {
	return Dependencies{
		Process:  process,
		LookPath: func(string) (string, error) { return "/usr/local/bin/gjc", nil },
		Now:      func() time.Time { return time.Unix(1, 0) },
		Getenv: func(name string) string {
			if name == "GJC_TOKEN" {
				return "isolated-token"
			}
			return ""
		},
		Environ: func() []string {
			return []string{
				"PATH=/safe/bin",
				"HOME=/safe/home",
				"TMPDIR=/safe/tmp",
				"LANG=C.UTF-8",
				"LC_CTYPE=UTF-8",
				"GJC_CODING_AGENT_DIR=/real-agent",
				"UNRELATED=secret",
			}
		},
	}
}

func assertPrivateDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode %s = %o", path, info.Mode().Perm())
	}
}

func assertGJCBundle(t *testing.T, bundleRoot string, request port.HostProbeRequest, resultPath string) {
	t.Helper()
	manifestData, err := os.ReadFile(filepath.Join(bundleRoot, "gajae-plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(bundleRoot, "gajae-plugin.json")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode info=%v err=%v", info, err)
	}
	var manifest gjcPluginManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.MCPs) != 1 || manifest.MCPs[0].Transport != "stdio" || !reflect.DeepEqual(manifest.MCPs[0].Args, []string{"./launcher.ts"}) {
		t.Fatalf("manifest = %+v", manifest)
	}
	launcherPath := filepath.Join(bundleRoot, "launcher.ts")
	info, err := os.Stat(launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("launcher mode = %o", info.Mode().Perm())
	}
	launcher, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	launcherText := string(launcher)
	if !strings.Contains(launcherText, "JSON.parse(") || strings.Contains(launcherText, "process.env") {
		t.Fatalf("launcher does not use embedded argv: %s", launcher)
	}
	wantServe := serveArgv(requestWithHarnessBinary(request, "/opt/agent-harness"), resultPath)
	if got := launcherArgv(t, launcherText); !reflect.DeepEqual(got, wantServe) {
		t.Fatalf("launcher argv = %q want %q", got, wantServe)
	}
}

func launcherArgv(t *testing.T, launcher string) []string {
	t.Helper()
	const prefix = "const argv = JSON.parse("
	const suffix = ") as string[];"
	start := strings.Index(launcher, prefix)
	if start < 0 {
		t.Fatalf("launcher argv declaration missing: %s", launcher)
	}
	start += len(prefix)
	end := strings.Index(launcher[start:], suffix)
	if end < 0 {
		t.Fatalf("launcher argv declaration is malformed: %s", launcher)
	}
	var encoded string
	if err := json.Unmarshal([]byte(launcher[start:start+end]), &encoded); err != nil {
		t.Fatal(err)
	}
	var argv []string
	if err := json.Unmarshal([]byte(encoded), &argv); err != nil {
		t.Fatal(err)
	}
	return argv
}

func writeGJCCapture(t *testing.T, path, runToken string) {
	t.Helper()
	tokenSum := sha256.Sum256([]byte(runToken))
	rawSum := sha256.Sum256([]byte(`{}`))
	capture := episodeCapture{
		FixtureID:          "empty_object",
		CallCount:          1,
		RawSHA256:          hex.EncodeToString(rawSum[:]),
		CanonicalArguments: map[string]any{},
		SchemaSHA256:       "schema",
		RunTokenSHA256:     hex.EncodeToString(tokenSum[:]),
		Classification:     "exact_valid",
		AdvertisedValid:    true,
		CanonicalValid:     true,
		Diagnostics:        []toolconformance.Diagnostic{},
	}
	data, err := json.Marshal(capture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func containsEnvironment(env []string, value string) bool {
	for _, entry := range env {
		if entry == value {
			return true
		}
	}
	return false
}

func environmentValue(env []string, name string) string {
	prefix := name + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
