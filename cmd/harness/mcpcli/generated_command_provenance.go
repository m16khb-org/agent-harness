package mcpcli

import (
	"context"

	provenanceapp "agent-harness/internal/application/issueopsprovenance"
	"agent-harness/internal/core/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func bindMCPIssueOpsExecutionNextCommand(value any, observer provenanceport.Observer) (any, error) {
	command, generation, replace := mcpExecutionNextCommand(value)
	bound, err := provenanceapp.Bind(context.Background(), command, generation, observer)
	if err != nil {
		return nil, err
	}
	if command == "" {
		return value, nil
	}
	return replace(bound), nil
}

func mcpExecutionNextCommand(value any) (string, uint64, func(string) any) {
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
	case issueops.ExecutionSyncBaseResult:
		return result.NextCommand, result.LeaseGeneration, func(command string) any {
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
