package daemonresilience

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/daemoncli"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult
type daemonPaths = daemoncli.Paths
type daemonStatus = daemoncli.Status

type daemonResilienceCommandRunner func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult

func Validate(binary, root string, seed int64) StepResult {
	return validateDaemonRestartResilienceWithDeps(binary, root, seed, daemonResilienceValidationDeps{})
}

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return Validate(binary, root, seed)
}

func validateDaemonRestartResilienceWithDeps(binary, root string, seed int64, deps daemonResilienceValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempDaemon, err := deps.mkdirTemp("", fmt.Sprintf("ahd-%d-*", seed))
	if err != nil {
		return commandstep.FailedStep("daemon resilience", err)
	}
	defer deps.removeAll(tempDaemon)
	paths := daemonPaths{
		Dir:    tempDaemon,
		Socket: filepath.Join(tempDaemon, "agent-harness.sock"),
		PID:    filepath.Join(tempDaemon, "agent-harness.pid"),
		Lock:   filepath.Join(tempDaemon, "agent-harness.lock"),
		Log:    filepath.Join(tempDaemon, "agent-harness.log"),
	}
	if err := deps.writeFile(paths.Lock, []byte("999999\n"), 0o600); err != nil {
		return commandstep.FailedStep("daemon resilience", err)
	}
	old := time.Now().Add(-2 * time.Minute)
	_ = deps.chtimes(paths.Lock, old, old)
	if err := deps.writeFile(paths.Socket, []byte("stale socket placeholder\n"), 0o600); err != nil {
		return commandstep.FailedStep("daemon resilience", err)
	}
	stdoutParts := []string{}
	commands := []string{}
	env := []string{"HARNESS_DAEMON_DIR=" + tempDaemon}
	runDaemonJSON := func(label string, args ...string) (daemonStatus, StepResult, error) {
		step := deps.run(root, label, 30*time.Second, "", env, append([]string{binary}, args...)...)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		var status daemonStatus
		if step.Stdout != "" {
			if err := json.Unmarshal([]byte(step.Stdout), &status); err != nil {
				return status, step, fmt.Errorf("parse %s JSON: %w", label, err)
			}
		}
		if !step.OK {
			return status, step, fmt.Errorf("%s failed: %s", label, step.Error)
		}
		return status, step, nil
	}
	defer deps.run(root, "daemon resilience cleanup stop", 30*time.Second, "", env, binary, "daemon", "stop", "--json")

	errs := []string{}
	startStatus, startStep, startErr := runDaemonJSON("daemon resilience start", "daemon", "start", "--json")
	if startErr != nil {
		return commandstep.CombineFailedStep("daemon resilience", started, startStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	if !startStatus.OK || !startStatus.Running || startStatus.PID <= 0 {
		errs = append(errs, "daemon did not start from stale lock/socket fixture")
	}
	if deps.exists(paths.Lock) {
		errs = append(errs, "stale daemon lock remained after start")
	}
	if info, err := deps.stat(paths.Socket); err != nil {
		errs = append(errs, "daemon socket missing after start: "+err.Error())
	} else if info.Mode().Perm() != 0o600 {
		errs = append(errs, fmt.Sprintf("daemon socket mode = %o, want 600", info.Mode().Perm()))
	}

	runningStatus, statusStep, statusErr := runDaemonJSON("daemon resilience status", "daemon", "status", "--json")
	if statusErr != nil {
		return commandstep.CombineFailedStep("daemon resilience", started, statusStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	if !runningStatus.Running || filepath.Clean(runningStatus.Paths.Dir) != filepath.Clean(tempDaemon) {
		errs = append(errs, "daemon status did not report running temp daemon")
	}

	stopStatus, stopStep, stopErr := runDaemonJSON("daemon resilience stop", "daemon", "stop", "--json")
	if stopErr != nil {
		return commandstep.CombineFailedStep("daemon resilience", started, stopStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	if !stopStatus.OK || stopStatus.Running {
		errs = append(errs, "daemon stop did not report stopped state")
	}

	afterStatus, afterStep, afterErr := runDaemonJSON("daemon resilience after status", "daemon", "status", "--json")
	if afterErr != nil {
		return commandstep.CombineFailedStep("daemon resilience", started, afterStep, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	if afterStatus.Running || deps.exists(paths.Socket) || deps.exists(paths.PID) {
		errs = append(errs, "daemon stop left running socket or pid file")
	}
	if len(errs) > 0 {
		return commandstep.AssertionStepWithOutput("daemon resilience", started, errs, stdoutParts, commands, aggregateOutputBudgetBytes)
	}
	stdoutText, stdoutTruncated, stdoutBytes := commandstep.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return StepResult{
		Label:           "daemon resilience",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
