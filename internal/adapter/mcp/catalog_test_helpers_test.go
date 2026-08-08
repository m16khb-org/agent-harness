package mcp

import "testing"

type toolDescriptorExpectation struct {
	name                string
	descriptionContains string
	required            []string
	properties          []string
}

func toolsByName(tools []Tool) map[string]Tool {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

func assertToolDescriptors(t *testing.T, label string, tools []Tool, expectations []toolDescriptorExpectation) map[string]Tool {
	t.Helper()
	if len(tools) != len(expectations) {
		t.Fatalf("expected %d %s tools, got %d", len(expectations), label, len(tools))
	}

	byName := toolsByName(tools)
	for _, expectation := range expectations {
		tool, ok := byName[expectation.name]
		if !ok {
			t.Fatalf("missing %s tool %q", label, expectation.name)
		}
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete %s tool descriptor: %+v", label, tool)
		}
		if expectation.descriptionContains != "" && !contains(tool.Description, expectation.descriptionContains) {
			t.Fatalf("%s description drifted: %s", expectation.name, tool.Description)
		}
		for _, name := range expectation.required {
			if !schemaRequires(tool.InputSchema, name) {
				t.Fatalf("%s schema must require %q: %#v", expectation.name, name, tool.InputSchema)
			}
		}
		for _, name := range expectation.properties {
			if !schemaHasProperty(tool.InputSchema, name) {
				t.Fatalf("%s schema missing %q: %#v", expectation.name, name, tool.InputSchema)
			}
		}
	}
	return byName
}

func schemaHasProperty(schema map[string]any, name string) bool {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = properties[name]
	return ok
}

func schemaRequires(schema map[string]any, name string) bool {
	required, ok := schema["required"].([]string)
	if !ok {
		return false
	}
	for _, item := range required {
		if item == name {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
