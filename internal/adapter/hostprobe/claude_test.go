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
	"strings"
	"testing"

	"agent-harness/internal/core/toolconformance"
	"agent-harness/internal/port"
)

type claudeCommandRunner struct {
	run func(context.Context, CommandRequest) (CommandOutput, error)
}

func (r claudeCommandRunner) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	return r.run(ctx, request)
}

func claudeProbeRequest() port.HostProbeRequest {
	return port.HostProbeRequest{
		HarnessBinary: "/ignored/by-constructor",
		FixtureID:     "empty_object",
		ProbeTool:     "agent_harness_web_fetch_resilient",
		SchemaSHA256:  "schema-sha",
		Prompt:        "Call only the probe tool.",
		Model:         "claude-opus-4-6",
		Profile:       "clean",
		Attempt:       1,
		RunToken:      "episode-token",
	}
}

func writeClaudeCapture(t *testing.T, resultPath, token string) {
	t.Helper()
	tokenDigest := sha256.Sum256([]byte(token))
	rawDigest := sha256.Sum256([]byte(`{}`))
	capture := episodeCapture{
		FixtureID:          "empty_object",
		CallCount:          1,
		RawSHA256:          hex.EncodeToString(rawDigest[:]),
		CanonicalArguments: map[string]any{},
		SchemaSHA256:       "schema-sha",
		RunTokenSHA256:     hex.EncodeToString(tokenDigest[:]),
		Classification:     "exact_valid",
		AdvertisedValid:    true,
		CanonicalValid:     true,
		Diagnostics:        []toolconformance.Diagnostic{},
	}
	data, err := json.Marshal(capture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func argumentValue(t *testing.T, argv []string, flag string) string {
	t.Helper()
	for index, value := range argv {
		if value == flag {
			if index+1 == len(argv) {
				t.Fatalf("flag %q has no value in %q", flag, argv)
			}
			return argv[index+1]
		}
	}
	t.Fatalf("flag %q missing from %q", flag, argv)
	return ""
}

func TestClaudeRunnerRunUsesOnePrivateProbeConfig(t *testing.T) {
	episodeRoot := filepath.Join(t.TempDir(), "episode")
	harnessBinary, err := filepath.Abs("./bin/agent-harness")
	if err != nil {
		t.Fatal(err)
	}
	request := claudeProbeRequest()
	inherited := []string{"PATH=/test/bin", "HOME=/private/home", "USER=tester", "TEST_CREDENTIAL=credential-value"}
	var received CommandRequest
	runner := NewClaudeRunner("./bin/agent-harness", Dependencies{
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(episodeRoot, 0o755); err != nil {
				return "", err
			}
			return episodeRoot, nil
		},
		Environ:  func() []string { return inherited },
		LookPath: func(string) (string, error) { return "/test/bin/claude", nil },
		Process: claudeCommandRunner{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
			received = command
			if command.Cwd != episodeRoot {
				t.Fatalf("cwd=%q want %q", command.Cwd, episodeRoot)
			}
			if command.Timeout != EpisodeTimeout {
				t.Fatalf("timeout=%s want %s", command.Timeout, EpisodeTimeout)
			}
			if !reflect.DeepEqual(command.Env, []string{"HOME=/private/home", "PATH=/test/bin", "USER=tester"}) {
				t.Fatalf("env=%q want isolated host env", command.Env)
			}
			rootInfo, err := os.Stat(episodeRoot)
			if err != nil {
				t.Fatal(err)
			}
			if rootInfo.Mode().Perm() != 0o700 {
				t.Fatalf("root mode=%#o want 0700", rootInfo.Mode().Perm())
			}

			configPath := argumentValue(t, command.Argv, "--mcp-config")
			configInfo, err := os.Stat(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if configInfo.Mode().Perm() != 0o600 {
				t.Fatalf("config mode=%#o want 0600", configInfo.Mode().Perm())
			}
			configData, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(configData), request.Prompt) || strings.Contains(string(configData), "credential-value") {
				t.Fatalf("config retained live input: %s", configData)
			}
			var config claudeMCPConfig
			if err := json.Unmarshal(configData, &config); err != nil {
				t.Fatal(err)
			}
			if len(config.MCPServers) != 1 {
				t.Fatalf("server count=%d want 1", len(config.MCPServers))
			}
			server, ok := config.MCPServers[claudeProbeServer]
			if !ok {
				t.Fatalf("config servers=%#v", config.MCPServers)
			}
			if server.Type != "stdio" || server.Command != harnessBinary {
				t.Fatalf("server=%#v", server)
			}
			resultPath := filepath.Join(episodeRoot, "result.json")
			wantServe := []string{"contract", "conformance", "serve", "--fixture-id", request.FixtureID, "--result-file", resultPath, "--run-token", request.RunToken}
			if !reflect.DeepEqual(server.Args, wantServe) {
				t.Fatalf("serve args=%q want %q", server.Args, wantServe)
			}
			wantArgv := []string{
				"/test/bin/claude", "-p", "--verbose", "--setting-sources", "",
				"--output-format", "stream-json", "--strict-mcp-config", "--mcp-config", configPath,
				"--no-session-persistence", "--permission-mode", "dontAsk", "--tools", "",
				"--allowedTools=mcp__agent_harness_probe__" + request.ProbeTool,
				"--model", request.Model, request.Prompt,
			}
			if !reflect.DeepEqual(command.Argv, wantArgv) {
				t.Fatalf("argv=%q want %q", command.Argv, wantArgv)
			}
			writeClaudeCapture(t, resultPath, request.RunToken)
			return CommandOutput{Stdout: []byte(request.Prompt), Stderr: []byte("credential-value")}, nil
		}},
	})

	result := runner.Run(context.Background(), request)
	if !result.Completed {
		t.Fatalf("result=%+v", result)
	}
	if received.Argv == nil {
		t.Fatal("Claude process was not invoked")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{request.Prompt, "credential-value", "/private/home"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("result retained %q: %s", forbidden, encoded)
		}
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root cleanup err=%v", err)
	}
}

