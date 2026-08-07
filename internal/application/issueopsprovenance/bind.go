package issueopsprovenance

import (
	"context"
	"fmt"

	commandparsecontract "agent-harness/internal/contract/commandparse"
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
		return nil, commandparsecontract.NewGeneratedCommandProvenanceObservationError(fmt.Errorf("observer is unavailable"))
	}
	receipt, err := observer.Observe(ctx)
	if err != nil {
		return nil, commandparsecontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	evidence := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  generation,
	}
	for index, command := range commands {
		if command == "" {
			continue
		}
		bound[index], err = commandparsecontract.BindGeneratedCommand(command, evidence)
		if err != nil {
			return nil, err
		}
	}
	return bound, nil
}
