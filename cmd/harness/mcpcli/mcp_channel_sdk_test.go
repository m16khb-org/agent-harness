package mcpcli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP SDK transport 세션을 통해 channel 도구가 advertise되고 프론트/서버
// 수발신이 실제로 동작하는지 검증한다.
func TestServeMCPStreamAdvertisesAndRunsChannelTools(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	session := startMCPTransportTestSession(t, "stdio", MCPDependencies{})

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	advertised := map[string]bool{}
	for _, tool := range tools.Tools {
		advertised[tool.Name] = true
	}
	for _, name := range []string{"channel_send", "channel_recv"} {
		if !advertised[name] {
			t.Fatalf("tools/list missing %s", name)
		}
	}

	// 서버 세션 → 계약 메시지 발신.
	sendResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "channel_send",
		Arguments: map[string]any{
			"channel": "contract",
			"from":    "server",
			"body":    "GET /users -> 200 {id,name,email}",
		},
	})
	if err != nil || sendResult.IsError {
		t.Fatalf("channel_send over SDK failed: err=%v result=%+v", err, sendResult)
	}

	// 프론트 세션 → 대기 수신(이미 메시지가 있으므로 즉시 반환).
	recvResult, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "channel_recv",
		Arguments: map[string]any{
			"channel":         "contract",
			"wait":            true,
			"timeout_seconds": 5,
		},
	})
	if err != nil || recvResult.IsError {
		t.Fatalf("channel_recv over SDK failed: err=%v result=%+v", err, recvResult)
	}
	content := toolResultText(recvResult)
	if !strings.Contains(content, `"from": "server"`) || !strings.Contains(content, `{id,name,email}`) {
		t.Fatalf("channel_recv SDK payload unexpected:\n%s", content)
	}
	if strings.Contains(content, `"timed_out": true`) {
		t.Fatalf("existing message must not time out:\n%s", content)
	}
}
