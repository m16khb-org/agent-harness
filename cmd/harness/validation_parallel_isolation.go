package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agent-harness/internal/core"
)

type parallelIsolationProbe struct {
	Worker       int      `json:"worker"`
	TempRoot     string   `json:"temp_root"`
	StateDir     string   `json:"state_dir"`
	DaemonDir    string   `json:"daemon_dir"`
	ArtifactPath string   `json:"artifact_path"`
	Key          string   `json:"key"`
	Commands     []string `json:"commands"`
	Error        string   `json:"error,omitempty"`
}

type parallelIsolationValidationDeps struct {
	runProbe func(string, string, int64, int) parallelIsolationProbe
}

func (deps parallelIsolationValidationDeps) withDefaults() parallelIsolationValidationDeps {
	if deps.runProbe == nil {
		deps.runProbe = runParallelIsolationProbe
	}
	return deps
}

type parallelIsolationProbeDeps struct {
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	mkdirAll          func(string, os.FileMode) error
	writeFile         func(string, []byte, os.FileMode) error
	runCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) StepResult
}

func (deps parallelIsolationProbeDeps) withDefaults() parallelIsolationProbeDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.mkdirAll == nil {
		deps.mkdirAll = os.MkdirAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.runCommandStepEnv == nil {
		deps.runCommandStepEnv = runCommandStepEnv
	}
	return deps
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return validateParallelTempIsolationWithDeps(binary, root, seed, parallelIsolationValidationDeps{})
}

func validateParallelTempIsolationWithDeps(binary, root string, seed int64, deps parallelIsolationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	const workers = 3
	results := make(chan parallelIsolationProbe, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			results <- deps.runProbe(binary, root, seed, worker)
		}(worker)
	}
	wg.Wait()
	close(results)

	probes := []parallelIsolationProbe{}
	errs := []string{}
	seenPaths := map[string]string{}
	for probe := range results {
		probes = append(probes, probe)
		if probe.Error != "" {
			errs = append(errs, fmt.Sprintf("worker %d: %s", probe.Worker, probe.Error))
		}
		for label, path := range map[string]string{
			"temp_root":     probe.TempRoot,
			"state_dir":     probe.StateDir,
			"daemon_dir":    probe.DaemonDir,
			"artifact_path": probe.ArtifactPath,
		} {
			if strings.TrimSpace(path) == "" {
				errs = append(errs, fmt.Sprintf("worker %d has empty %s", probe.Worker, label))
				continue
			}
			if previous, ok := seenPaths[path]; ok {
				errs = append(errs, fmt.Sprintf("path collision: %s reused by %s and worker %d %s", path, previous, probe.Worker, label))
				continue
			}
			seenPaths[path] = fmt.Sprintf("worker %d %s", probe.Worker, label)
		}
	}
	sort.Slice(probes, func(i, j int) bool { return probes[i].Worker < probes[j].Worker })
	stdoutBytes, _ := json.MarshalIndent(map[string]any{
		"workers": workers,
		"probes":  probes,
	}, "", "  ")
	stdoutText, stdoutTruncated, stdoutOriginalBytes := tailWithBudget(string(stdoutBytes), selfVerifyAggregateOutputBudgetBytes)
	if len(errs) > 0 {
		return StepResult{Label: "parallel isolation", OK: false, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated, Error: strings.Join(errs, "; ")}
	}
	commands := []string{}
	for _, probe := range probes {
		commands = append(commands, probe.Commands...)
	}
	return StepResult{Label: "parallel isolation", Command: strings.Join(commands, " && "), OK: true, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated}
}

func runParallelIsolationProbe(binary, root string, seed int64, worker int) parallelIsolationProbe {
	return runParallelIsolationProbeWithDeps(binary, root, seed, worker, parallelIsolationProbeDeps{})
}

func runParallelIsolationProbeWithDeps(binary, root string, seed int64, worker int, deps parallelIsolationProbeDeps) parallelIsolationProbe {
	deps = deps.withDefaults()
	probe := parallelIsolationProbe{Worker: worker, Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Commands: []string{}}
	tempRoot, err := deps.mkdirTemp("", fmt.Sprintf("agent-harness-parallel-%d-%d-*", seed, worker))
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.TempRoot = tempRoot
	defer deps.removeAll(tempRoot)
	probe.StateDir = filepath.Join(tempRoot, "state")
	probe.DaemonDir = filepath.Join(tempRoot, "daemon")
	buildDir := filepath.Join(tempRoot, "build")
	probe.ArtifactPath = filepath.Join(buildDir, "harness")
	if err := deps.mkdirAll(buildDir, 0o700); err != nil {
		probe.Error = err.Error()
		return probe
	}
	if err := deps.writeFile(probe.ArtifactPath, []byte("parallel isolation artifact\n"), 0o600); err != nil {
		probe.Error = err.Error()
		return probe
	}
	env := []string{"HARNESS_STATE_DIR=" + probe.StateDir, "HARNESS_DAEMON_DIR=" + probe.DaemonDir}
	value := fmt.Sprintf("worker=%d seed=%d", worker, seed)
	write := deps.runCommandStepEnv(root, fmt.Sprintf("parallel state write %d", worker), 30*time.Second, "", env, binary, "state", "write", "--key", probe.Key, "--value", value, "--json")
	probe.Commands = append(probe.Commands, write.Command)
	if !write.OK {
		probe.Error = "state write failed: " + write.Error
		return probe
	}
	read := deps.runCommandStepEnv(root, fmt.Sprintf("parallel state read %d", worker), 30*time.Second, "", env, binary, "state", "read", "--key", probe.Key, "--json")
	probe.Commands = append(probe.Commands, read.Command)
	if !read.OK {
		probe.Error = "state read failed: " + read.Error
		return probe
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		probe.Error = "state read parse failed: " + err.Error()
		return probe
	}
	if readResult.Record.Key != probe.Key || readResult.Record.Content != value {
		probe.Error = "state read returned another worker's content"
		return probe
	}
	list := deps.runCommandStepEnv(root, fmt.Sprintf("parallel state list %d", worker), 30*time.Second, "", env, binary, "state", "list", "--json")
	probe.Commands = append(probe.Commands, list.Command)
	if !list.OK {
		probe.Error = "state list failed: " + list.Error
		return probe
	}
	var listResult core.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		probe.Error = "state list parse failed: " + err.Error()
		return probe
	}
	if len(listResult.Keys) != 1 || listResult.Keys[0] != probe.Key {
		probe.Error = fmt.Sprintf("state list leaked keys across workers: %v", listResult.Keys)
		return probe
	}
	return probe
}
