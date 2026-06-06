package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
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
