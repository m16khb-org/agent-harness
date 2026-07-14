package hostprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/port"
)

const gjcExecutable = "gjc"

type GJCRunner struct {
	harnessBinary string
	deps          Dependencies
}

var _ port.HostProbeRunner = (*GJCRunner)(nil)

func NewGJCRunner(harnessBinary string, deps Dependencies) *GJCRunner {
	return &GJCRunner{
		harnessBinary: harnessBinary,
		deps:          normalizeDependencies(deps),
	}
}

func (*GJCRunner) Name() string { return "gjc" }

func (r *GJCRunner) Preflight(ctx context.Context, request port.HostProbeRequest) port.HostProbePreflight {
	root, err := newEpisodeRoot(r.deps, "gjc-preflight")
	if err != nil {
		return port.HostProbePreflight{
			Host: r.Name(), RequestedModel: request.Model, ObservedModel: request.Model,
			Cause: "harness_environment", Code: "isolated_environment_failed", EvidenceSource: "gjc_preflight",
		}
	}
	defer os.RemoveAll(root)
	agentRoot, err := newGJCPrivateDirectory(root, "agent")
	if err != nil {
		return port.HostProbePreflight{
			Host: r.Name(), RequestedModel: request.Model, ObservedModel: request.Model,
			Cause: "harness_environment", Code: "isolated_environment_failed", EvidenceSource: "gjc_preflight",
		}
	}
	return preflight(ctx, r.deps, r.Name(), gjcExecutable, request, isolatedGJCEnv(r.deps, nil, agentRoot))
}

func (r *GJCRunner) Run(ctx context.Context, request port.HostProbeRequest) port.HostProbeResult {
	started := r.deps.Now()
	model := strings.TrimSpace(request.Model)
	if model == "" || strings.EqualFold(model, "default") {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_auth_unavailable")
	}
	auth := requestedGJCAuth(r.deps, request.GJCAuthEnv)
	if len(auth) == 0 {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_auth_unavailable")
	}
	harnessBinary, err := r.harnessBinaryFor(request)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "harness_binary_invalid")
	}
	gjcPath, err := r.deps.LookPath(gjcExecutable)
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "executable_not_found")
	}

	root, err := newEpisodeRoot(r.deps, r.Name())
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_environment_failed")
	}
	defer os.RemoveAll(root)

	projectRoot, err := newGJCPrivateDirectory(root, "project")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_environment_failed")
	}
	agentRoot, err := newGJCPrivateDirectory(root, "agent")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_environment_failed")
	}
	bundleRoot, err := newGJCPrivateDirectory(root, "bundle")
	if err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_environment_failed")
	}
	resultPath := filepath.Join(root, "capture.json")
	serve := serveArgv(requestWithHarnessBinary(request, harnessBinary), resultPath)
	if err := writeGJCBundle(bundleRoot, serve); err != nil {
		return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "isolated_environment_failed")
	}
	env := isolatedGJCEnv(r.deps, auth, agentRoot)
	episodeCtx, cancel := context.WithTimeout(ctx, EpisodeTimeout)
	defer cancel()
	install := CommandRequest{
		Cwd:     projectRoot,
		Argv:    []string{gjcPath, "plugin", "install", bundleRoot, "--project"},
		Env:     env,
		Timeout: EpisodeTimeout,
	}
	if _, err := r.deps.Process.Run(episodeCtx, install); err != nil {
		cause, code := normalizedProcessFailure(err, "plugin_install_failed")
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	invoke := CommandRequest{
		Cwd: projectRoot,
		Argv: []string{
			gjcPath,
			"-p",
			"--mode=json",
			"--no-session",
			"--no-tools",
			"--no-lsp",
			"--no-skills",
			"--no-rules",
			"--model", model,
			request.Prompt,
		},
		Env:     env,
		Timeout: EpisodeTimeout,
	}
	if _, err := r.deps.Process.Run(episodeCtx, invoke); err != nil {
		cause, code := normalizedProcessFailure(err, "host_process_failed")
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	capture, err := decodeEpisodeCapture(resultPath, request)
	if err != nil {
		cause, code := normalizedCaptureFailure(err)
		return failedResult(r.Name(), "", request, started, r.deps, cause, code)
	}
	result := completedResult(r.Name(), "", request, started, r.deps, capture)
	result.ObservedModel = model
	return result
}

func (r *GJCRunner) harnessBinaryFor(_ port.HostProbeRequest) (string, error) {
	binary := strings.TrimSpace(r.harnessBinary)
	if binary == "" || !filepath.IsAbs(binary) {
		return "", fmt.Errorf("harness_binary_invalid")
	}
	return filepath.Clean(binary), nil
}

func requestWithHarnessBinary(request port.HostProbeRequest, harnessBinary string) port.HostProbeRequest {
	request.HarnessBinary = harnessBinary
	return request
}

func newGJCPrivateDirectory(root, name string) (string, error) {
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func requestedGJCAuth(deps Dependencies, names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, requested := range names {
		name := strings.TrimSpace(requested)
		if seen[name] || isGJCAmbientEnvironment(name) || name == "GJC_CODING_AGENT_DIR" || !isEnvironmentName(name) {
			continue
		}
		seen[name] = true
		if value := deps.Getenv(name); value != "" {
			out = append(out, name+"="+value)
		}
	}
	return out
}

func isolatedGJCEnv(deps Dependencies, auth []string, agentRoot string) []string {
	values := map[string]string{}
	for _, entry := range isolatedHostEnv(deps) {
		name, value, found := strings.Cut(entry, "=")
		if found && isGJCAmbientEnvironment(name) {
			values[name] = value
		}
	}
	for _, entry := range auth {
		name, value, found := strings.Cut(entry, "=")
		if found && !isGJCAmbientEnvironment(name) && name != "GJC_CODING_AGENT_DIR" {
			values[name] = value
		}
	}
	if agentRoot != "" {
		values["GJC_CODING_AGENT_DIR"] = agentRoot
	}
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, name := range keys {
		env = append(env, name+"="+values[name])
	}
	return env
}

func isGJCAmbientEnvironment(name string) bool {
	switch name {
	case "PATH", "HOME", "USER", "TMPDIR", "LANG", "LANGUAGE", "LC_ALL", "LC_ADDRESS", "LC_COLLATE", "LC_CTYPE", "LC_IDENTIFICATION", "LC_MEASUREMENT", "LC_MESSAGES", "LC_MONETARY", "LC_NAME", "LC_NUMERIC", "LC_PAPER", "LC_TELEPHONE", "LC_TIME":
		return true
	default:
		return false
	}
}

func isEnvironmentName(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
