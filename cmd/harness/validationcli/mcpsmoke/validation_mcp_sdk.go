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

	"agent-harness/cmd/harness/commandstep"
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

	client := mcp.NewClient(&mcp.Implementation{Name: "agent-harness-self-verify", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("connect SDK MCP session: %w", err))
	}
	defer session.Close()

	results := make([]any, 0, 11)
	results = append(results, session.InitializeResult())
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return sdkSmokeFailure(started, binary, stderr.String(), fmt.Errorf("list MCP tools: %w", err))
	}
	results = append(results, tools)
	for _, uri := range []string{
		"harness://commit-policy",
		"harness://state",
		"harness://docs",
		"harness://command-policy",
		"harness://project-docs",
		"harness://project-doc-upkeep",
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
	return StepResult{
		Label: "MCP smoke", Command: binary + " mcp", OK: true,
		DurationMS: time.Since(started).Milliseconds(), Stdout: stdout.String(), Stderr: stderr.String(),
		StdoutBytes: stdout.Len(), StderrBytes: stderr.Len(),
	}
}

func sdkSmokeFailure(started time.Time, binary, stderr string, err error) StepResult {
	return StepResult{
		Label: "MCP smoke", Command: binary + " mcp", OK: false,
		DurationMS: time.Since(started).Milliseconds(), Stderr: stderr, StderrBytes: len(stderr), Error: err.Error(),
	}
}
