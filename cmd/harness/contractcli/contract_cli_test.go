package contractcli

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/testsupport"
)

func TestRunContractRejectsMissingAndUnknownSubcommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing", args: nil, wantErr: "missing contract subcommand"},
		{name: "unknown", args: []string{"missing-command"}, wantErr: `unknown contract subcommand "missing-command"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr, err := captureProjectCLIStderr(t, func() error {
				return Run(tt.args)
			})

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
			if !strings.Contains(stderr, "agent-harness contract schema") || !strings.Contains(stderr, "agent-harness contract check") {
				t.Fatalf("expected contract usage on stderr, got:\n%s", stderr)
			}
		})
	}
}

func TestRunContractSchemaPrintsTextAndJSON(t *testing.T) {
	text := captureStdoutForContract(t, func() error {
		return Run([]string{"schema"})
	})
	if !strings.Contains(text, "agent_harness_cli_mcp_compatibility v1 ") {
		t.Fatalf("expected schema text summary, got:\n%s", text)
	}

	jsonOut := captureStdoutForContract(t, func() error {
		return Run([]string{"schema", "--json"})
	})
	var contract CompatibilityContract
	if err := json.Unmarshal([]byte(jsonOut), &contract); err != nil {
		t.Fatalf("decode contract schema JSON: %v\n%s", err, jsonOut)
	}
	if !contract.OK || contract.Name != "agent_harness_cli_mcp_compatibility" || contract.Version != 1 || contract.Hash == "" {
		t.Fatalf("unexpected contract schema: %#v", contract)
	}
}

func TestRunContractCheckPrintsTextAndJSON(t *testing.T) {
	text := captureStdoutForContract(t, func() error {
		return Run([]string{"check"})
	})
	if !strings.Contains(text, "contract ok: ") {
		t.Fatalf("expected contract check text summary, got:\n%s", text)
	}

	jsonOut := captureStdoutForContract(t, func() error {
		return Run([]string{"check", "--json"})
	})
	var contract CompatibilityContract
	if err := json.Unmarshal([]byte(jsonOut), &contract); err != nil {
		t.Fatalf("decode contract check JSON: %v\n%s", err, jsonOut)
	}
	if !contract.OK || contract.Hash == "" || !containsString(contract.MCPTools, "contract_schema") {
		t.Fatalf("unexpected contract check: %#v", contract)
	}
}

func TestRunContractSchemaAndCheckRejectInvalidFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "schema", args: []string{"schema", "--missing-flag"}},
		{name: "check", args: []string{"check", "--missing-flag"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := captureProjectCLIStderr(t, func() error {
				return Run(tt.args)
			})
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("expected invalid flag error, got %v", err)
			}
		})
	}
}

func captureProjectCLIStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStderrAndError(t, fn)
}

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}
