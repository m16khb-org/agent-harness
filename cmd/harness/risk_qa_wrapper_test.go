package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRiskQATierWrapperRunsElevatedDefaultCommands(t *testing.T) {
	root := t.TempDir()
	runRiskQATestCommand(t, root, "git", "init", "-q")
	writeFileForWrapperTest(t, filepath.Join(root, "cmd", "harness", "risk_qa.go"), "package main\n")
	runRiskQATestCommand(t, root, "git", "add", "cmd/harness/risk_qa.go")
	fakeBin := t.TempDir()
	writeFileForWrapperTest(t, filepath.Join(fakeBin, "go"), "#!/bin/sh\nprintf 'fake go %s\\n' \"$*\"\n")
	if err := os.Chmod(filepath.Join(fakeBin, "go"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	step := validateRiskQATier(root)
	if !step.OK || step.Label != "risk QA tier" {
		t.Fatalf("expected risk QA wrapper success, got %#v", step)
	}
	for _, want := range []string{"go test -race ./... -count=1", "go vet ./...", `"tier":"elevated"`, "fake go test -race ./... -count=1"} {
		if !strings.Contains(step.Command+"\n"+step.Stdout, want) {
			t.Fatalf("expected %q in command/stdout, got command=%q stdout=%q", want, step.Command, step.Stdout)
		}
	}
}

func TestPlanRiskQATierFromPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		tier     string
		commands []string
	}{
		{
			name:     "no changes",
			paths:    nil,
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "docs only",
			paths:    []string{".agent-harness/TESTING.md"},
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "go but not sensitive",
			paths:    []string{"examples/demo.go"},
			tier:     "static",
			commands: []string{"go vet ./..."},
		},
		{
			name:     "sensitive go",
			paths:    []string{"internal/core/policy.go", "cmd/harness/main.go"},
			tier:     "elevated",
			commands: []string{"go test -race ./... -count=1", "go vet ./..."},
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			plan := planRiskQATierFromPaths(tc.paths)
			if plan.Tier != tc.tier {
				t.Fatalf("tier=%q want %q: %+v", plan.Tier, tc.tier, plan)
			}
			if !sameStringSlice(plan.Commands, tc.commands) {
				t.Fatalf("commands=%+v want %+v", plan.Commands, tc.commands)
			}
		})
	}
}

func TestRiskQATierHelpersCoverGitWarningsAndJSON(t *testing.T) {
	nonGitRoot := t.TempDir()
	nonGitPlan := planRiskQATier(nonGitRoot)
	if nonGitPlan.Tier != "standard" || len(nonGitPlan.Commands) != 0 || len(nonGitPlan.Reasons) != 2 {
		t.Fatalf("unexpected non-git risk plan: %+v", nonGitPlan)
	}
	if !containsString(nonGitPlan.Reasons, "git status unavailable: exit status 128") || !containsString(nonGitPlan.Reasons, "working tree has no local changes") {
		t.Fatalf("non-git plan missing warnings: %+v", nonGitPlan.Reasons)
	}
	step := validateRiskQATier(nonGitRoot)
	if !step.OK || step.Label != "risk QA tier" || !strings.Contains(step.Stdout, `"tier":"standard"`) {
		t.Fatalf("unexpected no-command risk QA step: %+v", step)
	}

	gitRoot := t.TempDir()
	runStatusVerifyTestCommand(t, gitRoot, "git", "init")
	if plan := planRiskQATier(gitRoot); plan.Tier != "standard" || !containsString(plan.Reasons, "working tree has no local changes") {
		t.Fatalf("unexpected clean git risk plan: %+v", plan)
	}

	jsonText := riskQATierPlanJSON(RiskQATierPlan{Tier: "static", ChangedPaths: []string{"b.go", "a.go"}, Commands: []string{"go vet ./..."}})
	if !strings.Contains(jsonText, `"tier":"static"`) || !strings.Contains(jsonText, `"changed_paths":["b.go","a.go"]`) {
		t.Fatalf("unexpected risk QA JSON: %s", jsonText)
	}
}

func TestValidateRiskQATierWithDepsCoversCommandSuccessAndFailure(t *testing.T) {
	root := t.TempDir()
	plan := RiskQATierPlan{
		Tier:         "elevated",
		ChangedPaths: []string{"cmd/harness/risk_qa.go"},
		Reasons:      []string{"go changes detected"},
		Commands:     []string{"go test -race ./... -count=1", "go vet ./..."},
	}
	calls := []string{}
	success := validateRiskQATierWithDeps(root, riskQATierDeps{
		plan: func(gotRoot string) RiskQATierPlan {
			if gotRoot != root {
				t.Fatalf("plan root=%q want %q", gotRoot, root)
			}
			return plan
		},
		run: func(gotRoot string, command string) StepResult {
			calls = append(calls, command)
			return StepResult{Label: command, Command: "stub " + command, OK: true, Stdout: "ok " + command}
		},
	})
	if !success.OK || success.Command != "stub go test -race ./... -count=1 && stub go vet ./..." {
		t.Fatalf("unexpected successful risk QA result: %+v", success)
	}
	if !sameStringSlice(calls, plan.Commands) || !strings.Contains(success.Stdout, `"tier":"elevated"`) || !strings.Contains(success.Stdout, "ok go vet ./...") {
		t.Fatalf("risk QA success did not preserve command order/stdout: calls=%v result=%+v", calls, success)
	}

	failure := validateRiskQATierWithDeps(root, riskQATierDeps{
		plan: func(string) RiskQATierPlan { return plan },
		run: func(_ string, command string) StepResult {
			if command == "go test -race ./... -count=1" {
				return StepResult{Label: "risk QA race test", Command: "stub race", OK: false, Error: "race failed", Stdout: "race out"}
			}
			t.Fatalf("failure path should stop after first failing command, got %q", command)
			return StepResult{}
		},
	})
	if failure.OK || failure.Label != "risk QA tier" || !strings.Contains(failure.Command, "stub race") || !strings.Contains(failure.Error, "race failed") {
		t.Fatalf("unexpected failing risk QA result: %+v", failure)
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runRiskQATestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
