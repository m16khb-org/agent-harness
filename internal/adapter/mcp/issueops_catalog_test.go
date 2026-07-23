package mcp

import (
	"reflect"
	"testing"
)

func TestIssueOpsAdvertisesOnlyExecutionActionTool(t *testing.T) {
	tools := IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		t.Fatalf("IssueOps MCP tools = %#v, want only issueops_execution", tools)
	}

	properties, ok := tools[0].InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", tools[0].InputSchema["properties"])
	}
	action, ok := properties["action"].(map[string]any)
	if !ok {
		t.Fatalf("action schema = %#v", properties["action"])
	}
	want := []string{"prepare", "status", "claim", "release", "replace", "reconcile", "complete"}
	if got := action["enum"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("action enum = %#v, want %#v", got, want)
	}
	mode, ok := properties["mode"].(map[string]any)
	if !ok {
		t.Fatalf("mode schema = %#v", properties["mode"])
	}
	if got, want := mode["enum"], []string{"auto", "direct", "orca"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mode enum = %#v, want %#v", got, want)
	}
}

func TestIssueOpsDispatchContainsNoLegacyTools(t *testing.T) {
	dispatch := DispatchMap()
	if got := dispatch["issueops_execution"]; got != DispatchIssueOps {
		t.Fatalf("issueops_execution dispatch = %q", got)
	}
	for name := range dispatch {
		if name != "issueops_execution" && len(name) >= len("issueops_") && name[:len("issueops_")] == "issueops_" {
			t.Fatalf("legacy IssueOps MCP tool remains advertised: %s", name)
		}
	}
}
