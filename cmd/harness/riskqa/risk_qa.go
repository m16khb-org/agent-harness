package riskqa

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

const (
	selfVerifyCommandOutputBudgetBytes   = 32 * 1024
	selfVerifyAggregateOutputBudgetBytes = 8 * 1024
	riskQARaceTimeout                    = 10 * time.Minute
	riskQAVetTimeout                     = 120 * time.Second
	fullRaceCommand                      = "go test -race ./... -count=1"
)

type RiskQATierPlan struct {
	Tier         string   `json:"tier"`
	ChangedPaths []string `json:"changed_paths"`
	Reasons      []string `json:"reasons"`
	Commands     []string `json:"commands"`
}

type StepResult = commandstep.StepResult

func Validate(root string) StepResult {
	return ValidateWithDeps(root, defaultDeps())
}

func ValidateForSelfVerify(root string) (StepResult, bool) {
	deps := defaultDeps()
	plan := deps.Plan(root)
	deps.Plan = func(string) RiskQATierPlan { return plan }
	return ValidateWithDeps(root, deps), slices.Contains(plan.Commands, fullRaceCommand)
}

func defaultDeps() Deps {
	return Deps{
		Plan: Plan,
		Run: func(root string, command string) StepResult {
			switch command {
			case fullRaceCommand:
				return commandstep.Run(root, "risk QA race test", riskQARaceTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "test", "-race", "./...", "-count=1")
			case "go vet ./...":
				return commandstep.Run(root, "risk QA static vet", riskQAVetTimeout, "", selfVerifyCommandOutputBudgetBytes, "go", "vet", "./...")
			default:
				return commandstep.FailedStep("risk QA tier", fmt.Errorf("unknown risk QA command %q", command))
			}
		},
	}
}

type Deps struct {
	Plan func(string) RiskQATierPlan
	Run  func(root string, command string) StepResult
}

func ValidateWithDeps(root string, deps Deps) StepResult {
	started := time.Now()
	plan := deps.Plan(root)
	planJSON := PlanJSON(plan)
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
		step := deps.Run(root, command)
		commands = append(commands, step.Command)
		stdoutParts = append(stdoutParts, step.Stdout)
		if !step.OK {
			return commandstep.CombineFailedStep("risk QA tier", started, step, stdoutParts, commands, selfVerifyAggregateOutputBudgetBytes)
		}
	}
	stdoutText, stdoutTruncated, stdoutBytes := commandstep.TailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
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
