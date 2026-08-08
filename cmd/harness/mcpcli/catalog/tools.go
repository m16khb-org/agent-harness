package catalog

import mcpadapter "agent-harness/internal/domain/mcp"

// Tools returns the advertised MCP tools/list payload. It derives entirely from
// the adapter catalog (mcpadapter.AdvertisedTools) so the advertised list, the
// DispatchMap routing, and the tool schemas all share one source of truth.
func Tools() []map[string]any {
	return mcpadapter.ToolMaps(mcpadapter.AdvertisedTools())
}
