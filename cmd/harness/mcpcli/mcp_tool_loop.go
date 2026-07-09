package mcpcli

import (
	"fmt"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/core"
)

var loopMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"loop_start":          handleMCPLoopStart,
	"loop_record_attempt": handleMCPLoopRecordAttempt,
	"loop_status":         handleMCPLoopStatus,
	"loop_stop":           handleMCPLoopStop,
}

func handleLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := loopMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}

func loopMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
	}
	return mcpToolPayload(payload)
}

func handleMCPLoopStart(args map[string]any) MCPToolOutcome {
	result, err := core.StartLoopRun(core.LoopRunStartRequest{
		Repo:        argmap.String(args, "repo"),
		Name:        argmap.String(args, "name"),
		Goal:        argmap.String(args, "goal"),
		VerifyArgv:  argmap.StringSlice(args, "verify_argv"),
		MaxAttempts: argmap.Int(args, "max_attempts", 0),
	})
	return loopMCPOutcome(result, err, "Loop start failed")
}

func handleMCPLoopRecordAttempt(args map[string]any) MCPToolOutcome {
	result, err := core.RecordLoopAttempt(argmap.String(args, "id"), core.LoopRunRecordAttemptRequest{
		Verdict:  argmap.String(args, "verdict"),
		Evidence: argmap.StringSlice(args, "evidence"),
	})
	return loopMCPOutcome(result, err, "Loop record-attempt failed")
}

func handleMCPLoopStatus(args map[string]any) MCPToolOutcome {
	result, err := core.LoopRunStatus(argmap.String(args, "id"))
	return loopMCPOutcome(result, err, "Loop status failed")
}

func handleMCPLoopStop(args map[string]any) MCPToolOutcome {
	result, err := core.StopLoopRun(argmap.String(args, "id"), argmap.Bool(args, "success"), argmap.String(args, "reason"))
	return loopMCPOutcome(result, err, "Loop stop failed")
}
