package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"agent-harness/internal/adapter/policy"
	"agent-harness/internal/adapter/toolconformance"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const conformanceRawLimit = 64 << 10

type ConformanceProbeConfig struct {
	FixtureID          string
	ProbeTool          string
	Schema             map[string]any
	SchemaSHA          string
	ExpectedArguments  map[string]any
	ResultPath         string
	RunToken           string
	ProductionDispatch func()
}

type ConformanceCapture struct {
	FixtureID          string                         `json:"fixture_id"`
	CallCount          int                            `json:"call_count"`
	RawSHA256          string                         `json:"raw_sha256"`
	CanonicalArguments any                            `json:"canonical_arguments"`
	SchemaSHA256       string                         `json:"schema_sha256"`
	RunTokenSHA256     string                         `json:"run_token_sha256"`
	Classification     toolconformance.Classification `json:"classification"`
	AdvertisedValid    bool                           `json:"advertised_valid"`
	CanonicalValid     bool                           `json:"canonical_valid"`
	Diagnostics        []toolconformance.Diagnostic   `json:"diagnostics"`
}

type conformanceProbe struct {
	config ConformanceProbeConfig
	mu     sync.Mutex
	calls  int
}

// NewConformanceProbeServer constructs an isolated capture-only MCP server.
// It intentionally does not share the production catalog or dispatch path.
func NewConformanceProbeServer(config ConformanceProbeConfig) (*mcp.Server, error) {
	schema, err := cloneSchema(config.Schema)
	if err != nil {
		return nil, err
	}
	expected, err := cloneSchema(config.ExpectedArguments)
	if err != nil {
		return nil, err
	}
	config.Schema = schema
	config.ExpectedArguments = expected
	probe, err := newConformanceProbe(config)
	if err != nil {
		return nil, err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "agent_harness_probe", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: config.ProbeTool, InputSchema: config.Schema}, probe.handle)
	return server, nil
}

func newConformanceProbe(config ConformanceProbeConfig) (*conformanceProbe, error) {
	if config.ProbeTool == "" {
		return nil, fmt.Errorf("probe_tool_required")
	}
	if config.ResultPath == "" {
		return nil, fmt.Errorf("result_path_required")
	}
	if config.ExpectedArguments == nil {
		return nil, fmt.Errorf("expected_arguments_required")
	}
	actualSHA, err := toolconformance.CanonicalSchemaSHA256(config.Schema)
	if err != nil {
		return nil, err
	}
	if actualSHA != config.SchemaSHA {
		return nil, fmt.Errorf("schema_hash_mismatch")
	}
	return &conformanceProbe{config: config}, nil
}

func ServeConformanceProbe(ctx context.Context, input io.Reader, output io.Writer, config ConformanceProbeConfig) error {
	server, err := NewConformanceProbeServer(config)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(input), Writer: writeCloser{output}})
}

func (p *conformanceProbe) handle(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls > 0 {
		p.calls++
		if err := writeMultipleCallMarker(p.config.ResultPath, p.config.RunToken, p.calls); err != nil {
			_ = os.Remove(p.config.ResultPath)
			return probeFailure("multiple_call_marker_failed"), nil
		}
		return probeFailure("multiple_calls"), nil
	}
	if fileExists(p.config.ResultPath) {
		return probeFailure(captureCollisionCode(p.config.ResultPath, p.config.RunToken)), nil
	}
	p.calls++
	capture, err := captureArguments(p.config, []byte(req.Params.Arguments))
	if err != nil {
		return probeFailure(err.Error()), nil
	}
	capture.CallCount = p.calls
	if err := writeCapture(p.config.ResultPath, capture); err != nil {
		return probeFailure(err.Error()), nil
	}
	body, _ := json.Marshal(map[string]any{"ok": true, "captured": true, "fixture_id": p.config.FixtureID})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}, nil
}

func cloneSchema(schema map[string]any) (map[string]any, error) {
	if schema == nil {
		return nil, nil
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var copy map[string]any
	if err := json.Unmarshal(b, &copy); err != nil {
		return nil, err
	}
	return copy, nil
}

func captureCollisionCode(path, runToken string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "result_collision"
	}
	var capture ConformanceCapture
	if err := json.Unmarshal(b, &capture); err != nil || capture.RunTokenSHA256 == "" {
		return "result_collision"
	}
	if capture.RunTokenSHA256 != sha256Hex([]byte(runToken)) {
		return "stale_run_token"
	}
	return "result_collision"
}

