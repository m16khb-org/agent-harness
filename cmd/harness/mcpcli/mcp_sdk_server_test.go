package mcpcli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDKToolHandlerDispatchesCatalogTool(t *testing.T) {
	handler := sdkToolHandler(resolveHandlerGroup("contract_schema"), "contract_schema")
	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("sdk tool handler returned error: %v", err)
	}
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("sdk tool handler result = %#v, want one text content", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("sdk tool content type = %T, want *mcp.TextContent", result.Content[0])
	}
	if !strings.Contains(text.Text, "agent_harness_cli_mcp_compatibility") {
		t.Fatalf("sdk tool response missing contract schema payload:\n%s", text.Text)
	}
}

func TestSDKToolHandlerRejectsInvalidRawArguments(t *testing.T) {
	handler := sdkToolHandler(resolveHandlerGroup("contract_schema"), "contract_schema")
	_, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{not json`)},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid arguments") {
		t.Fatalf("sdk tool invalid args error = %v, want invalid arguments", err)
	}
}

func TestSDKResourceHandlerReadsHarnessResource(t *testing.T) {
	result, err := sdkResourceHandler()(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "harness://commit-policy"},
	})
	if err != nil {
		t.Fatalf("sdk resource handler returned error: %v", err)
	}
	if result == nil || len(result.Contents) != 1 {
		t.Fatalf("sdk resource result = %#v, want one resource content", result)
	}
	content := result.Contents[0]
	if content.URI != "harness://commit-policy" || content.MIMEType != "text/markdown" {
		t.Fatalf("unexpected sdk resource content metadata: %#v", content)
	}
	if !strings.Contains(strings.ToLower(content.Text), "commit") {
		t.Fatalf("sdk resource content did not include commit policy text:\n%s", content.Text)
	}
}

func TestInitSDKServerIsIdempotent(t *testing.T) {
	oldServer := sdkServer
	defer func() { sdkServer = oldServer }()
	sdkServer = nil

	first := initSDKServer()
	second := initSDKServer()

	if first == nil || second == nil {
		t.Fatalf("initSDKServer returned nil: first=%v second=%v", first, second)
	}
	if first != second {
		t.Fatalf("initSDKServer should reuse the registered SDK server")
	}
}
