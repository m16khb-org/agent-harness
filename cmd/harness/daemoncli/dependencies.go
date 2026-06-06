package daemoncli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var HarnessRoot = func() string {
	return "."
}

var ServeMCPStream = func(io.Reader, io.Writer, io.Writer) error {
	return fmt.Errorf("MCP stream handler is not configured")
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
