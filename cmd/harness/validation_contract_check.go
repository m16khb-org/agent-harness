package main

import (
	"encoding/json"
	"time"
)

func validateContractCheck(binary, root string) StepResult {
	return validateContractCheckWithDeps(binary, root, contractAuditWorkerValidationDeps{})
}

func validateContractCheckWithDeps(binary, root string, deps contractAuditWorkerValidationDeps) StepResult {
	deps = deps.withDefaults()
	step := deps.runCommandStep(root, "contract check", 30*time.Second, "", binary, "contract", "check", "--json")
	if !step.OK {
		return step
	}
	var result CompatibilityContract
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return failedStep("contract check", err)
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
	return assertionStep("contract check", time.Now(), errs)
}
