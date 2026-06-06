package mcpcli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPArgHelpersCoverSupportedScalarForms(t *testing.T) {
	args := map[string]any{
		"string":       "value",
		"slice_string": []string{"a", "b"},
		"slice_any":    []any{"a", 7, "b"},
		"csv":          "a, b,,c",
		"bool":         "true",
		"bool_bad":     "not-bool",
		"int_float":    float64(12),
		"int_string":   "13",
		"int_bad":      "nope",
		"int64_float":  float64(14),
		"int64_value":  int64(15),
		"int64_int":    16,
		"int64_string": "17",
		"float_value":  1.5,
		"float_int":    2,
		"float_int64":  int64(3),
		"float_string": "4.5",
	}

	if stringArg(args, "string") != "value" || stringArg(nil, "string") != "" {
		t.Fatalf("stringArg did not preserve expected string behavior")
	}
	if !argSet(args, "string") || argSet(nil, "string") || argSet(args, "missing") {
		t.Fatalf("argSet did not distinguish present/missing args")
	}
	if stringArgWithDefault(args, "missing", "fallback") != "fallback" {
		t.Fatalf("stringArgWithDefault did not use fallback")
	}
	assertStringSlice(t, stringSliceArg(args, "slice_string"), []string{"a", "b"})
	copied := stringSliceArg(args, "slice_string")
	copied[0] = "changed"
	assertStringSlice(t, stringSliceArg(args, "slice_string"), []string{"a", "b"})
	assertStringSlice(t, stringSliceArg(args, "slice_any"), []string{"a", "b"})
	assertStringSlice(t, stringSliceArg(args, "csv"), []string{"a", "b", "c"})
	if stringSliceArg(nil, "csv") != nil || stringSliceArg(args, "missing") != nil {
		t.Fatalf("stringSliceArg should return nil for absent inputs")
	}
	if !boolArg(args, "bool") || boolArg(args, "bool_bad") || boolArg(nil, "bool") {
		t.Fatalf("boolArg did not parse bool strings safely")
	}
	if intArg(args, "int_float", 0) != 12 || intArg(args, "int_string", 0) != 13 || intArg(args, "int_bad", 99) != 99 || intArg(nil, "x", 88) != 88 {
		t.Fatalf("intArg did not parse supported forms")
	}
	if int64Arg(args, "int64_float", 0) != 14 || int64Arg(args, "int64_value", 0) != 15 || int64Arg(args, "int64_int", 0) != 16 || int64Arg(args, "int64_string", 0) != 17 || int64Arg(nil, "x", 18) != 18 {
		t.Fatalf("int64Arg did not parse supported forms")
	}
	if floatArg(args, "float_value", 0) != 1.5 || floatArg(args, "float_int", 0) != 2 || floatArg(args, "float_int64", 0) != 3 || floatArg(args, "float_string", 0) != 4.5 || floatArg(nil, "x", 9.5) != 9.5 {
		t.Fatalf("floatArg did not parse supported forms")
	}
}

