package executioncmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type provenanceObserverStub struct {
	evidence provenanceport.Receipt
	err      error
}

func (s provenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, s.err
}

type countingProvenanceObserver struct {
	calls int
}

func (s *countingProvenanceObserver) Observe(context.Context) (provenanceport.Receipt, error) {
	s.calls++
	return provenanceport.Receipt{
		ExecutablePath:   fmt.Sprintf("/repo/bin/agent-harness-%d", s.calls),
		ExecutableSHA256: strings.Repeat(fmt.Sprint(s.calls), 64),
	}, nil
}

func TestBindExecutionNextCommandUsesObservedBinaryAndResultGeneration(t *testing.T) {
	raw := issueops.ExecutionReplaceResult{
		Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 7}},
		NextCommand: "agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm",
	}
	bound, err := bindExecutionNextCommand(raw, provenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := bound.(issueops.ExecutionReplaceResult)
	if !ok {
		t.Fatalf("bound result type = %T", bound)
	}
	if !strings.Contains(result.NextCommand, "--generated-for-generation 7") {
		t.Fatalf("bound next command = %q", result.NextCommand)
	}
}

func TestBindExecutionSyncBaseBindsConflictAbortWithSameProvenance(t *testing.T) {
	observer := &countingProvenanceObserver{}
	bound, err := bindExecutionNextCommand(issueops.ExecutionSyncBaseResult{
		LeaseGeneration: 7,
		NextCommand:     "agent-harness issueops execution sync-base --id io-1 --finalize",
		AbortCommand:    "agent-harness issueops execution sync-base --id io-1 --abort",
	}, observer)
	if err != nil {
		t.Fatal(err)
	}
	result := bound.(issueops.ExecutionSyncBaseResult)
	for name, command := range map[string]string{"next_command": result.NextCommand, "abort_command": result.AbortCommand} {
		if !strings.HasPrefix(command, "'/repo/bin/agent-harness-1'") ||
			!strings.Contains(command, "--generated-for-generation 7") ||
			!strings.Contains(command, "--generated-by-sha256 "+strings.Repeat("1", 64)) {
			t.Fatalf("%s is not bound to the shared observation: %q", name, command)
		}
	}
	if observer.calls != 1 {
		t.Fatalf("provenance observations=%d want=1", observer.calls)
	}
}

func TestOutputBindsBaseSyncRequiredTypedErrorCommand(t *testing.T) {
	observer := provenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}}
	err := output(nil, true, issueopscontract.NewBaseSyncRequiredError("io-1", 7), Deps{
		Provenance: observer,
		PrintError: func(error) error { return nil },
	})
	var typed *issueopscontract.BaseSyncRequiredError
	if !errors.As(err, &typed) {
		t.Fatalf("error type=%T error=%v", err, err)
	}
	if !strings.HasPrefix(typed.NextCommand, "'/repo/bin/agent-harness'") ||
		!strings.Contains(typed.NextCommand, "--generated-for-generation 7") ||
		!strings.Contains(typed.NextCommand, "--generated-by-sha256 "+strings.Repeat("a", 64)) {
		t.Fatalf("typed error next_command is unbound: %q", typed.NextCommand)
	}
}

func TestOutputBaseSyncRequiredObservationFailureHasNoUnboundFallback(t *testing.T) {
	err := output(nil, true, issueopscontract.NewBaseSyncRequiredError("io-1", 7), Deps{
		Provenance: provenanceObserverStub{err: errors.New("observation failed")},
		PrintError: func(error) error { return nil },
	})
	if err == nil {
		t.Fatal("observation failure exposed the typed error command")
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("observation failure is not structured: %T %v", err, err)
	}
	var typed *issueopscontract.BaseSyncRequiredError
	if errors.As(err, &typed) {
		t.Fatalf("observation failure returned unbound next_command: %q", typed.NextCommand)
	}
}

func TestBindExecutionNextCommandObservationFailureHasNoFallback(t *testing.T) {
	raw := issueops.ExecutionReplaceResult{
		Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 7}},
		NextCommand: "agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm",
	}
	bound, err := bindExecutionNextCommand(raw, provenanceObserverStub{err: errors.New("executable observation failed")})
	if err == nil {
		t.Fatal("observation failure must reject the generated command")
	}
	if bound != nil {
		t.Fatalf("observation failure exposed an unbound result: %#v", bound)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("observation failure is not structured: %T %v", err, err)
	}
}

func TestBindExecutionNextCommandMissingObserverHasNoFallback(t *testing.T) {
	raw := issueops.ExecutionReplaceResult{
		Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 7}},
		NextCommand: "agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm",
	}
	bound, err := bindExecutionNextCommand(raw, nil)
	if err == nil || bound != nil {
		t.Fatalf("missing observer bound=%#v err=%v", bound, err)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("missing observer failure is not structured: %T %v", err, err)
	}
}

