package contractcli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
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
			stderr, err := captureProjectCLIStderr(func() error {
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
			_, err := captureProjectCLIStderr(func() error {
				return Run(tt.args)
			})
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
				t.Fatalf("expected invalid flag error, got %v", err)
			}
		})
	}
}

func captureProjectCLIStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	os.Stderr = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		return "", closeErr
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return "", err
	}
	return out.String(), callErr
}

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	os.Stdout = w
	if err := fn(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