func probeFailure(code string) *mcp.CallToolResult {
	body, _ := json.Marshal(map[string]any{"ok": false, "error": code})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(body)}}}
}

func captureArguments(config ConformanceProbeConfig, raw []byte) (ConformanceCapture, error) {
	if len(raw) > conformanceRawLimit {
		return ConformanceCapture{}, fmt.Errorf("arguments_too_large")
	}
	classified, err := toolconformance.Classify(
		toolconformance.CallObservation{RawArguments: raw, CallCount: 1},
		config.Schema,
		config.ExpectedArguments,
	)
	if err != nil {
		return ConformanceCapture{}, err
	}
	var arguments any
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return ConformanceCapture{}, fmt.Errorf("invalid_json")
	}
	canonical, err := json.Marshal(arguments)
	if err != nil {
		return ConformanceCapture{}, err
	}
	rawSum := sha256.Sum256(raw)
	var canonicalValue any
	if err := json.Unmarshal(canonical, &canonicalValue); err != nil {
		return ConformanceCapture{}, err
	}
	replacements := map[string]string{}
	redactedArguments := redactArguments(canonicalValue, config.Schema, "", replacements)
	diagnostics := redactDiagnosticPaths(classified.Diagnostics, replacements)
	return ConformanceCapture{
		FixtureID: config.FixtureID, RawSHA256: hex.EncodeToString(rawSum[:]),
		CanonicalArguments: redactedArguments, SchemaSHA256: config.SchemaSHA,
		RunTokenSHA256: sha256Hex([]byte(config.RunToken)), Classification: classified.Classification,
		AdvertisedValid: classified.AdvertisedValid, CanonicalValid: classified.CanonicalValid,
		Diagnostics: diagnostics,
	}, nil
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func redactArguments(value any, schema map[string]any, path string, replacements map[string]string) any {
	switch v := value.(type) {
	case string:
		return policy.RedactFreeform(v)
	case []any:
		out := make([]any, len(v))
		child, _ := schema["items"].(map[string]any)
		for i := range v {
			out[i] = redactArguments(v[i], child, pointerPath(path, fmt.Sprintf("%d", i)), replacements)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		properties, _ := schema["properties"].(map[string]any)
		for key, item := range v {
			safeKey := key
			child, declared := properties[key].(map[string]any)
			if !declared {
				safeKey = "unknown_key_" + sha256Hex([]byte(key))[:12]
				replacements[pointerPath(path, key)] = pointerPath(path, safeKey)
			}
			if sensitiveArgumentKey(key) {
				out[safeKey] = "<redacted>"
				continue
			}
			out[safeKey] = redactArguments(item, child, pointerPath(path, safeKey), replacements)
		}
		return out
	default:
		return value
	}
}

func redactDiagnosticPaths(diagnostics []toolconformance.Diagnostic, replacements map[string]string) []toolconformance.Diagnostic {
	out := append([]toolconformance.Diagnostic(nil), diagnostics...)
	for index := range out {
		if replacement, ok := replacements[out[index].Path]; ok {
			out[index].Path = replacement
		}
	}
	return out
}

func pointerPath(parent, key string) string {
	key = strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
	return parent + "/" + key
}

func sensitiveArgumentKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, word := range []string{"token", "password", "passwd", "secret", "api_key", "apikey", "credential", "authorization"} {
		if strings.Contains(key, word) {
			return true
		}
	}
	return false
}

func writeCapture(path string, capture ConformanceCapture) error {
	if path == "" {
		return fmt.Errorf("result_path_required")
	}
	if fileExists(path) {
		return fmt.Errorf("result_collision")
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(parent, ".capture-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func writeMultipleCallMarker(path, runToken string, callCount int) error {
	marker := struct {
		CallCount      int    `json:"call_count"`
		RunTokenSHA256 string `json:"run_token_sha256"`
	}{CallCount: callCount, RunTokenSHA256: sha256Hex([]byte(runToken))}
	return writePrivateJSON(path+".multiple", marker)
}

func writePrivateJSON(path string, value any) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(parent, ".capture-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }

type writeCloser struct{ io.Writer }

func (writeCloser) Close() error { return nil }
