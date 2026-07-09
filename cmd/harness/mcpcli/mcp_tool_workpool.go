package mcpcli

import (
	"fmt"

	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/core"
)

var workpoolMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"workpool_create":    handleMCPWorkpoolCreate,
	"workpool_add_task":  handleMCPWorkpoolAddTask,
	"workpool_claim":     handleMCPWorkpoolClaim,
	"workpool_heartbeat": handleMCPWorkpoolHeartbeat,
	"workpool_submit":    handleMCPWorkpoolSubmit,
	"workpool_accept":    handleMCPWorkpoolAccept,
	"workpool_reject":    handleMCPWorkpoolReject,
	"workpool_status":    handleMCPWorkpoolStatus,
	"workpool_reap":      handleMCPWorkpoolReap,
	"workpool_close":     handleMCPWorkpoolClose,
}

func handleWorkpoolMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := workpoolMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}

func workpoolMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
	}
	return mcpToolPayload(payload)
}

func handleMCPWorkpoolCreate(args map[string]any) MCPToolOutcome {
	result, err := core.CreateWorkPool(core.WorkPoolCreateRequest{
		Repo:          argmap.String(args, "repo"),
		Name:          argmap.String(args, "name"),
		ParentCycleID: argmap.String(args, "parent_cycle"),
		PilotRequired: argmap.Bool(args, "pilot_required"),
		Size:          argmap.Int(args, "size", 0),
		LeaseTTL:      argmap.String(args, "lease_ttl"),
		MaxAttempts:   argmap.Int(args, "max_attempts", 0),
	})
	return workpoolMCPOutcome(result, err, "Workpool create failed")
}

func handleMCPWorkpoolAddTask(args map[string]any) MCPToolOutcome {
	result, err := core.AddWorkPoolTask(argmap.String(args, "pool"), core.WorkPoolAddTaskRequest{
		Title:              argmap.String(args, "title"),
		Instructions:       argmap.String(args, "instructions"),
		Scope:              argmap.StringSlice(args, "scope"),
		AcceptanceCriteria: argmap.StringSlice(args, "acceptance"),
		Pilot:              argmap.Bool(args, "pilot"),
	})
	return workpoolMCPOutcome(result, err, "Workpool add-task failed")
}

func handleMCPWorkpoolClaim(args map[string]any) MCPToolOutcome {
	result, err := core.ClaimWorkPool(argmap.String(args, "pool"), argmap.String(args, "worker"))
	return workpoolMCPOutcome(result, err, "Workpool claim failed")
}

func handleMCPWorkpoolHeartbeat(args map[string]any) MCPToolOutcome {
	result, err := core.HeartbeatWorkPool(argmap.String(args, "pool"), argmap.String(args, "task"), argmap.String(args, "worker"))
	return workpoolMCPOutcome(result, err, "Workpool heartbeat failed")
}

func handleMCPWorkpoolSubmit(args map[string]any) MCPToolOutcome {
	result, err := core.SubmitWorkPool(
		argmap.String(args, "pool"),
		argmap.String(args, "task"),
		argmap.String(args, "worker"),
		argmap.StringSlice(args, "evidence"),
		argmap.String(args, "branch"),
		argmap.String(args, "worktree"),
	)
	return workpoolMCPOutcome(result, err, "Workpool submit failed")
}

func handleMCPWorkpoolAccept(args map[string]any) MCPToolOutcome {
	result, err := core.AcceptWorkPool(argmap.String(args, "pool"), argmap.String(args, "task"), argmap.StringSlice(args, "evidence"))
	return workpoolMCPOutcome(result, err, "Workpool accept failed")
}

func handleMCPWorkpoolReject(args map[string]any) MCPToolOutcome {
	result, err := core.RejectWorkPool(argmap.String(args, "pool"), argmap.String(args, "task"), argmap.String(args, "reason"), argmap.Bool(args, "requeue"))
	return workpoolMCPOutcome(result, err, "Workpool reject failed")
}

func handleMCPWorkpoolStatus(args map[string]any) MCPToolOutcome {
	result, err := core.StatusWorkPool(argmap.String(args, "pool"))
	return workpoolMCPOutcome(result, err, "Workpool status failed")
}

func handleMCPWorkpoolReap(args map[string]any) MCPToolOutcome {
	result, err := core.ReapWorkPool(argmap.String(args, "pool"))
	return workpoolMCPOutcome(result, err, "Workpool reap failed")
}

func handleMCPWorkpoolClose(args map[string]any) MCPToolOutcome {
	result, err := core.CloseWorkPool(argmap.String(args, "pool"), argmap.Bool(args, "force"), argmap.String(args, "reason"))
	return workpoolMCPOutcome(result, err, "Workpool close failed")
}
