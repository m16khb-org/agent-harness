package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type commandPolicyCommandRunner func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult

type commandPolicyValidationDeps struct {
	makeTempDir func(kind string) (string, error)
	removeAll   func(path string) error
	exists      func(path string) bool
	run         commandPolicyCommandRunner
}

func (deps commandPolicyValidationDeps) withDefaults() commandPolicyValidationDeps {
	if deps.makeTempDir == nil {
		deps.makeTempDir = func(kind string) (string, error) {
			switch kind {
			case "workspace":
				return os.MkdirTemp("", "agent-harness-policy-*")
			case "outside":
				return os.MkdirTemp("", "agent-harness-policy-outside-*")
			default:
				return "", fmt.Errorf("unknown command policy temp kind: %s", kind)
			}
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = runCommandStep
	}
	return deps
}

func validateCommandPolicy(binary, root string) StepResult {
	return validateCommandPolicyWithDeps(binary, root, commandPolicyValidationDeps{})
}

func validateCommandPolicyWithDeps(binary, root string, deps commandPolicyValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempWorkspace, err := deps.makeTempDir("workspace")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer deps.removeAll(tempWorkspace)
	outside, err := deps.makeTempDir("outside")
	if err != nil {
		return failedStep("command policy smoke", err)
	}
	defer deps.removeAll(outside)

	stdoutParts := []string{}
	commands := []string{}
	for _, check := range commandPolicyChecks(binary, tempWorkspace, outside) {
		step := deps.run(root, check.label, 30*time.Second, "", check.name, check.args...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		if !step.OK {
			return combineFailedStep("command policy smoke", started, step, stdoutParts, commands)
		}
		if errs := check.validate(step.Stdout); len(errs) > 0 {
			return assertionStepWithOutput("command policy smoke", started, errs, stdoutParts, commands)
		}
	}
	marker := filepath.Join(tempWorkspace, "marker")
	if deps.exists(marker) {
		return assertionStepWithOutput("command policy smoke", started, []string{"fake-run created marker; command executed unexpectedly"}, stdoutParts, commands)
	}

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "command policy smoke",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

type commandPolicyValidationCheck struct {
	label    string
	name     string
	args     []string
	validate func(stdout string) []string
}

func commandPolicyChecks(binary, tempWorkspace, outside string) []commandPolicyValidationCheck {
	return []commandPolicyValidationCheck{
		{
			label: "policy allow",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "git", "status", "--short"},
			validate: func(stdout string) []string {
				var allowedEval core.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(stdout), &allowedEval); err != nil {
					return []string{err.Error()}
				}
				if !allowedEval.OK || !allowedEval.Allowed {
					return []string{"read-only git status was not allowed"}
				}
				return nil
			},
		},
		{
			label: "policy deny outside",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", outside, "--", "git", "status", "--short"},
			validate: func(stdout string) []string {
				var outsideEval core.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(stdout), &outsideEval); err != nil {
					return []string{err.Error()}
				}
				if outsideEval.Allowed || !containsString(outsideEval.DenyReasons, "cwd_outside_workspace") {
					return []string{"outside cwd was not denied"}
				}
				return nil
			},
		},
		{
			label: "policy deny outside path arg",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "cat", filepath.Join(outside, "note.txt")},
			validate: func(stdout string) []string {
				var outsidePathEval core.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(stdout), &outsidePathEval); err != nil {
					return []string{err.Error()}
				}
				if outsidePathEval.Allowed || !containsString(outsidePathEval.DenyReasons, "path_outside_workspace") {
					return []string{"outside path arg was not denied"}
				}
				return nil
			},
		},
		{
			label: "policy deny shell",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "sh", "-c", "echo ok"},
			validate: func(stdout string) []string {
				var shellEval core.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(stdout), &shellEval); err != nil {
					return []string{err.Error()}
				}
				if shellEval.Allowed || !containsString(shellEval.DenyReasons, "shell_interpreter_not_allowed") {
					return []string{"shell command was not denied"}
				}
				return nil
			},
		},
		{
			label: "policy fake-run",
			name:  binary,
			args:  []string{"policy", "fake-run", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--write", "--", "touch", "marker"},
			validate: func(stdout string) []string {
				var fakeResult core.CommandFakeRunResult
				if err := json.Unmarshal([]byte(stdout), &fakeResult); err != nil {
					return []string{err.Error()}
				}
				if !fakeResult.OK || fakeResult.Executed || !fakeResult.Policy.Allowed {
					return []string{"fake-run did not report accepted non-execution"}
				}
				return nil
			},
		},
	}
}
