package resources

import (
	"testing"

	"agent-harness/internal/adapter/core"
)

func TestResourcesContextIsByteDeterministic(t *testing.T) {
	stable, _, err := core.ContextSerializationStable(func() any { return MCPResources() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp resources immutable prefix is not byte-deterministic across builds")
	}
}
