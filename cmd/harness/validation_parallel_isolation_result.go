package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func parallelIsolationResult(started time.Time, workers int, probes []parallelIsolationProbe) StepResult {
	sort.Slice(probes, func(i, j int) bool { return probes[i].Worker < probes[j].Worker })
	stdoutText, stdoutTruncated, stdoutOriginalBytes := parallelIsolationOutput(workers, probes)
	errs := parallelIsolationErrors(probes)
	if len(errs) > 0 {
		return StepResult{Label: "parallel isolation", OK: false, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated, Error: strings.Join(errs, "; ")}
	}
	return StepResult{Label: "parallel isolation", Command: strings.Join(parallelIsolationCommands(probes), " && "), OK: true, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated}
}

func parallelIsolationOutput(workers int, probes []parallelIsolationProbe) (string, bool, int) {
	stdoutBytes, _ := json.MarshalIndent(map[string]any{
		"workers": workers,
		"probes":  probes,
	}, "", "  ")
	return tailWithBudget(string(stdoutBytes), selfVerifyAggregateOutputBudgetBytes)
}

func parallelIsolationErrors(probes []parallelIsolationProbe) []string {
	errs := []string{}
	seenPaths := map[string]string{}
	for _, probe := range probes {
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
	return errs
}

func parallelIsolationCommands(probes []parallelIsolationProbe) []string {
	commands := []string{}
	for _, probe := range probes {
		commands = append(commands, probe.Commands...)
	}
	return commands
}
