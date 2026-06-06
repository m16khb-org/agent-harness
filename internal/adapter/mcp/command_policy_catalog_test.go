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
	tools := CommandPolicyTools()
	if len(tools) != 2 {
		t.Fatalf("expected two command policy tools, got %d", len(tools))
	}

	byName := toolsByName(tools)
	for _, tool := range tools {
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete command policy tool descriptor: %+v", tool)
		}
	}

	check, ok := byName["command_policy_check"]
	if !ok {
		t.Fatal("missing command_policy_check descriptor")
	}
	if !contains(check.Description, "without executing it") {
		t.Fatalf("command_policy_check description drifted: %s", check.Description)
	}
	if !schemaRequires(check.InputSchema, "workspace_root") {
		t.Fatalf("command_policy_check schema must require workspace_root: %#v", check.InputSchema)
	}

	fakeRun, ok := byName["command_fake_run"]
	if !ok {
		t.Fatal("missing command_fake_run descriptor")
	}
	if !contains(fakeRun.Description, "never executes the command") {
		t.Fatalf("command_fake_run description drifted: %s", fakeRun.Description)
	}
	if !schemaHasProperty(fakeRun.InputSchema, "shell_reason") {
		t.Fatalf("command_fake_run schema missing shell_reason: %#v", fakeRun.InputSchema)
	}
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
