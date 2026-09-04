package mcp

import "testing"

func TestResourcesExposeStableDescriptors(t *testing.T) {
	resources := Resources()
	if len(resources) == 0 {
		t.Fatal("expected MCP resources")
	}

	byURI := map[string]Resource{}
	for _, resource := range resources {
		if resource.URI == "" {
			t.Fatalf("resource missing uri: %+v", resource)
		}
		if resource.Name == "" {
			t.Fatalf("resource %q missing name", resource.URI)
		}
		if resource.Description == "" {
			t.Fatalf("resource %q missing description", resource.URI)
		}
		if resource.MimeType == "" {
			t.Fatalf("resource %q missing mime type", resource.URI)
		}
		if _, exists := byURI[resource.URI]; exists {
			t.Fatalf("duplicate resource uri %q", resource.URI)
		}
		byURI[resource.URI] = resource
	}

	for _, uri := range []string{
		"issueops://commit-policy",
		"issueops://skill/atomic-commit-push",
		"issueops://agents",
		"issueops://docs",
		"issueops://project-docs",
		"issueops://project-doc-upkeep",
		"issueops://api-doc-guidance",
		"issueops://command-policy",
		"issueops://state",
	} {
		if _, exists := byURI[uri]; !exists {
			t.Fatalf("missing resource uri %q", uri)
		}
	}
}
