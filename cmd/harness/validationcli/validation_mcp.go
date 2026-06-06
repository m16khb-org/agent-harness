package validationcli

import "time"

func ValidateMCP(binary, root string) StepResult {
	return ValidateMCPWithDeps(binary, root, MCPValidationDeps{})
}

func ValidateMCPWithDeps(binary, root string, deps MCPValidationDeps) StepResult {
	deps = deps.withDefaults()
	tempState, err := deps.MkdirTemp("", "agent-harness-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.RemoveAll(tempState)
	daemonDir, err := deps.MkdirTemp("", "ahd-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer deps.RemoveAll(daemonDir)
	env := []string{
		"HARNESS_STATE_DIR=" + tempState,
		"HARNESS_DAEMON_DIR=" + daemonDir,
	}
	defer deps.RunCommandStepEnv(root, "MCP daemon stop", 5*time.Second, "", env, binary, "daemon", "stop", "--json")

	step := deps.RunCommandStepEnvWithBudget(root, "MCP smoke", 30*time.Second, MCPSmokeInput(), env, 0, binary, "mcp")
	if !step.OK {
		return step
	}
	ValidateMCPSmokeContract(&step)
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(step.Stdout, selfVerifyAggregateOutputBudgetBytes)
	return step
}
