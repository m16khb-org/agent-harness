package commandpolicy

import (
	"encoding/json"
	policy "issueops/internal/contract/policy"
	"path/filepath"
)

type commandPolicyValidationCheck struct {
	label    string
	name     string
	args     []string
	validate func(stdout string) []string
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func commandPolicyChecks(binary, tempWorkspace, outside string) []commandPolicyValidationCheck {
	return []commandPolicyValidationCheck{
		{
			label: "policy allow",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "git", "status", "--short"},
			validate: func(stdout string) []string {
				var allowedEval policy.CommandPolicyEvaluation
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
				var outsideEval policy.CommandPolicyEvaluation
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
				var outsidePathEval policy.CommandPolicyEvaluation
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
				var shellEval policy.CommandPolicyEvaluation
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
				var fakeResult policy.CommandFakeRunResult
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
