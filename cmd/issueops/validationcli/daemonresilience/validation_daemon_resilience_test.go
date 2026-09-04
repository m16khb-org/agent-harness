package daemonresilience

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateDaemonRestartResilienceWrapperUsesExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeDaemonResilienceFakeBinary(t, root)

	step := validateDaemonRestartResilience(binary, root, 505)
	if !step.OK || !strings.Contains(step.Command, "daemon start") || !strings.Contains(step.Command, "daemon stop") {
		t.Fatalf("expected wrapper success, got %+v", step)
	}
}

func TestValidateDaemonRestartResilienceWithDepsCoversSuccessAndSetupFailure(t *testing.T) {
	root := t.TempDir()
	tempDaemon := t.TempDir()
	calls := []string{}
	deps := daemonResilienceValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempDaemon, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(path string, content []byte, _ os.FileMode) error {
			if strings.HasSuffix(path, "issueops.sock") {
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			return nil
		},
		chtimes: func(string, time.Time, time.Time) error { return nil },
		stat: func(path string) (os.FileInfo, error) {
			if strings.HasSuffix(path, "issueops.sock") {
				return os.Stat(path)
			}
			return nil, errors.New("unexpected stat")
		},
		exists: func(path string) bool {
			return false
		},
		run: func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
			calls = append(calls, label+":"+strings.Join(command, " "))
			return daemonResilienceStep(t, label, daemonResiliencePayload(tempDaemon, label))
		},
	}

	step := validateDaemonRestartResilienceWithDeps("bin/issueops", root, 777, deps)
	if !step.OK || step.Label != "daemon resilience" || len(calls) != 5 || !strings.Contains(step.Command, "daemon start") || !strings.Contains(step.Command, "daemon stop") {
		t.Fatalf("unexpected success step: %#v calls=%v", step, calls)
	}

	deps.mkdirTemp = func(_, _ string) (string, error) { return "", errors.New("no daemon temp") }
	failed := validateDaemonRestartResilienceWithDeps("bin", root, 777, deps)
	if failed.OK || failed.Label != "daemon resilience" || !strings.Contains(failed.Error, "no daemon temp") {
		t.Fatalf("expected setup failure, got %#v", failed)
	}
}

func TestValidateDaemonRestartResilienceWithDepsCoversCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	tempDaemon := t.TempDir()
	deps := daemonResilienceTestDeps(t, tempDaemon)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		if label == "daemon resilience status" {
			return StepResult{Label: label, Command: strings.Join(command, " "), OK: false, Error: "status failed"}
		}
		return daemonResilienceStep(t, label, daemonResiliencePayload(tempDaemon, label))
	}
	commandFailure := validateDaemonRestartResilienceWithDeps("bin", root, 888, deps)
	if commandFailure.OK || !strings.Contains(commandFailure.Error, "status failed") || !strings.Contains(commandFailure.Command, "daemon status") {
		t.Fatalf("expected command failure, got %#v", commandFailure)
	}

	deps = daemonResilienceTestDeps(t, tempDaemon)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		if label == "daemon resilience start" {
			return StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: "{bad json"}
		}
		return daemonResilienceStep(t, label, daemonResiliencePayload(tempDaemon, label))
	}
	parseFailure := validateDaemonRestartResilienceWithDeps("bin", root, 888, deps)
	if parseFailure.OK || !strings.Contains(parseFailure.Error, "daemon resilience start failed") {
		t.Fatalf("expected parse failure, got %#v", parseFailure)
	}

	deps = daemonResilienceTestDeps(t, tempDaemon)
	deps.run = func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
		payload := daemonResiliencePayload(tempDaemon, label)
		if label == "daemon resilience start" {
			payload = daemonStatus{OK: true, Running: false, Paths: daemonResiliencePaths(tempDaemon)}
		}
		return daemonResilienceStep(t, label, payload)
	}
	contractFailure := validateDaemonRestartResilienceWithDeps("bin", root, 888, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "daemon did not start from stale lock/socket fixture") {
		t.Fatalf("expected contract failure, got %#v", contractFailure)
	}
}

func daemonResilienceTestDeps(t *testing.T, tempDaemon string) daemonResilienceValidationDeps {
	t.Helper()
	return daemonResilienceValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempDaemon, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(path string, content []byte, _ os.FileMode) error {
			if strings.HasSuffix(path, "issueops.sock") {
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			return nil
		},
		chtimes: func(string, time.Time, time.Time) error { return nil },
		stat: func(path string) (os.FileInfo, error) {
			if strings.HasSuffix(path, "issueops.sock") {
				return os.Stat(path)
			}
			return nil, fmt.Errorf("unexpected stat %s", path)
		},
		exists: func(path string) bool {
			return false
		},
		run: func(_ string, label string, _ time.Duration, _ string, _ []string, command ...string) StepResult {
			return daemonResilienceStep(t, label, daemonResiliencePayload(tempDaemon, label))
		},
	}
}

func daemonResilienceStep(t *testing.T, label string, payload daemonStatus) StepResult {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return StepResult{Label: label, Command: strings.ReplaceAll(label, " resilience ", " ") + " --json", OK: true, Stdout: string(b)}
}

func daemonResiliencePayload(dir, label string) daemonStatus {
	paths := daemonResiliencePaths(dir)
	switch label {
	case "daemon resilience start", "daemon resilience status":
		return daemonStatus{OK: true, Running: true, PID: 1234, Paths: paths}
	case "daemon resilience stop", "daemon resilience after status":
		return daemonStatus{OK: true, Running: false, Paths: paths}
	default:
		return daemonStatus{OK: true, Paths: paths}
	}
}

func daemonResiliencePaths(dir string) daemonPaths {
	return daemonPaths{
		Dir:    dir,
		Socket: filepath.Join(dir, "issueops.sock"),
		PID:    filepath.Join(dir, "issueops.pid"),
		Lock:   filepath.Join(dir, "issueops.lock"),
		Log:    filepath.Join(dir, "issueops.log"),
	}
}

func writeDaemonResilienceFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	body := `#!/bin/sh
set -eu
dir="${ISSUEOPS_DAEMON_DIR:?}"
socket="$dir/issueops.sock"
pid="$dir/issueops.pid"
lock="$dir/issueops.lock"
log="$dir/issueops.log"
status_running() {
  printf '{"ok":true,"running":true,"pid":1234,"paths":{"dir":"%s","socket":"%s","pid":"%s","lock":"%s","log":"%s"}}\n' "$dir" "$socket" "$pid" "$lock" "$log"
}
status_stopped() {
  printf '{"ok":true,"running":false,"paths":{"dir":"%s","socket":"%s","pid":"%s","lock":"%s","log":"%s"}}\n' "$dir" "$socket" "$pid" "$lock" "$log"
}
case "$*" in
  "daemon start --json")
    rm -f "$lock"
    printf started > "$socket"
    chmod 600 "$socket"
    printf 1234 > "$pid"
    status_running
    ;;
  "daemon status --json")
    if [ -e "$socket" ] && [ -e "$pid" ]; then status_running; else status_stopped; fi
    ;;
  "daemon stop --json")
    rm -f "$socket" "$pid"
    status_stopped
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
