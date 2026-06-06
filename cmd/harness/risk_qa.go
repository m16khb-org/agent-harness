package main

import (
	"fmt"
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
