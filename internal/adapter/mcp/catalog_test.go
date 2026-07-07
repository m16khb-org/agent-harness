package mcp

import (
	"strings"
	"testing"
)

func TestAdapterOwnedToolsHaveWriteSemantics(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range AdapterOwnedTools() {
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete tool descriptor: %+v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Name == "worker_enqueue" && !strings.Contains(tool.Description, "never executes shell") {
			t.Fatalf("worker enqueue description must state shell safety: %s", tool.Description)
		}
	}
}

func TestToolMapsPreserveDescriptorShape(t *testing.T) {
	tools := ToolMaps([]Tool{
		{
			Name:        "example_tool",
			Description: "Example tool.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	})

	if len(tools) != 1 {
		t.Fatalf("expected one mapped tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool["name"] != "example_tool" {
		t.Fatalf("unexpected name: %#v", tool["name"])
	}
	if tool["description"] != "Example tool." {
		t.Fatalf("unexpected description: %#v", tool["description"])
	}
	if _, ok := tool["inputSchema"].(map[string]any); !ok {
		t.Fatalf("inputSchema should remain a map, got %T", tool["inputSchema"])
	}
}

func TestDispatchMapCoversAllCatalogTools(t *testing.T) {
	dm := DispatchMap()

	// Collect every tool from all catalog functions.
	allTools := collectAllCatalogTools()
	if len(allTools) == 0 {
		t.Fatal("expected non-empty tool catalog")
	}

	// Every tool in the catalog must appear in DispatchMap.
	for _, tool := range allTools {
		if _, ok := dm[tool.Name]; !ok {
			t.Errorf("tool %q missing from DispatchMap", tool.Name)
		}
	}

	// Every entry in DispatchMap must have a valid group.
	validGroups := map[DispatchGroup]bool{
		DispatchProject:         true,
		DispatchPolicyState:     true,
		DispatchIssueOps:        true,
		DispatchWorkPool:        true,
		DispatchAssistantWorker: true,
		DispatchSelfLoop:        true,
	}
	for name, group := range dm {
		if !validGroups[group] {
			t.Errorf("tool %q has unknown dispatch group %q", name, group)
		}
	}

	// No duplicate tool names in DispatchMap (map guarantees this, but verify).
	if len(dm) < len(allTools) {
		t.Errorf("DispatchMap has %d entries but catalog has %d tools; some tools may be missing or duplicates may exist", len(dm), len(allTools))
	}
}

func TestDispatchMapHasNoUnknownGroup(t *testing.T) {
	dm := DispatchMap()
	valid := map[DispatchGroup]bool{
		DispatchProject: true, DispatchPolicyState: true, DispatchIssueOps: true, DispatchWorkPool: true,
		DispatchAssistantWorker: true, DispatchSelfLoop: true,
	}
	for name, group := range dm {
		if !valid[group] {
			t.Errorf("tool %q has unknown group %q", name, group)
		}
	}
}

// collectAllCatalogTools gathers every tool declared across all catalog
// functions. It delegates to AllTools so the test stays bound to the
// catalogSections single source of truth and cannot drift from it.
func collectAllCatalogTools() []Tool {
	return AllTools()
}

func TestResourceMapsPreserveDescriptorShape(t *testing.T) {
	resources := ResourceMaps([]Resource{
		{
			URI:         "harness://example",
			Name:        "Example",
			Description: "Example resource.",
			MimeType:    "text/plain",
		},
	})

	if len(resources) != 1 {
		t.Fatalf("expected one mapped resource, got %d", len(resources))
	}
	resource := resources[0]
	if resource["uri"] != "harness://example" {
		t.Fatalf("unexpected uri: %#v", resource["uri"])
	}
	if resource["name"] != "Example" {
		t.Fatalf("unexpected name: %#v", resource["name"])
	}
	if resource["description"] != "Example resource." {
		t.Fatalf("unexpected description: %#v", resource["description"])
	}
	if resource["mimeType"] != "text/plain" {
		t.Fatalf("unexpected mimeType: %#v", resource["mimeType"])
	}
}
