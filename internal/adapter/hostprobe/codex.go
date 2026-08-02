package hostprobe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

const codexProbeServer = "agent_harness_probe"

// CodexRunner runs one capture-only MCP episode in an isolated Codex session.
type CodexRunner struct {
	harnessBinary string
	deps          Dependencies
}

func NewCodexRunner(harnessBinary string, deps Dependencies) CodexRunner {
	return CodexRunner{
		harnessBinary: harnessBinary,
		deps:          normalizeDependencies(deps),
	}
}

func (r CodexRunner) Name() string {
	return "codex"
}

func (r CodexRunner) Preflight(ctx context.Context, request port.HostProbeRequest) port.HostProbePreflight {
	return preflight(ctx, r.deps, r.Name(), "codex", request, isolatedHostEnv(r.deps, "CODEX_HOME"))
}

func (r CodexRunner) Run(ctx context.Context, request port.HostProbeRequest) port.HostProbeResult {
	started := r.deps.Now()
	if r.harnessBinary == "" {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_missing")
	}
	harnessBinary, err := filepath.Abs(r.harnessBinary)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_path_invalid")
	}
	request.HarnessBinary = harnessBinary

	executable, err := r.deps.LookPath("codex")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}
	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "episode_root_create_failed")
	}
	defer func() { _ = os.RemoveAll(root) }()

	resultPath := filepath.Join(root, "result.json")
	hookSmoke := r.deps.Getenv("HARNESS_CHILD_SMOKE_HOOKS") == "1"
	argv := codexArgvMode(executable, root, request, resultPath, hookSmoke)
	envNames := []string{"CODEX_HOME"}
	if hookSmoke {
		envNames = append(envNames, "HARNESS_CHILD_SMOKE_HOOKS", "HARNESS_CHILD_SMOKE_OBSERVATION_FILE")
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    argv,
		Env:     isolatedHostEnv(r.deps, envNames...),
		Timeout: EpisodeTimeout,
	})
	if err != nil {
		cause, code := codexProcessFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		cause, code := codexCaptureFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	result := completedResult(r.Name(), "", request, started, r.deps, capture)
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

func codexArgv(executable, root string, request port.HostProbeRequest, resultPath string) []string {
	return codexArgvMode(executable, root, request, resultPath, false)
}

func codexArgvMode(executable, root string, request port.HostProbeRequest, resultPath string, hookSmoke bool) []string {
	serve := serveArgv(request, resultPath)
	args, _ := json.Marshal(serve[1:])
	argv := []string{
		executable,
		"exec",
	}
	if !hookSmoke {
		argv = append(argv, "--ignore-user-config")
	}
	argv = append(argv,
		"--ignore-rules",
		"--ephemeral",
		"--json",
		"--sandbox", "read-only",
		"--skip-git-repo-check",
		"-C", root,
		"-c", "approval_policy="+jsonString("never"),
		"-c", "mcp_servers."+codexProbeServer+".command="+jsonString(serve[0]),
		"-c", "mcp_servers."+codexProbeServer+".args="+string(args),
		"-c", "mcp_servers."+codexProbeServer+".default_tools_approval_mode="+jsonString("approve"),
	)
	if request.Model != "" && request.Model != "default" {
		argv = append(argv, "--model", request.Model)
	}
	return append(argv, request.Prompt)
}

func codexProcessFailure(err error) (string, string) {
	return normalizedProcessFailure(err, "host_process_failed")
}

func codexCaptureFailure(err error) (string, string) {
	return normalizedCaptureFailure(err)
}
