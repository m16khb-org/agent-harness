package mcpcli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestServeMCPStreamListsHarnessTools(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	var diag bytes.Buffer
	if err := ServeMCPStream(strings.NewReader(input), &out, &diag); err != nil {
		t.Fatal(err)
	}
	lines := splitLines(out.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid json %q: %v", line, err)
		}
		if _, ok := obj["result"]; !ok {
			t.Fatalf("missing result: %s", line)
		}
	}
	if !strings.Contains(out.String(), "atomic_commit_preflight") || !strings.Contains(out.String(), "Lore") {
		t.Fatalf("missing harness tools/context in output:\n%s", out.String())
	}
}