func TestBindExecutionNextCommandCoversEveryCommandBearingResult(t *testing.T) {
	observer := provenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}}
	tests := []struct {
		name       string
		generation uint64
		value      any
		command    func(any) string
	}{
		{
			name: "prepare", generation: 3,
			value: issueops.ExecutionPrepareResult{
				Execution:   &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 3}},
				NextCommand: "agent-harness issueops execution claim --id io-1 --generation 3",
			},
			command: func(value any) string { return value.(issueops.ExecutionPrepareResult).NextCommand },
		},
		{
			name: "sync-base", generation: 4,
			value: issueops.ExecutionSyncBaseResult{
				LeaseGeneration: 4,
				NextCommand:     "agent-harness issueops execution sync-base --id io-1 --apply",
			},
			command: func(value any) string { return value.(issueops.ExecutionSyncBaseResult).NextCommand },
		},
		{
			name: "switch-mode preview", generation: 5,
			value: issueops.ExecutionSwitchModeResult{
				LeaseGeneration: 5,
				NextCommand:     "agent-harness issueops execution switch-mode --id io-1 --apply",
			},
			command: func(value any) string { return value.(issueops.ExecutionSwitchModeResult).NextCommand },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bound, err := bindExecutionNextCommand(test.value, observer)
			if err != nil {
				t.Fatal(err)
			}
			command := test.command(bound)
			if !strings.HasPrefix(command, "'/repo/bin/agent-harness'") || !strings.Contains(command, "--generated-for-generation "+fmt.Sprint(test.generation)) {
				t.Fatalf("bound command = %q", command)
			}
		})
	}
}

func TestBindExecutionNextCommandCommandBearingResultsFailClosedWithoutObserver(t *testing.T) {
	values := []any{
		issueops.ExecutionPrepareResult{
			Execution:   &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 3}},
			NextCommand: "agent-harness issueops execution claim --id io-1 --generation 3",
		},
		issueops.ExecutionSyncBaseResult{
			LeaseGeneration: 4,
			NextCommand:     "agent-harness issueops execution sync-base --id io-1 --apply",
		},
		issueops.ExecutionSwitchModeResult{
			LeaseGeneration: 5,
			NextCommand:     "agent-harness issueops execution switch-mode --id io-1 --apply",
		},
	}
	for _, value := range values {
		bound, err := bindExecutionNextCommand(value, nil)
		if err == nil || bound != nil {
			t.Fatalf("missing observer for %T bound=%#v err=%v", value, bound, err)
		}
	}
}

// TestBindExecutionPrepareLeavesPreviewCommandUnbound는 #411을 고정한다.
//
// prepare preview 시점에는 execution이 아직 없어 lease generation이 0이다.
// provenance는 "이 generation의 lease에 결속된 명령"을 표현하므로 결속할 대상이
// 없는 명령에 붙일 수 없고, 붙이려 하면 Validate가 generation 0을 거부해
// prepare 자체가 실패한다 — Orca 준비 경로 전체가 막혔다.
func TestBindExecutionPrepareLeavesPreviewCommandUnbound(t *testing.T) {
	const preview = "agent-harness issueops execution prepare --id io-1 --mode orca --confirm --json"
	observer := &countingProvenanceObserver{}

	bound, err := bindExecutionNextCommand(issueopscontract.ExecutionPrepareResult{
		NextCommand: preview,
	}, observer)
	if err != nil {
		t.Fatalf("lease 없는 preview는 provenance 없이 통과해야 한다: %v", err)
	}
	result, ok := bound.(issueopscontract.ExecutionPrepareResult)
	if !ok {
		t.Fatalf("bound result type = %T", bound)
	}
	if result.NextCommand != preview {
		t.Fatalf("preview 명령은 그대로여야 한다: %q", result.NextCommand)
	}
	if observer.calls != 0 {
		t.Fatalf("결속할 lease가 없으면 바이너리를 관측하지도 않아야 한다: %d회 호출", observer.calls)
	}
}

// TestBindExecutionPrepareBindsOnceTheLeaseExists는 lease가 생긴 뒤에는 기존
// 계약이 그대로임을 고정한다.
func TestBindExecutionPrepareBindsOnceTheLeaseExists(t *testing.T) {
	bound, err := bindExecutionNextCommand(issueopscontract.ExecutionPrepareResult{
		Execution:   &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 3}},
		NextCommand: "agent-harness issueops execution claim --id io-1 --generation 3",
	}, provenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("b", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	result := bound.(issueopscontract.ExecutionPrepareResult)
	if !strings.Contains(result.NextCommand, "--generated-for-generation 3") {
		t.Fatalf("lease가 있으면 generation-bound여야 한다: %q", result.NextCommand)
	}
}
