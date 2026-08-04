package issueopsprovenance

import (
	"context"
	"fmt"

	issueopscontract "agent-harness/internal/contract/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func Bind(ctx context.Context, command string, generation uint64, observer provenanceport.Observer) (string, error) {
	bound, err := BindMany(ctx, []string{command}, generation, observer)
	if err != nil {
		return "", err
	}
	return bound[0], nil
}

func BindMany(ctx context.Context, commands []string, generation uint64, observer provenanceport.Observer) ([]string, error) {
	bound := append([]string(nil), commands...)
	hasCommand := false
	for _, command := range commands {
		hasCommand = hasCommand || command != ""
	}
	if !hasCommand {
		return bound, nil
	}
	if observer == nil {
		return nil, issueopscontract.NewGeneratedCommandProvenanceObservationError(fmt.Errorf("observer is unavailable"))
	}
	receipt, err := observer.Observe(ctx)
	if err != nil {
		return nil, issueopscontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	evidence := issueopscontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  generation,
	}
	for index, command := range commands {
		if command == "" {
			continue
		}
		bound[index], err = issueopscontract.BindGeneratedCommand(command, evidence)
		if err != nil {
			return nil, err
		}
	}
	return bound, nil
}
