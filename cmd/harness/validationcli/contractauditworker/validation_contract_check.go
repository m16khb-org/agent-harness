package contractauditworker

import (
	"encoding/json"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

func ValidateContractCheck(binary, root string) StepResult {
	return ValidateContractCheckWithDeps(binary, root, ValidationDeps{})
}

func ValidateContractCheckWithDeps(binary, root string, deps ValidationDeps) StepResult {
	deps = deps.withDefaults()
	step := deps.RunCommandStep(root, "contract check", 30*time.Second, "", binary, "contract", "check", "--json")
	if !step.OK {
		return step
	}
	var result struct {
		OK          bool   `json:"ok"`
		Hash        string `json:"hash"`
		CLICommands []struct {
			Name string `json:"name"`
		} `json:"cli_commands"`
	}
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return commandstep.FailedStep("contract check", err)
	}
	errs := []string{}
	if !result.OK || result.Hash == "" {
		errs = append(errs, "contract did not pass or hash is empty")
	}
	for _, want := range []string{"worker", "contract", "policy"} {
		found := false
		for _, command := range result.CLICommands {
			if command.Name == want {
				found = true
			}
		}
		if !found {
			errs = append(errs, "missing CLI command "+want)
		}
	}
	return commandstep.AssertionStep("contract check", time.Now(), errs)
}
