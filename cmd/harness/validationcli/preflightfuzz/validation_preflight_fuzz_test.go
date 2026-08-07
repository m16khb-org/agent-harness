package preflightfuzz

import (
	preflight "agent-harness/internal/adapter/preflight"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidatePreflightFuzzWithDepsCoversSuccessAndSetupFailure(t *testing.T) {
	root := t.TempDir()
	tempRepo := t.TempDir()
	gitCalls := []string{}
	writePaths := []string{}
	deps := preflightFuzzValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempRepo, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(path string, _ []byte, _ os.FileMode) error {
			writePaths = append(writePaths, path)
			return nil
		},
		git: func(_ string, args ...string) (int, string, string) {
			gitCalls = append(gitCalls, strings.Join(args, " "))
			return 0, "", ""
		},
		run: func(_ string, label string, _ time.Duration, _ string, command ...string) StepResult {
			return preflightFuzzStep(t, label, command, preflight.PreflightResult{
				OK: true,
				CommitStyleHints: map[string]any{
					"conventional_subjects": 1,
					"lore_bodies":           1,
				},
				SecretLikePaths: []string{".env"},
			})
		},
	}

	step := validatePreflightFuzzWithDeps("bin/agent-harness", root, 101, deps)
	if !step.OK || step.Label != "preflight fuzz" || !strings.Contains(step.Command, "preflight --json") {
		t.Fatalf("unexpected success step: %#v", step)
	}
	if len(gitCalls) != 3 || !strings.Contains(gitCalls[2], "commit -q") {
		t.Fatalf("expected init/add/commit git calls, got %v", gitCalls)
	}
	if len(writePaths) != 2 || !strings.HasSuffix(writePaths[0], "file.txt") || !strings.HasSuffix(writePaths[1], ".env") {
		t.Fatalf("expected seeded file and odd/even secret fixture writes, got %v", writePaths)
	}

	deps.mkdirTemp = func(_, _ string) (string, error) { return "", errors.New("no preflight temp") }
	failed := validatePreflightFuzzWithDeps("bin", root, 101, deps)
	if failed.OK || failed.Label != "preflight fuzz" || !strings.Contains(failed.Error, "no preflight temp") {
		t.Fatalf("expected setup failure, got %#v", failed)
	}
}

func TestValidatePreflightFuzzWithDepsCoversGitCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	tempRepo := t.TempDir()
	deps := preflightFuzzTestDeps(t, tempRepo)
	deps.git = func(_ string, args ...string) (int, string, string) {
		if len(args) > 0 && args[0] == "add" {
			return 1, "", "add failed"
		}
		return 0, "", ""
	}
	gitFailure := validatePreflightFuzzWithDeps("bin", root, 202, deps)
	if gitFailure.OK || !strings.Contains(gitFailure.Error, "git add: add failed") {
		t.Fatalf("expected git add failure, got %#v", gitFailure)
	}

	deps = preflightFuzzTestDeps(t, tempRepo)
	deps.run = func(_ string, label string, _ time.Duration, _ string, command ...string) StepResult {
		return StepResult{Label: label, Command: strings.Join(command, " "), OK: false, Error: "preflight failed"}
	}
	commandFailure := validatePreflightFuzzWithDeps("bin", root, 202, deps)
	if commandFailure.OK || !strings.Contains(commandFailure.Error, "preflight failed") {
		t.Fatalf("expected command failure, got %#v", commandFailure)
	}

	deps = preflightFuzzTestDeps(t, tempRepo)
	deps.run = func(_ string, label string, _ time.Duration, _ string, command ...string) StepResult {
		return StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: "{bad json"}
	}
	parseFailure := validatePreflightFuzzWithDeps("bin", root, 202, deps)
	if parseFailure.OK || !strings.Contains(parseFailure.Error, "invalid character") {
		t.Fatalf("expected parse failure, got %#v", parseFailure)
	}

	deps = preflightFuzzTestDeps(t, tempRepo)
	deps.run = func(_ string, label string, _ time.Duration, _ string, command ...string) StepResult {
		return preflightFuzzStep(t, label, command, preflight.PreflightResult{
			OK:               true,
			CommitStyleHints: map[string]any{"conventional_subjects": 1, "lore_bodies": 0},
			SecretLikePaths:  nil,
		})
	}
	contractFailure := validatePreflightFuzzWithDeps("bin", root, 202, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "Lore body not detected") || !strings.Contains(contractFailure.Error, "secret-like path not detected") {
		t.Fatalf("expected contract failure, got %#v", contractFailure)
	}
}

func preflightFuzzTestDeps(t *testing.T, tempRepo string) preflightFuzzValidationDeps {
	t.Helper()
	return preflightFuzzValidationDeps{
		mkdirTemp: func(_, _ string) (string, error) { return tempRepo, nil },
		removeAll: func(string) error { return nil },
		writeFile: func(string, []byte, os.FileMode) error { return nil },
		git: func(string, ...string) (int, string, string) {
			return 0, "", ""
		},
		run: func(_ string, label string, _ time.Duration, _ string, command ...string) StepResult {
			return preflightFuzzStep(t, label, command, preflight.PreflightResult{
				OK: true,
				CommitStyleHints: map[string]any{
					"conventional_subjects": 1,
					"lore_bodies":           1,
				},
				SecretLikePaths: []string{".env"},
			})
		},
	}
}

func preflightFuzzStep(t *testing.T, label string, command []string, payload preflight.PreflightResult) StepResult {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return StepResult{Label: label, Command: strings.Join(command, " "), OK: true, Stdout: string(b)}
}

func TestPreflightFuzzSecretNameVariesBySeed(t *testing.T) {
	root := t.TempDir()
	tempRepo := t.TempDir()
	writes := []string{}
	deps := preflightFuzzTestDeps(t, tempRepo)
	deps.writeFile = func(path string, _ []byte, _ os.FileMode) error {
		writes = append(writes, filepath.Base(path))
		return nil
	}
	if step := validatePreflightFuzzWithDeps("bin", root, 200, deps); !step.OK {
		t.Fatalf("even seed failed: %#v", step)
	}
	if step := validatePreflightFuzzWithDeps("bin", root, 201, deps); !step.OK {
		t.Fatalf("odd seed failed: %#v", step)
	}
	if strings.Join(writes, ",") != "file.txt,nested.secret,file.txt,.env" {
		t.Fatalf("unexpected seed-dependent fixture writes: %v", writes)
	}
}
