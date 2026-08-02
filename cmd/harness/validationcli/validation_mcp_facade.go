package validationcli

import "agent-harness/cmd/harness/validationcli/mcpsmoke"

type MCPValidationDeps = mcpsmoke.MCPValidationDeps

func ValidateMCP(binary, root string) StepResult {
	return mcpsmoke.ValidateMCP(binary, root)
}

func ValidateMCPWithDeps(binary, root string, deps MCPValidationDeps) StepResult {
	return mcpsmoke.ValidateMCPWithDeps(binary, root, deps)
}

func ValidateMCPSmokeContract(step *StepResult) {
	mcpsmoke.ValidateMCPSmokeContract(step)
}

func MCPSmokeHasExpectedMarkers(stdout string) bool {
	return mcpsmoke.MCPSmokeHasExpectedMarkers(stdout)
}

func MCPSmokeExpectedMarkers() []string {
	return mcpsmoke.MCPSmokeExpectedMarkers()
}
