package executioncmd

import (
	"context"
	"errors"

	provenanceapp "agent-harness/internal/application/issueopsprovenance"
	issueopscontract "agent-harness/internal/contract/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func bindExecutionNextCommand(value any, observer provenanceport.Observer) (any, error) {
	if result, ok := value.(issueopscontract.ExecutionSyncBaseResult); ok {
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
	// generation 0은 아직 lease가 없다는 뜻이다 — prepare preview가 그 경우다.
	// provenance는 "이 generation의 lease에 결속된 명령"을 표현하므로 결속할
	// 대상이 없으면 붙일 것이 없다. 억지로 붙이면 Validate가 generation 0을
	// 거부해 preview 자체가 실패하고, Orca 준비 경로가 통째로 막힌다(#411).
	//
	// unbound 명령은 hook의 기존 executable 계약(PATH·repo-relative form)으로
	// 정상 분류되므로 안전 경계가 낮아지지 않는다. absolute form만 provenance를
	// 요구하는데, 그 요구는 lease가 생긴 뒤의 명령에 그대로 남는다.
	if generation == 0 {
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
	case issueopscontract.ExecutionPrepareResult:
		generation := uint64(0)
		if result.Execution != nil {
			generation = result.Execution.Lease.Generation
		}
		return result.NextCommand, generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueopscontract.ExecutionResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueopscontract.ExecutionReplaceResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueopscontract.ExecutionResumeResult:
		return result.NextCommand, result.Execution.Lease.Generation, func(command string) any {
			result.NextCommand = command
			return result
		}
	case issueopscontract.ExecutionSwitchModeResult:
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
