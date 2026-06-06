package mcp

import "testing"

func TestLocalAssistantToolsExposeStableDescriptors(t *testing.T) {
	tools := LocalAssistantTools()
	if len(tools) != 2 {
		t.Fatalf("expected two local assistant tools, got %d", len(tools))
	}

	byName := toolsByName(tools)
	for _, tool := range tools {
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete local assistant tool descriptor: %+v", tool)
		}
	}

	commitSuggest, ok := byName["commit_suggest"]
	if !ok {
		t.Fatal("missing commit_suggest descriptor")
	}
	if !contains(commitSuggest.Description, "Conventional + Lore Hybrid") {
		t.Fatalf("commit_suggest description drifted: %s", commitSuggest.Description)
	}
	if !schemaHasProperty(commitSuggest.InputSchema, "agy_model") {
		t.Fatalf("commit_suggest schema missing agy_model: %#v", commitSuggest.InputSchema)
	}

	lintDiagnose, ok := byName["lint_diagnose"]
	if !ok {
		t.Fatal("missing lint_diagnose descriptor")
	}
	if !schemaRequires(lintDiagnose.InputSchema, "command_argv") {
		t.Fatalf("lint_diagnose schema must require command_argv: %#v", lintDiagnose.InputSchema)
	}
	if !schemaHasProperty(lintDiagnose.InputSchema, "command_argv") {
		t.Fatalf("lint_diagnose schema missing command_argv property: %#v", lintDiagnose.InputSchema)
	}
}
