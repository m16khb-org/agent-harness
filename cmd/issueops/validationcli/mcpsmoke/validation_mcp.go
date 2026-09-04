package mcpsmoke

import "time"

func ValidateMCP(binary, root string) StepResult {
	return ValidateMCPWithDeps(binary, root, MCPValidationDeps{})
}

func ValidateMCPWithDeps(binary, root string, deps MCPValidationDeps) StepResult {
	deps = deps.withDefaults()
	tempState, err := deps.MkdirTemp("", "issueops-mcp-state-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer func() { _ = deps.RemoveAll(tempState) }()
	daemonDir, err := deps.MkdirTemp("", "ahd-*")
	if err != nil {
		return failedStep("MCP smoke", err)
	}
	defer func() { _ = deps.RemoveAll(daemonDir) }()
	env := []string{
		"ISSUEOPS_STATE_DIR=" + tempState,
		"ISSUEOPS_DAEMON_DIR=" + daemonDir,
		"ISSUEOPS_MCP_DIRECT=1",
	}
	defer deps.RunCommandStepEnv(root, "MCP daemon stop", 5*time.Second, "", env, binary, "daemon", "stop", "--json")

	step := deps.RunSDKSmoke(root, binary, env, 30*time.Second)
	if !step.OK {
		return step
	}
	ValidateMCPSmokeContract(&step)
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = tailWithBudget(step.Stdout, aggregateOutputBudgetBytes)
	return step
}
