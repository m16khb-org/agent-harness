package parallelisolation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agent-harness/cmd/harness/commandstep"
	statecontract "agent-harness/internal/contract/state"
)

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

func runCommandStepEnv(root, label string, timeout time.Duration, input string, env []string, name string, args ...string) StepResult {
	return commandstep.RunEnv(root, label, timeout, input, env, 32*1024, name, args...)
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
	var readResult statecontract.StateResult
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
	var listResult statecontract.StateListResult
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
