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
	want := []string{"prepare", "status", "claim", "release", "replace", "resume", "reconcile", "complete"}
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
	host, ok := properties["host"].(map[string]any)
	if !ok {
		t.Fatalf("host schema = %#v", properties["host"])
	}
	if got, want := host["enum"], []string{"codex", "claude", "omo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("host enum = %#v, want %#v", got, want)
	}
	ownerHost, ok := properties["owner_host"].(map[string]any)
	if !ok {
		t.Fatalf("owner_host schema = %#v", properties["owner_host"])
	}
	if got, want := ownerHost["enum"], []string{"codex", "claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owner_host enum = %#v, want %#v", got, want)
	}
	claimCurrentToken, ok := properties["claim_current_token"].(map[string]any)
	if !ok || claimCurrentToken["type"] != "boolean" {
		t.Fatalf("claim_current_token schema = %#v, want boolean", properties["claim_current_token"])
	}
}

func TestIssueOpsExecutionSnapshotSchemaIsClosedAndPortable(t *testing.T) {
	tools := IssueOpsBasicTools()
	properties := tools[0].InputSchema["properties"].(map[string]any)
	snapshot, ok := properties["issue_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("issue_snapshot schema = %#v", properties["issue_snapshot"])
	}
	if got := snapshot["required"]; !reflect.DeepEqual(got, []string{"provider", "source", "web_url", "body", "state"}) {
		t.Fatalf("issue_snapshot required = %#v", got)
	}
	if got := snapshot["additionalProperties"]; got != false {
		t.Fatalf("issue_snapshot additionalProperties = %#v", got)
	}
	snapshotProperties, ok := snapshot["properties"].(map[string]any)
	if !ok {
		t.Fatalf("issue_snapshot properties = %#v", snapshot["properties"])
	}
	for _, field := range []string{"provider", "source", "web_url", "body", "state"} {
		if _, ok := snapshotProperties[field]; !ok {
			t.Fatalf("issue_snapshot is missing %q: %#v", field, snapshotProperties)
		}
	}
	for _, forbidden := range []string{"server_namespace", "wrapper", "profile", "token", "config"} {
		if _, ok := snapshotProperties[forbidden]; ok {
			t.Fatalf("issue_snapshot exposes host-private field %q", forbidden)
		}
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
