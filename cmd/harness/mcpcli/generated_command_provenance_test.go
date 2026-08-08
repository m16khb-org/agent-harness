package mcpcli

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

type mcpProvenanceObserverStub struct {
	evidence provenanceport.Receipt
	err      error
}

func (s mcpProvenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, s.err
}

func TestBindMCPIssueOpsExecutionNextCommandMatchesCLIContract(t *testing.T) {
	raw := issueops.ExecutionReplaceResult{
		Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 5}},
		NextCommand: "agent-harness issueops execution resume --id io-1 --expected-generation 5 --confirm",
	}
	bound, err := bindMCPIssueOpsExecutionNextCommand(raw, mcpProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := bound.(issueops.ExecutionReplaceResult)
	if !ok || !strings.Contains(result.NextCommand, "--generated-for-generation 5") {
		t.Fatalf("MCP bound result = %#v", bound)
	}
}

func TestMCPIssueOpsExecutionHandlerBindsBaseSyncRequiredErrorNextCommand(t *testing.T) {
	observer := mcpProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64),
	}}
	outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(7),
		"host": "codex", "session_id": "session-1", "session_pid": float64(42),
		"session_started_at": "2026-08-04T00:00:00Z", "session_executable": "/bin/codex",
		"cwd": "/repo.worktrees/318", "confirm": true,
	}, MCPDependencies{
		Resume: func(context.Context, string, issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			return issueops.ExecutionResumeResult{}, issueopscontract.NewBaseSyncRequiredError("io-aaaaaaaaaaaa", 7)
		},
		Provenance: observer,
	})
	payload, ok := outcome.Payload.(map[string]any)
	next, _ := payload["next_command"].(string)
	if !ok || !outcome.IsError ||
		!strings.HasPrefix(next, "'/repo/bin/agent-harness'") ||
		!strings.Contains(next, "--generated-for-generation 7") ||
		!strings.Contains(next, "--generated-by-sha256 "+strings.Repeat("a", 64)) {
		t.Fatalf("MCP typed error outcome=%#v", outcome)
	}
}

func TestMCPIssueOpsExecutionTypedErrorObservationFailureHasNoUnboundFallback(t *testing.T) {
	outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(7),
		"host": "codex", "session_id": "session-1", "session_pid": float64(42),
		"session_started_at": "2026-08-04T00:00:00Z", "session_executable": "/bin/codex",
		"cwd": "/repo.worktrees/318", "confirm": true,
	}, MCPDependencies{
		Resume: func(context.Context, string, issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			return issueops.ExecutionResumeResult{}, issueopscontract.NewBaseSyncRequiredError("io-aaaaaaaaaaaa", 7)
		},
		Provenance: mcpProvenanceObserverStub{err: errors.New("observation failed")},
	})
	payload, ok := outcome.Payload.(map[string]any)
	if !ok || !outcome.IsError || payload["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("MCP typed error observation failure=%#v", outcome)
	}
	if _, exists := payload["next_command"]; exists {
		t.Fatalf("MCP typed error observation failure leaked command: %#v", payload)
	}
}

func TestMCPIssueOpsExecutionHandlerBindsGeneratedNextCommand(t *testing.T) {
	observer := mcpProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(5),
		"host": "codex", "session_id": "session-1", "session_pid": float64(42),
		"session_started_at": "2026-08-04T00:00:00Z", "session_executable": "/bin/codex",
		"cwd": "/repo.worktrees/303", "confirm": true,
	}, MCPDependencies{
		Resume: func(context.Context, string, issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			return issueops.ExecutionResumeResult{
				OK: true, ID: "io-aaaaaaaaaaaa",
				Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 5}},
				NextCommand: "agent-harness issueops execution claim --id io-aaaaaaaaaaaa --generation 5 --claim-token-file /tmp/token",
			}, nil
		},
		Provenance: observer,
	})
	result, ok := outcome.Payload.(issueops.ExecutionResumeResult)
	if !ok || outcome.IsError || !strings.Contains(result.NextCommand, "--generated-for-generation 5") {
		t.Fatalf("MCP execution outcome = %#v", outcome)
	}
}

func TestMCPIssueOpsExecutionHandlerObservationFailureHasNoUnboundPayload(t *testing.T) {
	outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(5),
		"host": "codex", "session_id": "session-1", "session_pid": float64(42),
		"session_started_at": "2026-08-04T00:00:00Z", "session_executable": "/bin/codex",
		"cwd": "/repo.worktrees/303", "confirm": true,
	}, MCPDependencies{
		Resume: func(context.Context, string, issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			return issueops.ExecutionResumeResult{
				OK: true, ID: "io-aaaaaaaaaaaa",
				Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 5}},
				NextCommand: "agent-harness issueops execution claim --id io-aaaaaaaaaaaa --generation 5 --claim-token-file /tmp/token",
			}, nil
		},
		Provenance: mcpProvenanceObserverStub{err: errors.New("executable unavailable")},
	})
	if !outcome.IsError {
		t.Fatalf("MCP observation failure outcome = %#v", outcome)
	}
	if payload, ok := outcome.Payload.(issueops.ExecutionResumeResult); ok && payload.NextCommand != "" {
		t.Fatalf("MCP observation failure leaked unbound command: %#v", payload)
	}
}

func TestMCPIssueOpsExecutionHandlerMissingObserverHasNoUnboundPayload(t *testing.T) {
	outcome := handleMCPIssueOpsExecutionWithDependencies(map[string]any{
		"action": "resume", "id": "io-aaaaaaaaaaaa", "expected_generation": float64(5),
		"host": "codex", "session_id": "session-1", "session_pid": float64(42),
		"session_started_at": "2026-08-04T00:00:00Z", "session_executable": "/bin/codex",
		"cwd": "/repo.worktrees/303", "confirm": true,
	}, MCPDependencies{
		Resume: func(context.Context, string, issueops.ExecutionResumeRequest) (issueops.ExecutionResumeResult, error) {
			return issueops.ExecutionResumeResult{
				OK: true, ID: "io-aaaaaaaaaaaa",
				Execution:   issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 5}},
				NextCommand: "agent-harness issueops execution claim --id io-aaaaaaaaaaaa --generation 5 --claim-token-file /tmp/token",
			}, nil
		},
	})
	if !outcome.IsError {
		t.Fatalf("MCP missing observer outcome = %#v", outcome)
	}
	if payload, ok := outcome.Payload.(issueops.ExecutionResumeResult); ok && payload.NextCommand != "" {
		t.Fatalf("MCP missing observer leaked unbound command: %#v", payload)
	}
	structured, ok := outcome.Payload.(map[string]any)
	if !ok || structured["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("MCP missing observer failure is not structured: %#v", outcome.Payload)
	}
}

func TestBindMCPIssueOpsExecutionNextCommandCoversEveryCommandBearingResult(t *testing.T) {
	observer := mcpProvenanceObserverStub{evidence: provenanceport.Receipt{
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
			bound, err := bindMCPIssueOpsExecutionNextCommand(test.value, observer)
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
