package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RiskQATierPlan struct {
	Tier         string   `json:"tier"`
	ChangedPaths []string `json:"changed_paths"`
	Reasons      []string `json:"reasons"`
	Commands     []string `json:"commands"`
}

func validateRiskQATier(root string) StepResult {
	return validateRiskQATierWithDeps(root, riskQATierDeps{
		plan: planRiskQATier,
		run: func(root string, command string) StepResult {
			switch command {
			case "go test -race ./... -count=1":
				return runCommandStep(root, "risk QA race test", 180*time.Second, "", "go", "test", "-race", "./...", "-count=1")
			case "go vet ./...":
				return runCommandStep(root, "risk QA static vet", 120*time.Second, "", "go", "vet", "./...")
			default:
				return failedStep("risk QA tier", fmt.Errorf("unknown risk QA command %q", command))
			}
		},
	})
}

type riskQATierDeps struct {
	plan func(string) RiskQATierPlan
	run  func(root string, command string) StepResult
}

func validateRiskQATierWithDeps(root string, deps riskQATierDeps) StepResult {
	started := time.Now()
	plan := deps.plan(root)
	planJSON := riskQATierPlanJSON(plan)
	stdoutParts := []string{planJSON}
	commands := []string{}
	if len(plan.Commands) == 0 {
		return StepResult{
			Label:      "risk QA tier",
			OK:         true,
			DurationMS: time.Since(started).Milliseconds(),
			Stdout:     planJSON,
		}
	}
	for _, command := range plan.Commands {
		step := deps.run(root, command)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		if !step.OK {
			return combineFailedStep("risk QA tier", started, step, stdoutParts, commands)
		}
	}
	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "risk QA tier",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}

func planRiskQATier(root string) RiskQATierPlan {
	paths, warnings := gitChangedPaths(root)
	plan := planRiskQATierFromPaths(paths)
	plan.Reasons = append(plan.Reasons, warnings...)
	sort.Strings(plan.Reasons)
	return plan
}

func planRiskQATierFromPaths(paths []string) RiskQATierPlan {
	plan := RiskQATierPlan{Tier: "standard", ChangedPaths: uniqueSortedStrings(paths), Reasons: []string{}, Commands: []string{}}
	if len(plan.ChangedPaths) == 0 {
		plan.Reasons = append(plan.Reasons, "working tree has no local changes")
		return plan
	}
	goChanged := false
	sensitive := false
	for _, path := range plan.ChangedPaths {
		if strings.HasSuffix(path, ".go") {
			goChanged = true
		}
		if isRiskSensitivePath(path) {
			sensitive = true
		}
	}
	if goChanged {
		plan.Tier = "static"
		plan.Reasons = append(plan.Reasons, "go changes detected")
		plan.Commands = append(plan.Commands, "go vet ./...")
	}
	if goChanged && sensitive {
		plan.Tier = "elevated"
		plan.Reasons = append(plan.Reasons, "go changes touch policy, MCP, adapter, daemon, state, or harness orchestration surfaces")
		plan.Commands = append([]string{"go test -race ./... -count=1"}, plan.Commands...)
	}
	if !goChanged {
		plan.Reasons = append(plan.Reasons, "no Go changes detected; race/static tier skipped")
	}
	sort.Strings(plan.Reasons)
	return plan
}

func gitChangedPaths(root string) ([]string, []string) {
	cmd := exec.Command("git", "-C", root, "status", "--short", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, []string{"git status unavailable: " + err.Error()}
	}
	paths := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		path := parseGitStatusPath(line)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return uniqueSortedStrings(paths), nil
}

func parseGitStatusPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if len(line) > 3 {
		line = line[3:]
	} else {
		line = strings.TrimSpace(line)
	}
	if strings.Contains(line, " -> ") {
		parts := strings.Split(line, " -> ")
		line = parts[len(parts)-1]
	}
	line = strings.Trim(line, ` "`)
	return filepath.ToSlash(line)
}

func isRiskSensitivePath(path string) bool {
	path = filepath.ToSlash(path)
	if strings.HasPrefix(path, "cmd/harness/") || strings.HasPrefix(path, "internal/") {
		return true
	}
	for _, token := range []string{"daemon", "worker", "policy", "state", "mcp", "adapter", "install", "hook", "self_augment", "self-augment"} {
		if strings.Contains(path, token) {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func riskQATierPlanJSON(plan RiskQATierPlan) string {
	b, err := json.Marshal(plan)
	if err != nil {
		return fmt.Sprintf(`{"tier":%q,"error":%q}`, plan.Tier, err.Error())
	}
	return string(b)
}
