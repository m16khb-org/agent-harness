package externalllm

import (
	"testing"
)

func TestNewAgyProviderDefaults(t *testing.T) {
	p := NewAgyProvider()
	if p.Command != "agy" {
		t.Errorf("expected default command agy, got %q", p.Command)
	}
	if p.Timeout <= 0 {
		t.Errorf("expected positive timeout, got %v", p.Timeout)
	}
}

func TestAgyProviderCommandFallback(t *testing.T) {
	p := &AgyProvider{}
	if p.command() != "agy" {
		t.Errorf("expected fallback agy, got %q", p.command())
	}
	p.Command = "  "
	if p.command() != "agy" {
		t.Errorf("expected fallback agy for whitespace command, got %q", p.command())
	}
}

func TestAgyProviderTimeoutFallback(t *testing.T) {
	p := &AgyProvider{}
	if p.timeout() <= 0 {
		t.Errorf("expected positive default timeout, got %v", p.timeout())
	}
	p.Timeout = 0
	if p.timeout() <= 0 {
		t.Errorf("expected positive fallback timeout, got %v", p.timeout())
	}
}

func TestFormatSchemaExampleEmpty(t *testing.T) {
	if s := formatSchemaExample(nil); s != "{}" {
		t.Errorf("expected {} for nil schema, got %q", s)
	}
	if s := formatSchemaExample(map[string]any{}); s != "{}" {
		t.Errorf("expected {} for empty schema, got %q", s)
	}
}

func TestFormatSchemaExampleWithProps(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{
			"ok":      map[string]any{"type": "boolean"},
			"score":   map[string]any{"type": "number"},
			"summary": map[string]any{"type": "string"},
			"tags":    map[string]any{"type": "array"},
			"meta":    map[string]any{"type": "object"},
		},
	}
	result := formatSchemaExample(schema)
	if result == "" || result == "{}" {
		t.Errorf("expected non-trivial example, got %q", result)
	}
}

func TestAgyProviderImplementsExternalLLM(t *testing.T) {
	// Compile-time check via var _ ensures AgyProvider satisfies the interface.
	// This test just confirms the package compiles.
	var p interface{} = NewAgyProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}
