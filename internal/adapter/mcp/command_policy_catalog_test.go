package mcp

import "testing"

func TestCommandPolicyInputSchemaPreservesPolicyFields(t *testing.T) {
	schema := CommandPolicyInputSchema()
	for _, name := range []string{"workspace_root", "cwd", "argv"} {
		if !schemaRequires(schema, name) {
			t.Fatalf("command policy schema must require %q: %#v", name, schema["required"])
		}
	}

	for _, name := range []string{
		"timeout",
		"env_allowlist",
		"network_allowed",
		"write_allowed",
		"shell_allowed",
		"shell_reason",
		"audit_log_id",
	} {
		if !schemaHasProperty(schema, name) {
			t.Fatalf("command policy schema missing %q: %#v", name, schema["properties"])
		}
	}
}

func TestCommandPolicyToolsExposeStableDescriptors(t *testing.T) {
	assertToolDescriptors(t, "command policy", CommandPolicyTools(), []toolDescriptorExpectation{
		{
			name:                "command_policy_check",
			descriptionContains: "without executing it",
			required:            []string{"workspace_root"},
		},
		{
			name:                "command_fake_run",
			descriptionContains: "never executes the command",
			properties:          []string{"shell_reason"},
		},
	})
}

func TestCommandPolicyAuditToolsExposeStableDescriptor(t *testing.T) {
	tools := CommandPolicyAuditTools()
	if len(tools) != 1 {
		t.Fatalf("expected one command policy audit tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "command_policy_audit" {
		t.Fatalf("unexpected audit tool name: %q", tool.Name)
	}
	if !contains(tool.Description, "redacted JSONL audit record") {
		t.Fatalf("audit description drifted: %s", tool.Description)
	}
	if !schemaRequires(tool.InputSchema, "workspace_root") {
		t.Fatalf("audit schema must reuse command policy input schema: %#v", tool.InputSchema)
	}
	if !schemaHasProperty(tool.InputSchema, "audit_log_id") {
		t.Fatalf("audit schema missing audit_log_id: %#v", tool.InputSchema)
	}
}
