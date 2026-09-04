package mcpcli

import (
	"fmt"

	"issueops/cmd/issueops/mcpcli/argmap"
	channelcontract "issueops/internal/contract/channel"
)

var channelMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"channel_send": handleMCPChannelSend,
	"channel_recv": handleMCPChannelRecv,
}

func handleChannelMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := channelMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}

func channelMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
	}
	return mcpToolPayload(payload)
}

func handleMCPChannelSend(args map[string]any) MCPToolOutcome {
	result, err := ChannelSend(channelcontract.SendRequest{
		Channel: argmap.String(args, "channel"),
		From:    argmap.String(args, "from"),
		Body:    argmap.String(args, "body"),
	})
	return channelMCPOutcome(result, err, "Channel send failed")
}

func handleMCPChannelRecv(args map[string]any) MCPToolOutcome {
	result, err := ChannelRecv(channelcontract.RecvRequest{
		Channel:        argmap.String(args, "channel"),
		SinceID:        argmap.String(args, "since_id"),
		Wait:           argmap.Bool(args, "wait"),
		TimeoutSeconds: argmap.Int(args, "timeout_seconds", 0),
		Limit:          argmap.Int(args, "limit", 0),
	})
	return channelMCPOutcome(result, err, "Channel recv failed")
}
