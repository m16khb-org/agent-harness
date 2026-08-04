package executioncmd

import (
	"context"
	"errors"

	provenanceapp "agent-harness/internal/application/issueopsprovenance"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func bindExecutionNextCommand(value any, observer provenanceport.Observer) (any, error) {
	if result, ok := value.(issueops.ExecutionSyncBaseResult); ok {
		bound, err := provenanceapp.BindMany(
			context.Background(),
			[]string{result.NextCommand, result.AbortCommand},
			result.LeaseGeneration,
			observer,
		)
		if err != nil {
			return nil, err
		}
		result.NextCommand, result.AbortCommand = bound[0], bound[1]
		return result, nil
	}
	command, generation, replace := executionNextCommand(value)
	if command == "" {
		return value, nil
	}
	bound, err := provenanceapp.Bind(context.Background(), command, generation, observer)
	if err != nil {
		return nil, err
	}
	return replace(bound), nil
}

func executionNextCommand(value any) (string, uint64, func(string) any) {
	switch result := value.(type) {
	case issueops.ExecutionPrepareResult:
		generation := uint64(0)
		if result.Execution != nil {
			generation = result.Execution.Lease.Generation
		}
		return result.NextCommand, generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueops.ExecutionResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueops.ExecutionReplaceResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueops.ExecutionResumeResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueops.ExecutionSwitchModeResult:
		return result.NextCommand, result.LeaseGeneration, func(command string) any {
			result.NextCommand = command
			return result
		}
	default:
		return "", 0, func(string) any { return value }
	}
}

func bindExecutionErrorNextCommand(err error, observer provenanceport.Observer) error {
	var typed *issueopscontract.BaseSyncRequiredError
	if !errors.As(err, &typed) {
		return err
	}
	bound, bindErr := provenanceapp.Bind(context.Background(), typed.NextCommand, typed.CompletionGeneration, observer)
	if bindErr != nil {
		return bindErr
	}
	typed.NextCommand = bound
	return err
}
