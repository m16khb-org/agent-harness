package daemoncli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var IssueOpsRoot = func() string {
	return "."
}

var ServeMCPStream = func(io.Reader, io.Writer, io.Writer) error {
	return fmt.Errorf("MCP stream handler is not configured")
}

var ServeMCPStreamContext = func(context.Context, io.Reader, io.Writer, io.Writer) error {
	return fmt.Errorf("context-aware MCP stream handler is not configured")
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
