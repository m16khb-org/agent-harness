package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	mcpcontract "issueops/internal/contract/mcp"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/toolconformance"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConformanceProbeWritesPrivateAtomicCapture(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "probe")
	path := filepath.Join(dir, "result.json")
	capture := ConformanceCapture{FixtureID: "empty_object", CallCount: 1, RawSHA256: "raw", CanonicalArguments: "<redacted>", SchemaSHA256: "schema", RunTokenSHA256: "token", Classification: core.Classification(core.ExactValid), AdvertisedValid: true, CanonicalValid: true, Diagnostics: []core.Diagnostic{}}
	if err := writeCapture(path, capture); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("dir=%v err=%v", info, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file=%v err=%v", info, err)
	}
	var got ConformanceCapture
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, capture) {
		t.Fatalf("got=%#v", got)
	}
	if err := writeCapture(path, capture); err == nil {
		t.Fatal("result collision accepted")
	}
}

func TestConformanceProbeRejectsMissingResultPath(t *testing.T) {
	if err := writeCapture("", ConformanceCapture{}); err == nil {
		t.Fatal("missing path accepted")
	}
}

func TestConformanceProbeFailsClosedWithoutProductionDispatch(t *testing.T) {
	called := 0
	config := testProbeConfig(t, filepath.Join(t.TempDir(), "capture.json"))
	if _, err := captureArguments(config, []byte(`{not json`)); err == nil {
		t.Fatal("malformed json accepted")
	}
	if _, err := captureArguments(config, make([]byte, conformanceRawLimit+1)); err == nil {
		t.Fatal("oversized arguments accepted")
	}
	if _, err := captureArguments(config, []byte(`{"token":"secret"}`)); err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Fatalf("production dispatch calls=%d", called)
	}
}

func TestConformanceProbeSDKRoundTripCapturesOnlyRenamedProbe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence", "capture.json")
	dispatchCalls := 0
	arguments := map[string]any{
		"network_allowed": false,
		"api_key":         "not-safe-to-store",
		"nested":          map[string]any{"authorization": "Bearer secret"},
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"network_allowed": map[string]any{"type": "boolean"},
			"api_key":         map[string]any{"type": "string"},
			"nested": map[string]any{
				"type":       "object",
				"properties": map[string]any{"authorization": map[string]any{"type": "string"}},
			},
		},
	}
	config := newTestProbeConfig(t, "mixed_scalars", "harness_probe_mixed_scalars", schema, arguments, path, "run-token")
	config.ProductionDispatch = func() { dispatchCalls++ }
	session := serveConformanceProbeStdio(t, config)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != config.ProbeTool {
		t.Fatalf("tools=%+v", tools.Tools)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: config.ProbeTool, Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if dispatchCalls != 0 {
		t.Fatalf("production dispatch calls=%d", dispatchCalls)
	}
	var capture ConformanceCapture
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	rawSum := sha256.Sum256(raw)
	tokenSum := sha256.Sum256([]byte(config.RunToken))
	if capture.CallCount != 1 || capture.RawSHA256 != fmtHash(rawSum) || capture.SchemaSHA256 != config.SchemaSHA || capture.RunTokenSHA256 != fmtHash(tokenSum) {
		t.Fatalf("capture hashes=%+v", capture)
	}
	want := map[string]any{
		"network_allowed": false,
		"api_key":         "<redacted>",
		"nested":          map[string]any{"authorization": "<redacted>"},
	}
	if !sameJSON(capture.CanonicalArguments, want) {
		t.Fatalf("canonical=%#v want=%#v", capture.CanonicalArguments, want)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("dir=%v err=%v", info, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file=%v err=%v", info, err)
	}
}

func TestConformanceProbeSDKSecondCallReportsMultipleCallsWithoutOverwritingCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	config := testProbeConfig(t, path)
	server, err := NewConformanceProbeServer(config)
	if err != nil {
		t.Fatal(err)
	}
	session := connectConformanceProbe(t, server)
	if result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: config.ProbeTool, Arguments: map[string]any{"first": true}}); err != nil || result.IsError {
		t.Fatalf("first result=%+v err=%v", result, err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: config.ProbeTool, Arguments: map[string]any{"second": true}})
	if err != nil || !result.IsError || !strings.Contains(probeText(result), "multiple_calls") {
		t.Fatalf("second result=%+v err=%v", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(first) {
		t.Fatalf("first capture overwritten\nfirst=%s\nafter=%s", first, after)
	}
	var marker struct {
		CallCount int `json:"call_count"`
	}
	markerData, err := os.ReadFile(path + ".multiple")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(markerData, &marker); err != nil || marker.CallCount != 2 {
		t.Fatalf("multiple-call marker=%+v err=%v", marker, err)
	}
}

func TestConformanceProbeHandlerFailsClosedForMalformedAndOversizedRawArguments(t *testing.T) {
	for name, raw := range map[string][]byte{
		"malformed": []byte(`{not json`),
		"oversized": make([]byte, conformanceRawLimit+1),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.json")
			config := testProbeConfig(t, path)
			probe, err := newConformanceProbe(config)
			if err != nil {
				t.Fatal(err)
			}
			result, err := probe.handle(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: raw}})
			if err != nil || !result.IsError {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("capture unexpectedly exists: %v", err)
			}
		})
	}
}

