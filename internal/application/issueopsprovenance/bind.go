package issueopsprovenance

import (
	"context"
	"fmt"

	issueopscontract "agent-harness/internal/contract/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func Bind(ctx context.Context, command string, generation uint64, observer provenanceport.Observer) (string, error) {
	if command == "" {
		return "", nil
	}
	if observer == nil {
		return "", issueopscontract.NewGeneratedCommandProvenanceObservationError(fmt.Errorf("observer is unavailable"))
	}
	receipt, err := observer.Observe(ctx)
	if err != nil {
		return "", issueopscontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	evidence := issueopscontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  generation,
	}
	return issueopscontract.BindGeneratedCommand(command, evidence)
}
