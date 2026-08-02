package parallelisolation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statecontract "agent-harness/internal/contract/state"

	"agent-harness/internal/core"
)

func TestValidateParallelTempIsolationWithDepsCoversSuccessErrorsAndCollision(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationValidationDeps{
		runProbe: func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
			base := filepath.Join(root, fmt.Sprintf("worker-%d-%d", seed, worker))
			return parallelIsolationProbe{Worker: worker, TempRoot: base, StateDir: base + "/state", DaemonDir: base + "/daemon", ArtifactPath: base + "/build/harness", Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Commands: []string{fmt.Sprintf("write-%d", worker)}}
		},
	}
	step := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if !step.OK || step.Label != "parallel isolation" || !strings.Contains(step.Command, "write-0") || !strings.Contains(step.Stdout, `"workers": 3`) {
		t.Fatalf("expected parallel success, got %+v", step)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, "same"), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), DaemonDir: filepath.Join(root, "daemon", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker)}
	}
	collision := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if collision.OK || !strings.Contains(collision.Error, "path collision:") {
		t.Fatalf("expected path collision, got %+v", collision)
	}

	deps.runProbe = func(_ string, _ string, seed int64, worker int) parallelIsolationProbe {
		return parallelIsolationProbe{Worker: worker, TempRoot: filepath.Join(root, fmt.Sprint(worker)), StateDir: filepath.Join(root, "state", fmt.Sprint(worker)), DaemonDir: filepath.Join(root, "daemon", fmt.Sprint(worker)), ArtifactPath: filepath.Join(root, "artifact", fmt.Sprint(worker)), Key: fmt.Sprintf("parallel-%d-%d", seed, worker), Error: "boom"}
	}
	failed := validateParallelTempIsolationWithDeps("harness", root, 7, deps)
	if failed.OK || !strings.Contains(failed.Error, "worker 0: boom") {
		t.Fatalf("expected worker error aggregation, got %+v", failed)
	}
}

func TestRunParallelIsolationProbeWithDepsCoversCommandAndContractFailures(t *testing.T) {
	root := t.TempDir()
	deps := parallelIsolationProbeDeps{
		mkdirTemp: func(_ string, pattern string) (string, error) { return filepath.Join(root, pattern+"dir"), nil },
		removeAll: func(string) error { return nil },
		mkdirAll:  func(string, os.FileMode) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
		runCommandStepEnv: func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
			switch {
			case strings.Contains(label, "write"):
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true}
			case strings.Contains(label, "read"):
				body, _ := json.Marshal(core.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			default:
				body, _ := json.Marshal(core.StateListResult{OK: true, Keys: []string{"parallel-9-2"}})
				return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
			}
		},
	}
	probe := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if probe.Error != "" || probe.Key != "parallel-9-2" || len(probe.Commands) != 3 {
		t.Fatalf("expected successful probe, got %+v", probe)
	}

	deps.runCommandStepEnv = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{OK: false, Error: "write denied"}
	}
	writeFail := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(writeFail.Error, "state write failed: write denied") {
		t.Fatalf("expected write failure, got %+v", writeFail)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	badReadJSON := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(badReadJSON.Error, "state read parse failed") {
		t.Fatalf("expected read parse failure, got %+v", badReadJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "other", Content: "wrong"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{}`}
	}
	wrongContent := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if wrongContent.Error != "state read returned another worker's content" {
		t.Fatalf("expected content failure, got %+v", wrongContent)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: `{"ok":`}
	}
	badListJSON := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(badListJSON.Error, "state list parse failed") {
		t.Fatalf("expected list parse failure, got %+v", badListJSON)
	}

	deps.runCommandStepEnv = func(_ string, label string, _ time.Duration, _ string, _ []string, _ string, args ...string) StepResult {
		if strings.Contains(label, "read") {
			body, _ := json.Marshal(core.StateResult{OK: true, Record: statecontract.RecordEnvelope{Key: "parallel-9-2", Content: "worker=2 seed=9"}})
			return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
		}
		body, _ := json.Marshal(core.StateListResult{OK: true, Keys: []string{"parallel-9-2", "leak"}})
		return StepResult{Label: label, Command: strings.Join(args, " "), OK: true, Stdout: string(body)}
	}
	leaked := runParallelIsolationProbeWithDeps("harness", root, 9, 2, deps)
	if !strings.Contains(leaked.Error, "state list leaked keys across workers") {
		t.Fatalf("expected list leak failure, got %+v", leaked)
	}
}
