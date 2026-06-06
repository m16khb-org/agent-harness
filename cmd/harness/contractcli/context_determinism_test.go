package contractcli

import (
	"testing"

	"agent-harness/internal/core"
)

func TestCompatibilityContractContextIsByteDeterministic(t *testing.T) {
	stable, _, err := core.ContextSerializationStable(func() any { return BuildCompatibilityContract() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("contract_schema immutable prefix is not byte-deterministic across builds")
	}
}