func TestConformanceProbeSDKRejectsWrongToolAndMalformedResultCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	if err := os.WriteFile(path, []byte("first-result"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := testProbeConfig(t, path)
	server, err := NewConformanceProbeServer(config)
	if err != nil {
		t.Fatal(err)
	}
	session := connectConformanceProbe(t, server)
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "not_the_probe", Arguments: map[string]any{}}); err == nil {
		t.Fatal("wrong tool accepted")
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: config.ProbeTool, Arguments: map[string]any{}})
	if err != nil || !result.IsError || !strings.Contains(probeText(result), "result_collision") {
		t.Fatalf("collision result=%+v err=%v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first-result" {
		t.Fatalf("collision overwrote=%q err=%v", data, err)
	}
}

func TestConformanceProbeRejectsStaleRunTokenWithoutOverwritingCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	stale := ConformanceCapture{FixtureID: "empty_object", CallCount: 1, RawSHA256: "raw", CanonicalArguments: map[string]any{}, SchemaSHA256: "schema", RunTokenSHA256: fmtHash(sha256.Sum256([]byte("old-run-token"))), Classification: core.Classification(core.ExactValid), AdvertisedValid: true, CanonicalValid: true, Diagnostics: []core.Diagnostic{}}
	if err := writeCapture(path, stale); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := testProbeConfig(t, path)
	config.RunToken = "new-run-token"
	server, err := NewConformanceProbeServer(config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := connectConformanceProbe(t, server).CallTool(context.Background(), &mcp.CallToolParams{Name: config.ProbeTool, Arguments: map[string]any{}})
	if err != nil || !result.IsError || !strings.Contains(probeText(result), "stale_run_token") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("stale result overwritten=%q err=%v", after, err)
	}
}

func TestConformanceProbeCopiesSourceSchemaBeforeAdvertising(t *testing.T) {
	source := map[string]any{"type": "object", "properties": map[string]any{"first": map[string]any{"type": "string"}}}
	config := newTestProbeConfig(t, "empty_object", "harness_probe_empty_object", source, map[string]any{}, filepath.Join(t.TempDir(), "capture.json"), "token")
	server, err := NewConformanceProbeServer(config)
	if err != nil {
		t.Fatal(err)
	}
	source["properties"].(map[string]any)["later"] = map[string]any{"type": "boolean"}
	tools, err := connectConformanceProbe(t, server).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	inputSchema, ok := tools.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("schema type=%T", tools.Tools[0].InputSchema)
	}
	properties := inputSchema["properties"].(map[string]any)
	if _, ok := properties["later"]; ok {
		t.Fatalf("advertised schema aliases source: %#v", properties)
	}
}

func testProbeConfig(t *testing.T, path string) mcpcontract.ConformanceProbeConfig {
	t.Helper()
	return newTestProbeConfig(t, "empty_object", "harness_probe_empty_object", map[string]any{"type": "object"}, map[string]any{}, path, "token")
}

func newTestProbeConfig(t *testing.T, fixtureID, probeTool string, schema, expected map[string]any, path, token string) mcpcontract.ConformanceProbeConfig {
	t.Helper()
	schemaSHA, err := core.CanonicalSchemaSHA256(schema)
	if err != nil {
		t.Fatal(err)
	}
	return mcpcontract.ConformanceProbeConfig{
		FixtureID: fixtureID, ProbeTool: probeTool, Schema: schema, SchemaSHA: schemaSHA,
		ExpectedArguments: expected, ResultPath: path, RunToken: token,
	}
}
func connectConformanceProbe(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "conformance_test_client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func serveConformanceProbeStdio(t *testing.T, config mcpcontract.ConformanceProbeConfig) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- ServeConformanceProbe(ctx, inputReader, outputWriter, config) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "conformance_stdio_client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: outputReader, Writer: inputWriter}, nil)
	if err != nil {
		cancel()
		_ = inputWriter.Close()
		_ = outputReader.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		_ = inputWriter.Close()
		_ = outputReader.Close()
		<-done
	})
	return session
}

func probeText(result *mcp.CallToolResult) string {
	if len(result.Content) != 1 {
		return ""
	}
	if text, ok := result.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

func fmtHash(sum [sha256.Size]byte) string { return fmt.Sprintf("%x", sum) }

func sameJSON(got, want any) bool {
	gotJSON, gotErr := json.Marshal(got)
	wantJSON, wantErr := json.Marshal(want)
	return gotErr == nil && wantErr == nil && string(gotJSON) == string(wantJSON)
}
