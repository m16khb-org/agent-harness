package mcpsmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"issueops/cmd/issueops/commandstep"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func runSDKSmoke(root, binary string, env []string, timeout time.Duration) StepResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "mcp")
	cmd.Dir = root
	cmd.Env = commandstep.MergeEnvOverrides(os.Environ(), env)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "issueops-self-verify", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("connect SDK MCP session: %w", err))
	}
	// 세션 Close는 stderr 버퍼(cmd.Stderr)와의 경쟁을 없앤 뒤에만 읽는다:
	// subprocess가 아직 살아있는 동안 stderr.String()을 평가하면 데이터
	// 레이스다(-race 디텍터가 실제로 잡았다). 정상 경로는 명시적으로
	// Close하고 결과를 조립한다.
	defer session.Close()

	results := make([]any, 0, 11)
	results = append(results, session.InitializeResult())
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("list MCP tools: %w", err))
	}
	results = append(results, tools)
	for _, uri := range []string{
		"issueops://commit-policy",
		"issueops://state",
		"issueops://docs",
		"issueops://command-policy",
		"issueops://project-docs",
		"issueops://project-doc-upkeep",
	} {
		result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("read MCP resource %s: %w", uri, err))
		}
		results = append(results, result)
	}
	for _, call := range []mcp.CallToolParams{
		{Name: "project_docs_route", Arguments: map[string]any{"repo": ".", "task": "commit"}},
		{Name: "state_prune", Arguments: map[string]any{"max_age": "1h"}},
		{Name: "state_doctor", Arguments: map[string]any{}},
	} {
		result, err := session.CallTool(ctx, &call)
		if err != nil {
			return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("call MCP tool %s: %w", call.Name, err))
		}
		if result.IsError {
			return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("MCP tool %s returned isError", call.Name))
		}
		results = append(results, result)
	}

	var stdout strings.Builder
	for index, result := range results {
		body, err := json.Marshal(result)
		if err != nil {
			return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("marshal MCP result %d: %w", index+1, err))
		}
		stdout.Write(body)
		stdout.WriteByte('\n')
	}
	// 모든 상호작용이 끝났다: 세션을 닫아 subprocess를 종료한 뒤 stderr를
	// 스냅샷한다(defer Close보다 먼저 String()을 평가하면 레이스).
	closeErr := session.Close()
	stderrSnapshot := stderr.String()
	if closeErr != nil {
		return sdkSmokeFailure(started, binary, stderrSnapshot, fmt.Errorf("close SDK MCP session: %w", closeErr))
	}
	return StepResult{
		Label: "MCP smoke", Command: binary + " mcp", OK: true,
		DurationMS: time.Since(started).Milliseconds(), Stdout: stdout.String(), Stderr: stderrSnapshot,
		StdoutBytes: stdout.Len(), StderrBytes: len(stderrSnapshot),
	}
}

func sdkSmokeFailure(started time.Time, binary, stderr string, err error) StepResult {
	return StepResult{
		Label: "MCP smoke", Command: binary + " mcp", OK: false,
		DurationMS: time.Since(started).Milliseconds(), Stderr: stderr, StderrBytes: len(stderr), Error: err.Error(),
	}
}