func TestMCPTransportCoversParseNotificationAndMethodErrors(t *testing.T) {
	var out bytes.Buffer
	var diagnostics bytes.Buffer
	input := strings.NewReader(strings.Join([]string{
		"{not json}",
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":1,"method":"missing/method","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`,
		"",
	}, "\n"))

	if err := ServeMCPStream(input, &out, &diagnostics); err != nil {
		t.Fatalf("ServeMCPStream: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"Parse error"`) || !strings.Contains(output, `"Method not found"`) || !strings.Contains(output, `"serverInfo"`) {
		t.Fatalf("unexpected MCP output:\n%s", output)
	}
	if !strings.Contains(diagnostics.String(), "notifications/initialized") {
		t.Fatalf("notification was not written to diagnostics: %s", diagnostics.String())
	}

	out.Reset()
	writeRPCErrorTo(&out, nil, -32000, "boom", "data")
	if !strings.Contains(out.String(), `"id":null`) || !strings.Contains(out.String(), `"boom"`) {
		t.Fatalf("writeRPCErrorTo did not preserve null id error response: %s", out.String())
	}
}

func TestMCPResourceReadCoversGuidanceStateAndErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, tc := range []struct {
		uri       string
		wantType  string
		wantText  string
		wantError bool
	}{
		{uri: "harness://project-doc-upkeep", wantType: "text/markdown", wantText: "Project Doc Upkeep Guidance"},
		{uri: "harness://api-doc-guidance", wantType: "text/markdown", wantText: "API Documentation Guidance"},
		{uri: "harness://command-policy", wantType: "application/json", wantText: "workspace"},
		{uri: "harness://state", wantType: "application/json", wantText: "keys"},
		{uri: "harness://commit-policy", wantType: "text/markdown", wantText: "Conventional"},
		{uri: "harness://unknown", wantError: true},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			result, rpcErr := HandleResourceRead(mustMarshalMCPTest(t, map[string]any{"uri": tc.uri}))
			if tc.wantError {
				if rpcErr == nil || !strings.Contains(rpcErr.Message, "Unknown resource") {
					t.Fatalf("expected unknown resource error, got result=%#v err=%+v", result, rpcErr)
				}
				return
			}
			if rpcErr != nil {
				t.Fatalf("HandleResourceRead(%s): %+v", tc.uri, rpcErr)
			}
			content := singleMCPResourceContent(t, result)
			if content["mimeType"] != tc.wantType || !strings.Contains(content["text"].(string), tc.wantText) {
				t.Fatalf("unexpected resource content for %s: %#v", tc.uri, content)
			}
		})
	}

	_, rpcErr := HandleResourceRead(json.RawMessage(`{bad json}`))
	if rpcErr == nil || rpcErr.Code != -32602 {
		t.Fatalf("expected invalid params error, got %+v", rpcErr)
	}
}

func TestMCPProjectToolCallCoversDirectPayloadAndUnknownTool(t *testing.T) {
	direct, rpcErr := HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "commit_policy", "arguments": map[string]any{}}))
	if rpcErr != nil {
		t.Fatalf("commit_policy: %+v", rpcErr)
	}
	if text := extractSingleTextResult(t, direct); !strings.Contains(text, "Conventional") {
		t.Fatalf("commit_policy did not return markdown policy text: %s", text)
	}

	payload, rpcErr := HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "project_docs_bootstrap_plan", "arguments": map[string]any{"repo": t.TempDir()}}))
	if rpcErr != nil {
		t.Fatalf("project_docs_bootstrap_plan: %+v", rpcErr)
	}
	if text := extractSingleTextResult(t, payload); !strings.Contains(text, `"dry_run"`) && !strings.Contains(text, `"write"`) {
		t.Fatalf("bootstrap plan payload did not look like JSON result: %s", text)
	}

	_, rpcErr = HandleToolCall(mustMarshalMCPTest(t, map[string]any{"name": "missing_tool", "arguments": map[string]any{}}))
	if rpcErr == nil || rpcErr.Code != -32602 || !strings.Contains(rpcErr.Message, "Unknown tool") {
		t.Fatalf("expected unknown tool error, got %+v", rpcErr)
	}
	_, rpcErr = HandleToolCall(json.RawMessage(`{bad json}`))
	if rpcErr == nil || rpcErr.Code != -32602 {
		t.Fatalf("expected invalid params error, got %+v", rpcErr)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length got %d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] got %q want %q in %#v", i, got[i], want[i], got)
		}
	}
}

func mustMarshalMCPTest(t *testing.T, value any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func singleMCPResourceContent(t *testing.T, result any) map[string]any {
	t.Helper()
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected resource result type %T", result)
	}
	contents, ok := outer["contents"].([]map[string]any)
	if !ok || len(contents) != 1 {
		t.Fatalf("unexpected resource contents: %#v", outer["contents"])
	}
	return contents[0]
}
