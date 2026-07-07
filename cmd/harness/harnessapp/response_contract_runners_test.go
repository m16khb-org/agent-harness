package harnessapp

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"agent-harness/cmd/harness/harnessapp/responsecontract"
	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
)

func runCLIJSONContract(t *testing.T, replacements map[string]string, fn func() error) any {
	t.Helper()
	stdout := captureStdoutForContract(t, fn)
	var value any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatalf("unmarshal CLI JSON %q: %v", stdout, err)
	}
	return responsecontract.NormalizeContractValue(value, replacements)
}

func runMCPToolContract(t *testing.T, replacements map[string]string, name string, arguments map[string]any) any {
	t.Helper()
	value := runMCPToolContractRaw(t, name, arguments)
	return responsecontract.NormalizeMCPTextJSON(responsecontract.NormalizeContractValue(value, replacements), replacements)
}

func runMCPToolContractRaw(t *testing.T, name string, arguments map[string]any) map[string]any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := handleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("handleToolCall(%s): %+v", name, rpcErr)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("unexpected MCP result type %T", value)
	}
	return typed
}

func captureStdoutForContract(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func mustStateReadForContract(t *testing.T, key string) core.StateResult {
	t.Helper()
	result, err := core.StateRead(key)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustWriteStateRecordForContract(t *testing.T, stateDir, key string, record core.StateRecord) {
	t.Helper()
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("state", key, append(b, '\n')); err != nil {
		t.Fatal(err)
	}
}
