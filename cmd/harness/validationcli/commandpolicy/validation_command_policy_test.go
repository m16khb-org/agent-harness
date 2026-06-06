package commandpolicy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
)

func TestValidateCommandPolicyWrapperUsesExecutableSurface(t *testing.T) {
	root := t.TempDir()
	binary := writeCommandPolicyFakeBinary(t, root)

	step := validateCommandPolicy(binary, root)
	if !step.OK || !strings.Contains(step.Command, "policy check") || !strings.Contains(step.Command, "policy fake-run") {
		t.Fatalf("expected wrapper success, got %+v", step)
	}
}

func TestValidateCommandPolicyWithDepsCoversSuccessAndSetupFailure(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	calls := []string{}
	deps := commandPolicyValidationDeps{
		makeTempDir: func(kind string) (string, error) {
			switch kind {
			case "workspace":
				return workspace, nil
			case "outside":
				return outside, nil
			default:
				t.Fatalf("unexpected temp kind: %s", kind)
			}
			return "", nil
		},
		removeAll: func(string) error { return nil },
		exists:    func(string) bool { return false },
		run: func(dir, label string, timeout time.Duration, stdin, name string, args ...string) StepResult {
			if dir != root || timeout != 30*time.Second || stdin != "" {
				t.Fatalf("unexpected command envelope: dir=%q label=%q timeout=%s stdin=%q", dir, label, timeout, stdin)
			}
			command := strings.Join(append([]string{name}, args...), " ")
			calls = append(calls, label+":"+command)
			switch label {
			case "policy allow":
				return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: true})
			case "policy deny outside":
				return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"cwd_outside_workspace"}})
			case "policy deny outside path arg":
				return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"path_outside_workspace"}})
			case "policy deny shell":
				return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"shell_interpreter_not_allowed"}})
			case "policy fake-run":
				return policyStep(label, command, core.CommandFakeRunResult{OK: true, Executed: false, Policy: core.CommandPolicyEvaluation{OK: true, Allowed: true}})
			default:
				t.Fatalf("unexpected label: %s", label)
			}
			return StepResult{}
		},
	}
	step := validateCommandPolicyWithDeps("bin/agent-harness", root, deps)
	if !step.OK || len(calls) != 5 || !strings.Contains(step.Command, "policy allow:bin/agent-harness policy check") || !strings.Contains(step.Command, "policy fake-run:bin/agent-harness policy fake-run") {
		t.Fatalf("unexpected command policy success: step=%+v calls=%v", step, calls)
	}

	deps.makeTempDir = func(string) (string, error) { return "", errors.New("temp fail") }
	failed := validateCommandPolicyWithDeps("bin", root, deps)
	if failed.OK || failed.Error != "temp fail" {
		t.Fatalf("unexpected temp failure: %+v", failed)
	}
}

func TestValidateCommandPolicyWithDepsCoversCommandParseAndContractFailures(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	deps := commandPolicyValidationDeps{
		makeTempDir: func(kind string) (string, error) {
			if kind == "workspace" {
				return workspace, nil
			}
			return outside, nil
		},
		removeAll: func(string) error { return nil },
		exists:    func(string) bool { return false },
	}

	deps.run = func(string, string, time.Duration, string, string, ...string) StepResult {
		return StepResult{Label: "policy allow", Command: "allow", OK: false, Error: "boom"}
	}
	commandFailure := validateCommandPolicyWithDeps("bin", root, deps)
	if commandFailure.OK || !strings.Contains(commandFailure.Error, "policy allow: boom") {
		t.Fatalf("unexpected command failure: %+v", commandFailure)
	}

	deps.run = func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) StepResult {
		return StepResult{Label: label, Command: label, OK: true, Stdout: "{"}
	}
	parseFailure := validateCommandPolicyWithDeps("bin", root, deps)
	if parseFailure.OK || parseFailure.Error == "" {
		t.Fatalf("expected JSON parse failure, got %+v", parseFailure)
	}

	deps.run = func(_ string, label string, _ time.Duration, _ string, _ string, _ ...string) StepResult {
		switch label {
		case "policy allow":
			return policyStep(label, label, core.CommandPolicyEvaluation{OK: true, Allowed: false})
		default:
			return policyStep(label, label, core.CommandPolicyEvaluation{OK: true, Allowed: false})
		}
	}
	allowContract := validateCommandPolicyWithDeps("bin", root, deps)
	if allowContract.OK || !strings.Contains(allowContract.Error, "read-only git status was not allowed") {
		t.Fatalf("unexpected allow contract failure: %+v", allowContract)
	}

	deps.run = validCommandPolicyRunner(t)
	deps.exists = func(path string) bool {
		return strings.HasSuffix(path, "marker")
	}
	markerFailure := validateCommandPolicyWithDeps("bin", root, deps)
	if markerFailure.OK || !strings.Contains(markerFailure.Error, "fake-run created marker") {
		t.Fatalf("unexpected marker failure: %+v", markerFailure)
	}
}

func validCommandPolicyRunner(t *testing.T) commandPolicyCommandRunner {
	t.Helper()
	return func(_ string, label string, _ time.Duration, _ string, name string, args ...string) StepResult {
		command := strings.Join(append([]string{name}, args...), " ")
		switch label {
		case "policy allow":
			return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: true})
		case "policy deny outside":
			return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"cwd_outside_workspace"}})
		case "policy deny outside path arg":
			return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"path_outside_workspace"}})
		case "policy deny shell":
			return policyStep(label, command, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"shell_interpreter_not_allowed"}})
		case "policy fake-run":
			return policyStep(label, command, core.CommandFakeRunResult{OK: true, Executed: false, Policy: core.CommandPolicyEvaluation{OK: true, Allowed: true}})
		default:
			t.Fatalf("unexpected label: %s", label)
		}
		return StepResult{}
	}
}

func policyStep(tLabel, command string, value any) StepResult {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return StepResult{Label: tLabel, Command: tLabel + ":" + command, OK: true, Stdout: string(b)}
}

func writeCommandPolicyFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-harness")
	allowed := mustMarshalCommandPolicyTest(t, core.CommandPolicyEvaluation{OK: true, Allowed: true})
	cwdOutside := mustMarshalCommandPolicyTest(t, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"cwd_outside_workspace"}})
	pathOutside := mustMarshalCommandPolicyTest(t, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"path_outside_workspace"}})
	shellDenied := mustMarshalCommandPolicyTest(t, core.CommandPolicyEvaluation{OK: true, Allowed: false, DenyReasons: []string{"shell_interpreter_not_allowed"}})
	fakeRun := mustMarshalCommandPolicyTest(t, core.CommandFakeRunResult{OK: true, Executed: false, Policy: core.CommandPolicyEvaluation{OK: true, Allowed: true}})
	body := "#!/bin/sh\nset -eu\ncase \"$*\" in\n" +
		"  policy\\ check*agent-harness-policy-outside-*\" -- git status --short\") printf '%s\\n' '" + cwdOutside + "' ;;\n" +
		"  policy\\ check*\" -- cat \"*agent-harness-policy-outside-*) printf '%s\\n' '" + pathOutside + "' ;;\n" +
		"  policy\\ check*\" -- sh -c echo ok\") printf '%s\\n' '" + shellDenied + "' ;;\n" +
		"  policy\\ check*\" -- git status --short\") printf '%s\\n' '" + allowed + "' ;;\n" +
		"  policy\\ fake-run*) printf '%s\\n' '" + fakeRun + "' ;;\n" +
		"  *) echo \"unexpected fake harness args: $*\" >&2; exit 2 ;;\n" +
		"esac\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustMarshalCommandPolicyTest(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
