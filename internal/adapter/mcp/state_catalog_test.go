package mcp

import "testing"

func TestStateToolsExposeStableDescriptors(t *testing.T) {
	tools := StateTools()
	if len(tools) != 6 {
		t.Fatalf("expected six state tools, got %d", len(tools))
	}

	byName := toolsByName(tools)
	for _, tool := range tools {
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete state tool descriptor: %+v", tool)
		}
	}

	write, ok := byName["state_write"]
	if !ok {
		t.Fatal("missing state_write descriptor")
	}
	if !schemaRequires(write.InputSchema, "key") || !schemaRequires(write.InputSchema, "content") {
		t.Fatalf("state_write schema required fields drifted: %#v", write.InputSchema)
	}

	prune, ok := byName["state_prune"]
	if !ok {
		t.Fatal("missing state_prune descriptor")
	}
	if !schemaRequires(prune.InputSchema, "max_age") {
		t.Fatalf("state_prune schema must require max_age: %#v", prune.InputSchema)
	}
	if !schemaHasProperty(prune.InputSchema, "confirm") {
		t.Fatalf("state_prune schema missing confirm: %#v", prune.InputSchema)
	}

	doctor, ok := byName["state_doctor"]
	if !ok {
		t.Fatal("missing state_doctor descriptor")
	}
	if !contains(doctor.Description, "without modifying state") {
		t.Fatalf("state_doctor description drifted: %s", doctor.Description)
	}

	migrate, ok := byName["state_migrate"]
	if !ok {
		t.Fatal("missing state_migrate descriptor")
	}
	if !schemaHasProperty(migrate.InputSchema, "confirm") {
		t.Fatalf("state_migrate schema missing confirm: %#v", migrate.InputSchema)
	}
}
