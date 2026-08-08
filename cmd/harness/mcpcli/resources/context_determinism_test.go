package resources

import (
	contextregion "agent-harness/internal/domain/contextregion"
	"testing"
)

func TestResourcesContextIsByteDeterministic(t *testing.T) {
	stable, _, err := contextregion.ContextSerializationStable(func() any { return MCPResources() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp resources immutable prefix is not byte-deterministic across builds")
	}
}
