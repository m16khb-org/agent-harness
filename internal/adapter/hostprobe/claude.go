package hostprobe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

const claudeProbeServer = "agent_harness_probe"

type ClaudeRunner struct {
	harnessBinary string
	deps          Dependencies
}

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

type claudeMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func NewClaudeRunner(harnessBinary string, deps Dependencies) ClaudeRunner {
	return ClaudeRunner{
		harnessBinary: harnessBinary,
		deps:          normalizeDependencies(deps),
	}
}

func (ClaudeRunner) Name() string { return "claude" }

func (r ClaudeRunner) Preflight(ctx context.Context, request port.HostProbeRequest) port.HostProbePreflight {
	return preflight(ctx, r.deps, r.Name(), "claude", request, isolatedHostEnv(r.deps))
}

func (r ClaudeRunner) Run(ctx context.Context, request port.HostProbeRequest) port.HostProbeResult {
	started := r.deps.Now()
	if r.harnessBinary == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_missing")
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_path_invalid")
	}
	request.HarnessBinary = harnessBinary

	executable, err := r.deps.LookPath("claude")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_root_create_failed")
	}
	defer func() { _ = os.RemoveAll(root) }()

	resultPath := filepath.Join(root, "result.json")
	serve := serveArgv(request, resultPath)
	config, err := json.Marshal(claudeMCPConfig{
		MCPServers: map[string]claudeMCPServer{
			claudeProbeServer: {
				Type:    "stdio",
				Command: serve[0],
				Args:    serve[1:],
			},
		},
	})
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mcp_config_encode_failed")
	}
	configPath := filepath.Join(root, ".mcp.json")
	if err := writePrivateFile(configPath, config); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "mcp_config_write_failed")
	}

	hookSmoke := r.deps.Getenv("HARNESS_CHILD_SMOKE_HOOKS") == "1"
	envNames := []string{}
	if hookSmoke {
		envNames = append(envNames, "HARNESS_CHILD_SMOKE_HOOKS", "HARNESS_CHILD_SMOKE_OBSERVATION_FILE")
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    claudeArgvMode(executable, configPath, request, hookSmoke),
		Env:     isolatedHostEnv(r.deps, envNames...),
		Timeout: EpisodeTimeout,
	})
	if err != nil {
		cause, code := claudeProcessFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		cause, code := claudeCaptureFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	result := completedResult(r.Name(), "", request, started, r.deps, capture)
	if observed := observedModelFromOutput(output.Stdout); observed != "" {
		result.ObservedModel = observed
	}
	if hookSmoke {
		observation, err := observeHostStream(output.Stdout)
		if err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "transport", "host_stream_invalid")
		}
		recorded, err := observeRecordedHookEvents(r.deps)
		if err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "transport", err.Error())
		}
		mergeHookObservation(&observation, recorded)
		applyHostStreamObservation(&result, observation, output.ExitCode)
		if err := persistChildSmokeObservation(r.deps, result, observation.MCPCallCount); err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "transport", err.Error())
		}
	}
	return result
}

func claudeArgv(executable, configPath string, request port.HostProbeRequest) []string {
	return claudeArgvMode(executable, configPath, request, false)
}

func claudeArgvMode(executable, configPath string, request port.HostProbeRequest, hookSmoke bool) []string {
	settingSources := ""
	if hookSmoke {
		settingSources = "user"
	}
	argv := []string{
		executable,
		"-p",
		"--verbose",
		"--setting-sources", settingSources,
		"--output-format", "stream-json",
		"--strict-mcp-config",
		"--mcp-config", configPath,
		"--no-session-persistence",
		"--permission-mode", "dontAsk",
		"--tools", "",
		"--allowedTools=mcp__" + claudeProbeServer + "__" + request.ProbeTool,
	}
	if request.Model != "" && request.Model != "default" {
		argv = append(argv, "--model", request.Model)
	}
	return append(argv, request.Prompt)
}

func claudeProcessFailure(err error) (string, string) {
	return normalizedProcessFailure(err, "host_process_failed")
}

func claudeCaptureFailure(err error) (string, string) {
	return normalizedCaptureFailure(err)
}
