package mcpcli

import "agent-harness/cmd/harness/mcpcli/catalog"

func MCPTools() []map[string]any {
	return catalog.Tools()
}
