package contractauditworker

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/internal/contract/failurecause"
)

func ValidateToolConformance(binary, root string) StepResult {
	return ValidateToolConformanceWithDeps(binary, root, ValidationDeps{})
}

func ValidateToolConformanceWithDeps(binary, root string, deps ValidationDeps) StepResult {
	deps = deps.withDefaults()
	step := deps.RunCommandStep(root, "tool contract conformance", 30*time.Second, "", binary, "contract", "conformance", "baseline", "--json")
	if !step.OK {
		step.FailureEvidence = []failurecause.Evidence{{Cause: failurecause.ContractInput, Code: "baseline_command_failed", Source: "tool_conformance"}}
		return step
	}
	var result struct {
		OK        bool `json:"ok"`
		CaseCount int  `json:"case_count"`
		Gate      struct {
			Decision string `json:"decision"`
		} `json:"gate"`
	}
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		failed := commandstep.FailedStep("tool contract conformance", err)
		failed.FailureEvidence = []failurecause.Evidence{{Cause: failurecause.ContractInput, Code: "baseline_response_invalid", Source: "tool_conformance"}}
		return failed
	}
	errs := []string{}
	if !result.OK {
		errs = append(errs, "tool conformance baseline did not pass")
	}
	if result.CaseCount < 10 {
		errs = append(errs, fmt.Sprintf("tool conformance baseline case_count=%d want>=10", result.CaseCount))
	}
	if result.Gate.Decision != "baseline_passed" {
		errs = append(errs, "tool conformance baseline gate is not baseline_passed")
	}
	validated := commandstep.AssertionStep("tool contract conformance", time.Now(), errs)
	if !validated.OK {
		validated.FailureEvidence = []failurecause.Evidence{{Cause: failurecause.ContractInput, Code: "baseline_contract_failed", Source: "tool_conformance"}}
	}
	return validated
}
