package mcpcli

import (
	"testing"

	"agent-harness/internal/core"
)

func TestMCPCatalogContextsAreByteDeterministic(t *testing.T) {
	builders := map[string]func() any{
		"mcp_tools":     func() any { return MCPTools() },
		"mcp_resources": func() any { return MCPResources() },
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
