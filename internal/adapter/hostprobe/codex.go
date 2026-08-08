package hostprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/port"
)

const (
	codexProbeServer              = "agent_harness_probe"
	codexSmokePreToolUseArguments = "--host codex --enforce-worktree --enforce-korean-remote-artifacts --enforce-vcs-issue-linking --enforce-staged-checks --enforce-gitops-kubectl"
)

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
	environment := isolatedHostEnv(r.deps, envNames...)
	if hookSmoke {
		runtimeCodexHome, err := prepareCodexSmokeHome(root, harnessBinary, r.deps.Getenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE"), r.deps)
		if err != nil {
			return failedResult(r.Name(), "", request, started, r.deps, "harness_environment", "codex_smoke_home_invalid")
		}
		environment = replaceEnvironmentValue(environment, "CODEX_HOME", runtimeCodexHome)
	}
	output, err := r.deps.Process.Run(ctx, CommandRequest{
		Cwd:     root,
		Argv:    argv,
		Env:     environment,
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

type codexSmokeHookDocument struct {
	Hooks map[string][]codexSmokeHookGroup `json:"hooks"`
}

type codexSmokeHookGroup struct {
	Hooks []codexSmokeHook `json:"hooks"`
}

type codexSmokeHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func prepareCodexSmokeHome(root, harnessBinary, observationPath string, deps Dependencies) (string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(harnessBinary) || !filepath.IsAbs(observationPath) {
		return "", fmt.Errorf("codex_smoke_path_invalid")
	}
	sourceHome, err := resolveCodexSourceHome(deps)
	if err != nil {
		return "", err
	}
	document, err := projectActivatedCodexSmokeHooks(filepath.Join(sourceHome, "hooks.json"), harnessBinary, observationPath)
	if err != nil {
		return "", err
	}
	runtimeHome := filepath.Join(root, "codex-home")
	if err := os.Mkdir(runtimeHome, 0o700); err != nil {
		return "", err
	}
	if err := linkCodexSmokeAuth(runtimeHome, sourceHome); err != nil {
		return "", err
	}
	data, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	hooksPath := filepath.Join(runtimeHome, "hooks.json")
	file, err := os.OpenFile(hooksPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	directory, err := os.Open(runtimeHome)
	if err != nil {
		return "", err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return "", err
	}
	return runtimeHome, nil
}

func resolveCodexSourceHome(deps Dependencies) (string, error) {
	sourceHome := strings.TrimSpace(deps.Getenv("CODEX_HOME"))
	if sourceHome == "" {
		home := strings.TrimSpace(deps.Getenv("HOME"))
		if !filepath.IsAbs(home) {
			return "", fmt.Errorf("codex_home_invalid")
		}
		sourceHome = filepath.Join(home, ".codex")
	}
	if !filepath.IsAbs(sourceHome) {
		return "", fmt.Errorf("codex_home_invalid")
	}
	return sourceHome, nil
}

func projectActivatedCodexSmokeHooks(sourcePath, harnessBinary, observationPath string) (codexSmokeHookDocument, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return codexSmokeHookDocument{}, err
	}
	var source struct {
		Hooks map[string][]struct {
			Hooks []map[string]json.RawMessage `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return codexSmokeHookDocument{}, err
	}
	prefix := "/usr/bin/env HARNESS_CHILD_SMOKE_HOOKS=1 HARNESS_CHILD_SMOKE_OBSERVATION_FILE=" + shellSingleQuote(observationPath) + " "
	projected := codexSmokeHookDocument{Hooks: make(map[string][]codexSmokeHookGroup, 2)}
	for _, contract := range []struct {
		event      string
		subcommand string
		expected   string
	}{
		{event: "SessionStart", subcommand: "session-start", expected: shellSingleQuote(harnessBinary) + " hook session-start --host codex"},
		{event: "PreToolUse", subcommand: "pre-tool-use", expected: shellSingleQuote(harnessBinary) + " hook pre-tool-use " + codexSmokePreToolUseArguments},
	} {
		var candidates []codexSmokeHook
		managedPrefix := shellSingleQuote(harnessBinary) + " hook " + contract.subcommand
		for _, group := range source.Hooks[contract.event] {
			for _, rawHook := range group.Hooks {
				var command string
				if err := json.Unmarshal(rawHook["command"], &command); err != nil || !strings.HasPrefix(command, managedPrefix) {
					continue
				}
				if len(rawHook) != 3 {
					return codexSmokeHookDocument{}, fmt.Errorf("codex_smoke_hook_structure_invalid")
				}
				encoded, err := json.Marshal(rawHook)
				if err != nil {
					return codexSmokeHookDocument{}, err
				}
				var hook codexSmokeHook
				if err := json.Unmarshal(encoded, &hook); err != nil {
					return codexSmokeHookDocument{}, err
				}
				if hook.Type != "command" || hook.Timeout != 5 || hook.Command != contract.expected {
					return codexSmokeHookDocument{}, fmt.Errorf("codex_smoke_hook_contract_invalid")
				}
				candidates = append(candidates, hook)
			}
		}
		if len(candidates) != 1 {
			return codexSmokeHookDocument{}, fmt.Errorf("codex_smoke_hook_cardinality_invalid")
		}
		hook := candidates[0]
		hook.Command = prefix + hook.Command
		projected.Hooks[contract.event] = []codexSmokeHookGroup{{Hooks: []codexSmokeHook{hook}}}
	}
	return projected, nil
}

func linkCodexSmokeAuth(runtimeHome, sourceHome string) error {
	authPath := filepath.Join(sourceHome, "auth.json")
	info, err := os.Lstat(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("codex_auth_invalid")
	}
	return os.Symlink(authPath, filepath.Join(runtimeHome, "auth.json"))
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func replaceEnvironmentValue(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	replaced := false
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				result = append(result, prefix+value)
				replaced = true
			}
			continue
		}
		result = append(result, entry)
	}
	if !replaced {
		result = append(result, prefix+value)
	}
	sort.Strings(result)
	return result
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
	} else {
		argv = append(argv, "--dangerously-bypass-hook-trust")
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
