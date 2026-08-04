package executioncmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type provenanceObserverStub struct {
	evidence provenanceport.Receipt
	err      error
}

func (s provenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, s.err
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
