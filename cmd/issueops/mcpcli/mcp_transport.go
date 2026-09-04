package mcpcli

import (
	"context"
	"io"
	"os"
)

func RunMCP() error {
	return RunMCPWithDependencies(MCPDependencies{})
}

func RunMCPWithDependencies(deps MCPDependencies) error {
	if os.Getenv("ISSUEOPS_MCP_DIRECT") == "1" {
		return ServeMCPStreamWithDependencies(os.Stdin, os.Stdout, os.Stderr, deps)
	}
	return RunMCPProxy()
}

// ServeMCPStream runs the MCP server over the official SDK transport for both
// split stdio streams and daemon-backed bidirectional connections.
func ServeMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return ServeMCPStreamWithDependencies(input, output, diagnostics, MCPDependencies{})
}

func ServeMCPStreamWithDependencies(input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return ServeMCPStreamContextWithDependencies(context.Background(), input, output, diagnostics, deps)
}

func ServeMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, MCPDependencies{})
}

func ServeMCPStreamContextWithDependencies(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return serveMCPStreamSDK(ctx, input, output, diagnostics, deps)
}
