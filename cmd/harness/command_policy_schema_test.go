package main

import (
	"testing"

	mcpadapter "agent-harness/internal/adapter/mcp"
)

func TestCommandPolicyInputSchemaStableBoundary(t *testing.T) {
	schema := mcpadapter.CommandPolicyInputSchema()
	if schema["type"] != "object" {
		t.Fatalf("schema type drifted: %#v", schema["type"])
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required should be []string, got %T", schema["required"])
	}
	for _, name := range []string{"workspace_root", "cwd", "argv"} {
		if !schemaRequiredContains(required, name) {
			t.Fatalf("required fields missing %q: %#v", name, required)
		}
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties should be map[string]any, got %T", schema["properties"])
	}
	for _, name := range []string{
		"workspace_root",
		"cwd",
		"argv",
		"timeout",
		"env_allowlist",
		"network_allowed",
		"write_allowed",
		"shell_allowed",
		"shell_reason",
		"audit_log_id",
	} {
		if _, exists := properties[name]; !exists {
			t.Fatalf("properties missing %q: %#v", name, properties)
		}
	}

	assertPropertyType(t, properties, "workspace_root", "string")
	assertPropertyType(t, properties, "cwd", "string")
	assertPropertyType(t, properties, "argv", "array")
	assertPropertyType(t, properties, "timeout", "string")
	assertPropertyType(t, properties, "env_allowlist", "array")
	assertPropertyType(t, properties, "network_allowed", "boolean")
	assertPropertyType(t, properties, "write_allowed", "boolean")
	assertPropertyType(t, properties, "shell_allowed", "boolean")
	assertPropertyType(t, properties, "shell_reason", "string")
	assertPropertyType(t, properties, "audit_log_id", "string")
}

func assertPropertyType(t *testing.T, properties map[string]any, name string, want string) {
	t.Helper()
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("property %q should be map[string]any, got %T", name, properties[name])
	}
	if property["type"] != want {
		t.Fatalf("property %q type drifted: got %#v want %q", name, property["type"], want)
	}
}

func schemaRequiredContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
