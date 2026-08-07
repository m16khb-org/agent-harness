package contractcli

import (
	contextregion "agent-harness/internal/domain/contextregion"
	"testing"
)

func TestCompatibilityContractContextIsByteDeterministic(t *testing.T) {
	stable, _, err := contextregion.ContextSerializationStable(func() any { return BuildCompatibilityContract() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("contract_schema immutable prefix is not byte-deterministic across builds")
	}
}
