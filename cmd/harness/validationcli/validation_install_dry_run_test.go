package validationcli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateInstallDryRunSmokeWrapperUsesExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeInstallDryRunFakeBinary(t, root)

	step := validateInstallDryRunSmoke(binary, root, 404)
	if !step.OK || !strings.Contains(step.Command, "install-native --dry-run --project-local --json") {
		t.Fatalf("expected wrapper success, got %+v", step)
	}
}

func TestValidateInstallDryRunSmokeWithDepsCoversSuccessAndSetupFailures(t *testing.T) {
	root := t.TempDir()
	tempHome := t.TempDir()
	tempRoot := t.TempDir()
	deps := installDryRunValidationDeps{
		makeTempDir: func(kind string, seed int64) (string, error) {
			if seed != 23 {
				t.Fatalf("unexpected seed: %d", seed)
			}
			switch kind {
			case "home":
				return tempHome, nil
			case "root":
				return tempRoot, nil
			default:
				t.Fatalf("unexpected temp kind: %s", kind)
			}
			return "", nil
		},
		removeAll: func(string) error { return nil },
		makeDirAll: func(path string, _ uint32) error {
			if path != filepath.Join(tempRoot, "skills", skillName) {
				t.Fatalf("unexpected skill dir: %s", path)
			}
			return nil
		},
		writeFile: func(path string, data []byte, _ uint32) error {
			if path != filepath.Join(tempRoot, "skills", skillName, "SKILL.md") || !strings.Contains(string(data), "install dry-run smoke") {
				t.Fatalf("unexpected skill file write: path=%s data=%q", path, string(data))
			}
			return nil
		},
		exists: func(string) bool { return false },
		run: func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			if dir != root || label != "install dry-run smoke" || timeout != 30*time.Second || stdin != "" {
				t.Fatalf("unexpected command envelope: dir=%q label=%q timeout=%s stdin=%q", dir, label, timeout, stdin)
			}
			if !containsString(env, "HOME="+tempHome) || !containsString(env, "CODEX_HOME="+filepath.Join(tempHome, ".codex")) || !containsString(env, "HARNESS_ROOT="+tempRoot) {
				t.Fatalf("unexpected env: %v", env)
			}
			command := strings.Join(append([]string{name}, args...), " ")
			if command != "bin/agent-harness install-native --dry-run --project-local --json" {
				t.Fatalf("unexpected command: %s", command)
			}
			return StepResult{Label: label, Command: command, OK: true, Stdout: mustMarshalInstallDryRunTest(t, validInstallDryRunResult())}
		},
	}
	step := validateInstallDryRunSmokeWithDeps("bin/agent-harness", root, 23, deps)
	if !step.OK || step.Label != "install dry-run smoke" || !strings.Contains(step.Command, "install-native --dry-run") {
		t.Fatalf("unexpected install dry-run success: %+v", step)
	}

	deps.makeTempDir = func(string, int64) (string, error) { return "", errors.New("temp fail") }
	tempFailure := validateInstallDryRunSmokeWithDeps("bin", root, 23, deps)
	if tempFailure.OK || tempFailure.Error != "temp fail" {
		t.Fatalf("unexpected temp failure: %+v", tempFailure)
	}
	deps.makeTempDir = func(kind string, _ int64) (string, error) {
		if kind == "home" {
			return tempHome, nil
		}
		return tempRoot, nil
	}

	deps.writeFile = func(string, []byte, uint32) error { return errors.New("write fail") }
	writeFailure := validateInstallDryRunSmokeWithDeps("bin", root, 23, deps)
	if writeFailure.OK || writeFailure.Error != "write fail" {
		t.Fatalf("unexpected write failure: %+v", writeFailure)
	}
}

func TestValidateInstallDryRunSmokeWithDepsCoversCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	tempHome := t.TempDir()
	tempRoot := t.TempDir()
	deps := installDryRunValidationDeps{
		makeTempDir: func(kind string, _ int64) (string, error) {
			if kind == "home" {
				return tempHome, nil
			}
			return tempRoot, nil
		},
		removeAll:  func(string) error { return nil },
		makeDirAll: func(string, uint32) error { return nil },
		writeFile:  func(string, []byte, uint32) error { return nil },
		exists:     func(string) bool { return false },
	}

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "install dry-run smoke", Command: "install", OK: false, Error: "boom"}
	}
	commandFailure := validateInstallDryRunSmokeWithDeps("bin", root, 3, deps)
	if commandFailure.OK || commandFailure.Error != "boom" {
		t.Fatalf("unexpected command failure: %+v", commandFailure)
	}

	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "install dry-run smoke", Command: "install", OK: true, Stdout: "{"}
	}
	parseFailure := validateInstallDryRunSmokeWithDeps("bin", root, 3, deps)
	if parseFailure.OK || parseFailure.Error == "" {
		t.Fatalf("expected parse failure, got %+v", parseFailure)
	}

	invalidResult := validInstallDryRunResult()
	invalidResult.DryRun = false
	invalidResult.Hosts = invalidResult.Hosts[:1]
	invalidResult.SkillNames = nil
	invalidResult.Files[0].Written = true
	invalidResult.Files[0].WouldWrite = false
	invalidResult.Links[0].Created = true
	invalidResult.Links[0].WouldCreate = false
	deps.run = func(string, string, time.Duration, string, []string, string, ...string) StepResult {
		return StepResult{Label: "install dry-run smoke", Command: "install", OK: true, Stdout: mustMarshalInstallDryRunTest(t, invalidResult)}
	}
	deps.exists = func(path string) bool {
		return strings.HasSuffix(path, filepath.Join(tempRoot, ".mcp.json"))
	}
	contractFailure := validateInstallDryRunSmokeWithDeps("bin", root, 3, deps)
	if contractFailure.OK || !strings.Contains(contractFailure.Error, "install dry-run result flags mismatch") || !strings.Contains(contractFailure.Error, "install dry-run wrote unexpected path") {
		t.Fatalf("unexpected contract failure: %+v", contractFailure)
	}
}

func validInstallDryRunResult() installDryRunSmokeResult {
	return installDryRunSmokeResult{
		OK:           true,
		DryRun:       true,
		ProjectLocal: true,
		Hosts: []installDryRunSmokeHost{
			{Host: "codex", OK: true, DryRun: true},
			{Host: "claude", OK: true, DryRun: true},
		},
		Files: []installDryRunSmokeFile{
			{Path: "configs/codex.toml", WouldWrite: true},
		},
		Links: []installDryRunSmokeLink{
			{Path: ".claude/skills/atomic-commit-push", WouldCreate: true},
		},
		SkillNames: []string{skillName},
	}
}

func mustMarshalInstallDryRunTest(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeInstallDryRunFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	body := "#!/bin/sh\nset -eu\ncase \"$*\" in\n" +
		"  \"install-native --dry-run --project-local --json\") printf '%s\\n' '" + mustMarshalInstallDryRunTest(t, validInstallDryRunResult()) + "' ;;\n" +
		"  *) echo \"unexpected fake harness args: $*\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
