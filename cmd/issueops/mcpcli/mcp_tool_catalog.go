package mcpcli

import "issueops/cmd/issueops/mcpcli/catalog"

func MCPTools() []map[string]any {
	return catalog.Tools()
}
