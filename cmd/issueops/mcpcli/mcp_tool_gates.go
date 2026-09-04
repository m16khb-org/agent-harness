package mcpcli

import (
	"fmt"

	"issueops/cmd/issueops/mcpcli/argmap"
	gatescontract "issueops/internal/contract/gates"
)

var gatesMCPHandlers = map[string]func(map[string]any) MCPToolOutcome{
	"gates_check":   handleMCPGatesCheck,
	"gates_status":  handleMCPGatesStatus,
	"gates_report":  handleMCPGatesReport,
	"gates_abandon": handleMCPGatesAbandon,
	"gates_init":    handleMCPGatesInit,
}

func handleGatesMCPToolCall(call MCPToolCall) MCPToolOutcome {
	handler, ok := gatesMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(call.Arguments)
}

func gatesMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
	}
	return mcpToolPayload(payload)
}

func gatesBaseCheckRequest(args map[string]any) gatescontract.CheckRequest {
	// CLI의 --write 기본값(true)과 parity를 맞춘다. 게이트 CHECK는 검증
	// 명령(빌드/테스트/코드젠)이 많아 workspace-write 기본 권한이 실제 사용
	// 패턴과 일치한다. 좁히려면 write_allowed=false를 명시적으로 보낸다.
	writeAllowed := true
	if _, present := args["write_allowed"]; present {
		writeAllowed = argmap.Bool(args, "write_allowed")
	}
	return gatescontract.CheckRequest{
		WorkspaceRoot:  argmap.String(args, "workspace_root"),
		CWD:            argmap.String(args, "cwd"),
		Files:          argmap.StringSlice(args, "files"),
		EnvAllowlist:   argmap.StringSlice(args, "env_allowlist"),
		WriteAllowed:   writeAllowed,
		NetworkAllowed: argmap.Bool(args, "network_allowed"),
	}
}

func handleMCPGatesCheck(args map[string]any) MCPToolOutcome {
	req := gatesBaseCheckRequest(args)
	req.TimeoutSeconds = argmap.Int(args, "timeout_seconds", 0)
	result, err := GatesCheck(req)
	return gatesMCPOutcome(result, err, "Gates check failed")
}

func handleMCPGatesStatus(args map[string]any) MCPToolOutcome {
	req := gatesBaseCheckRequest(args)
	req.StatusOnly = true
	result, err := GatesCheck(req)
	return gatesMCPOutcome(result, err, "Gates status failed")
}

func handleMCPGatesReport(args map[string]any) MCPToolOutcome {
	req := gatesBaseCheckRequest(args)
	req.StatusOnly = true
	result, err := GatesCheck(req)
	return gatesMCPOutcome(result, err, "Gates report failed")
}

func handleMCPGatesAbandon(args map[string]any) MCPToolOutcome {
	result, err := GatesAbandon(gatescontract.AbandonRequest{
		File:   argmap.String(args, "file"),
		GateID: argmap.String(args, "gate_id"),
		Reason: argmap.String(args, "reason"),
	})
	return gatesMCPOutcome(result, err, "Gates abandon failed")
}

func handleMCPGatesInit(args map[string]any) MCPToolOutcome {
	result, err := GatesInit(gatescontract.InitRequest{
		File:  argmap.String(args, "file"),
		Scope: argmap.String(args, "scope"),
		Gates: argmap.StringSlice(args, "gates"),
	})
	return gatesMCPOutcome(result, err, "Gates init failed")
}