func TestClaudeRunnerRunDefaultModelOmitsModelFlag(t *testing.T) {
	episodeRoot := filepath.Join(t.TempDir(), "episode")
	request := claudeProbeRequest()
	request.Model = "default"
	runner := NewClaudeRunner("harness", Dependencies{
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(episodeRoot, 0o700); err != nil {
				return "", err
			}
			return episodeRoot, nil
		},
		LookPath: func(string) (string, error) { return "/test/bin/claude", nil },
		Process: claudeCommandRunner{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
			for _, arg := range command.Argv {
				if arg == "--model" {
					t.Fatalf("default model must not add --model: %q", command.Argv)
				}
			}
			writeClaudeCapture(t, filepath.Join(episodeRoot, "result.json"), request.RunToken)
			return CommandOutput{}, nil
		}},
	})

	if result := runner.Run(context.Background(), request); !result.Completed {
		t.Fatalf("result=%+v", result)
	}
}

func TestClaudeRunnerRunMissingCaptureIsTransportFailure(t *testing.T) {
	episodeRoot := filepath.Join(t.TempDir(), "episode")
	request := claudeProbeRequest()
	runner := NewClaudeRunner("harness", Dependencies{
		TempDir: func(_, _ string) (string, error) {
			if err := os.Mkdir(episodeRoot, 0o700); err != nil {
				return "", err
			}
			return episodeRoot, nil
		},
		LookPath: func(string) (string, error) { return "/test/bin/claude", nil },
		Process: claudeCommandRunner{run: func(context.Context, CommandRequest) (CommandOutput, error) {
			return CommandOutput{}, nil
		}},
	})

	result := runner.Run(context.Background(), request)
	if result.Completed || result.Cause != "unknown" || result.Code != "no_call" || result.EvidenceSource != "claude_runner" {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(episodeRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("episode root cleanup err=%v", err)
	}
}

func TestClaudeRunnerPreflightUsesClaudeExecutable(t *testing.T) {
	request := claudeProbeRequest()
	request.Model = "default"
	var executable string
	runner := NewClaudeRunner("harness", Dependencies{
		LookPath: func(name string) (string, error) {
			executable = name
			return "/test/bin/claude", nil
		},
		Environ: func() []string { return []string{"PATH=/test/bin"} },
		Process: claudeCommandRunner{run: func(_ context.Context, command CommandRequest) (CommandOutput, error) {
			if !reflect.DeepEqual(command.Argv, []string{"/test/bin/claude", "--version"}) {
				t.Fatalf("argv=%q", command.Argv)
			}
			if !reflect.DeepEqual(command.Env, []string{"PATH=/test/bin"}) {
				t.Fatalf("env=%q", command.Env)
			}
			if command.Timeout != VersionTimeout {
				t.Fatalf("timeout=%s", command.Timeout)
			}
			return CommandOutput{Stdout: []byte("claude 2.1.209\n")}, nil
		}},
	})

	result := runner.Preflight(context.Background(), request)
	if executable != "claude" || !result.Ready || result.Version != "claude 2.1.209" || result.Host != "claude" {
		t.Fatalf("preflight=%+v executable=%q", result, executable)
	}
}

func TestNewClaudeRunnerNormalizesDependencies(t *testing.T) {
	runner := NewClaudeRunner("harness", Dependencies{})
	if runner.deps.Process == nil || runner.deps.LookPath == nil || runner.deps.Now == nil || runner.deps.TempDir == nil || runner.deps.Getenv == nil || runner.deps.Environ == nil {
		t.Fatalf("dependencies were not normalized: %#v", runner.deps)
	}
	if runner.harnessBinary != "harness" {
		t.Fatalf("harness binary=%q", runner.harnessBinary)
	}
}

var _ port.HostProbeRunner = ClaudeRunner{}
