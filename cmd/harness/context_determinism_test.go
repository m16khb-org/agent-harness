package main

import (
	"testing"

	"agent-harness/internal/core"
)

// TestReusableContextBuildersAreByteDeterministic enforces the Reasonix
// immutable-prefix contract on every builder whose output an agent or host
// reuses as a stable context prefix. The response-contract golden masks
// volatile fields with $TIMESTAMP, so it cannot catch ordering or content
// drift on its own; this asserts the stable projection is byte-identical
// across repeated builds of unchanged inputs.
func TestReusableContextBuildersAreByteDeterministic(t *testing.T) {
	builders := map[string]func() any{
		"contract_schema": func() any { return compatibilityContract() },
		"mcp_tools":       func() any { return mcpTools() },
		"mcp_resources":   func() any { return mcpResources() },
	}
	for name, build := range builders {
		stable, _, err := core.ContextSerializationStable(build)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !stable {
			t.Fatalf("%s immutable prefix is not byte-deterministic across builds", name)
		}
	}
}
