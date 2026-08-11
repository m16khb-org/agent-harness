package mcp

import "testing"

func TestSelfVerifyToolExposesSingleEvidencePass(t *testing.T) {
	var selfVerify Tool
	for _, tool := range AdvertisedTools() {
		if tool.Name == "self_verify" {
			selfVerify = tool
			break
		}
	}
	properties, ok := selfVerify.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("self_verify properties missing: %+v", selfVerify)
	}
	if _, exists := properties["full"]; exists {
		t.Fatal("self_verify still exposes repeated full mode")
	}
	if _, exists := properties["iterations"]; exists {
		t.Fatal("self_verify still exposes repeated iterations")
	}
}
