package catalog

import (
	"testing"

	"agent-harness/internal/core"
)

func TestToolsContextIsByteDeterministic(t *testing.T) {
	stable, _, err := core.ContextSerializationStable(func() any { return Tools() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp tools immutable prefix is not byte-deterministic across builds")
	}
}
