package main

import "time"

func validateMCP(binary, root string) StepResult {
	return validateMCPWithDeps(binary, root, mcpValidationDeps{})
}

func validateMCPWithDeps(binary, root string, deps mcpValidationDeps) StepResult {
	deps = deps.withDefaults()
	tempState, err := deps.mkdirTemp("", "agent-harness-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.removeAll(tempState)
	daemonDir, err := deps.mkdirTemp("", "ahd-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.removeAll(daemonDir)
	env := []string{
		"HARNESS_STATE_DIR=" + tempState,
		"HARNESS_DAEMON_DIR=" + daemonDir,
	}
	defer deps.runCommandStepEnv(root, "MCP daemon stop", 5*time.Second, "", env, binary, "daemon", "stop", "--json")

	step := deps.runCommandStepEnvWithBudget(root, "MCP smoke", 30*time.Second, mcpSmokeInput(), env, 0, binary, "mcp")
	if !step.OK {
		return step
	}
	validateMCPSmokeContract(&step)
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(step.Stdout, selfVerifyAggregateOutputBudgetBytes)
	return step
}
